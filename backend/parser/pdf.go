package parser

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"

	"vectile/backend/ocr"
)

// pdfPool is a lazily-initialized singleton so the WASM runtime is reused
// across all ParsePDF calls rather than re-created per file.
var (
	pdfPool     pdfium.Pool
	pdfPoolOnce sync.Once
	pdfPoolErr  error
)

// initPDFPool lazily builds the shared PDFium WASM pool.
//
// Stdout and Stderr are supplied explicitly because go-pdfium defaults both to
// os.Stdout / os.Stderr, and it hands them to wazero as the WASI stdio fds.
// The released Windows build is linked with -H windowsgui, which has no
// console: the std handles are not merely empty, they are invalid, so wazero
// fails to resolve "/dev/stdout" and every page fails to render with
//
//	could not instantiate webassembly module: GetFileType /dev/stdout:
//	The handle is invalid.
//
// The pool itself initialises either way, so the failure only appeared at PDF
// parse time and read as "0 indexed". Binding both streams to the app log
// keeps pdfium's own diagnostics (issue text, load errors) instead of
// discarding them, and gives wazero a writer that cannot fail a handle lookup.
func initPDFPool() {
	pdfPool, pdfPoolErr = webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
		Stdout:   logWriter{},
		Stderr:   logWriter{},
	})
}

// logWriter routes a child process's output into the app log at debug level,
// so pdfium diagnostics are visible with VECTILE_DEBUG=1 rather than lost.
//
// The Write method must never block or error: it now backs the WASI stdio fds
// for a module that renders every page, so a failure here would fail parsing.
// It returns len(p) unconditionally for that reason.
type logWriter struct{}

func (logWriter) Write(p []byte) (int, error) {
	// Checked before formatting: pdfium is chatty and this must stay cheap
	// when debug logging is off, which is the default.
	if len(p) > 0 && slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		if msg := strings.TrimSpace(string(p)); msg != "" {
			slog.Debug("pdfium", "msg", msg)
		}
	}
	return len(p), nil
}

// ClosePDFPool shuts down the PDFium WASM pool. Call on application exit.
func ClosePDFPool() {
	if pdfPool != nil {
		if err := pdfPool.Close(); err != nil {
			slog.Error("failed to close PDF pool", "err", err)
		}
	}
}

// defaultMinWords is the word count below which a page counts as having no
// usable text layer. Born-digital pages run to hundreds of words; a page of a
// scan yields a stray header or nothing at all.
const defaultMinWords = 10

// ocrDPI is the render resolution for the OCR fallback. Tesseract wants the
// glyphs at roughly 300 DPI; below that accuracy falls off quickly.
const ocrDPI = 300

// PageText represents a single page of extracted PDF text.
type PageText struct {
	PageNumber int // 1-based
	Text       string
	OCR        bool // true when the text came from OCR rather than a text layer
}

// PDFOptions configures the OCR fallback for pages with no text layer. A nil
// value, or OCR false, keeps the original behaviour: pages without text are
// skipped and no external process runs.
type PDFOptions struct {
	OCR       bool
	Languages []string
	MinWords  int // 0 uses defaultMinWords
}

// PDFStats summarises what one PDF yielded. NoTextPages counts pages that ended
// up with neither text nor readable OCR text, which is how the app knows a
// library contains scans and can suggest installing OCR.
type PDFStats struct {
	Pages       int
	TextPages   int
	OCRPages    int
	NoTextPages int
}

// ParsePDF extracts text from a PDF file page by page using PDFium (WASM).
//
// Pages with no usable text layer are rendered and put through the bundled
// Tesseract when opts enables it and the bundle is installed; a page OCR cannot
// read falls back to whatever little text it had. ctx cancels a long scan
// between pages and inside a single OCR run.
func ParsePDF(ctx context.Context, path string, opts *PDFOptions) ([]PageText, PDFStats) {
	if ctx == nil {
		// The OCR runner derives a timeout from this, which panics on nil.
		ctx = context.Background()
	}

	pdfPoolOnce.Do(initPDFPool)
	if pdfPoolErr != nil {
		slog.Error("failed to init PDF pool", "err", pdfPoolErr)
		return nil, PDFStats{}
	}

	instance, err := pdfPool.GetInstance(30 * time.Second)
	if err != nil {
		slog.Error("failed to get PDF pool instance", "err", err)
		return nil, PDFStats{}
	}
	defer instance.Close()

	pdfBytes, err := os.ReadFile(path)
	if err != nil {
		slog.Error("failed to read PDF file", "path", path, "err", err)
		return nil, PDFStats{}
	}

	doc, err := instance.OpenDocument(&requests.OpenDocument{File: &pdfBytes})
	if err != nil {
		slog.Error("failed to open PDF document", "path", path, "err", err)
		return nil, PDFStats{}
	}
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})

	pageCountResp, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		slog.Error("failed to get PDF page count", "path", path, "err", err)
		return nil, PDFStats{}
	}

	minWords := defaultMinWords
	// Installed() is checked per file, not cached, so a bundle installed while
	// the app is running is used by the next document.
	ocrEnabled := opts != nil && opts.OCR && ocr.Installed()
	if opts != nil && opts.MinWords > 0 {
		minWords = opts.MinWords
	}

	numPages := pageCountResp.PageCount
	var pages []PageText
	var stats PDFStats
	for i := 0; i < numPages; i++ {
		if ctx.Err() != nil {
			break
		}
		stats.Pages++

		resp, err := instance.GetPageText(&requests.GetPageText{
			Page: requests.Page{
				ByIndex: &requests.PageByIndex{Document: doc.Document, Index: i},
			},
		})
		if err != nil {
			slog.Warn("failed to extract text from page", "page", i+1, "path", path, "err", err)
			stats.NoTextPages++
			continue
		}

		text := strings.TrimSpace(resp.Text)
		if len(strings.Fields(text)) >= minWords {
			stats.TextPages++
			pages = append(pages, PageText{PageNumber: i + 1, Text: text})
			continue
		}

		// Sparse or missing text layer: this page is a picture of a page.
		if ocrEnabled {
			if ocred := renderAndOCR(ctx, instance, doc.Document, i, opts.Languages); ocred != "" {
				slog.Debug("OCR produced text for page",
					"page", i+1, "path", path, "words", len(strings.Fields(ocred)))
				stats.OCRPages++
				pages = append(pages, PageText{PageNumber: i + 1, Text: ocred, OCR: true})
				continue
			}
		}

		// Keep whatever little text the page did have, so a sparse but real text
		// layer is never thrown away.
		if text != "" {
			stats.TextPages++
			pages = append(pages, PageText{PageNumber: i + 1, Text: text})
			continue
		}
		stats.NoTextPages++
	}

	if len(pages) == 0 {
		// Log enough to tell the three cases apart, because the indexer only
		// sees "no content" and the shipped GUI build has no console: without
		// this line a total PDF failure and an empty scan look identical.
		switch {
		case pdfPoolErr != nil:
			slog.Error("no extractable text: PDF engine failed to start",
				"path", path, "err", pdfPoolErr)
		case opts != nil && opts.OCR && !ocr.Installed():
			slog.Warn("no extractable text in PDF and the OCR plugin is not installed",
				"path", path, "pages", numPages)
		case ocrEnabled:
			slog.Warn("no extractable text in PDF (OCR produced no text either)",
				"path", path, "pages", numPages)
		default:
			slog.Warn("no extractable text found in PDF (OCR disabled)",
				"path", path, "pages", numPages)
		}
	}

	return pages, stats
}

// renderAndOCR rasterizes one page and runs the bundled Tesseract on it.
func renderAndOCR(ctx context.Context, instance pdfium.Pdfium, document references.FPDF_DOCUMENT, pageIndex int, languages []string) string {
	resp, err := instance.RenderPageInDPI(&requests.RenderPageInDPI{
		Page: requests.Page{
			ByIndex: &requests.PageByIndex{Document: document, Index: pageIndex},
		},
		DPI: ocrDPI,
	})
	if err != nil {
		slog.Warn("failed to render PDF page for OCR", "page", pageIndex+1, "err", err)
		return ""
	}
	// Frees the rendered bitmap. Without this every page leaks into the WASM
	// heap for as long as the pool lives, which is the whole app run.
	defer resp.Cleanup()

	text, err := ocr.Run(ctx, resp.Result.Image, languages)
	if err != nil {
		// A cancelled run, or a bundle that is not installed, is expected.
		slog.Warn("OCR failed for page", "page", pageIndex+1, "err", err)
		return ""
	}
	return text
}
