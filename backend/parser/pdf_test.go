package parser

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"vectile/backend/appdata"
)

// minimalPDF is a one-page PDF with no text layer: what a scan looks like. It
// carries a correct xref table, so PDFium has nothing to repair and the test
// measures our parsing rather than its error recovery.
func minimalPDF() []byte {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] >>",
	}

	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, body := range objects {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n", len(objects)+1)
	b.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(objects)+1, xref)
	return b.Bytes()
}

func writeScanPDF(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scan.pdf")
	if err := os.WriteFile(path, minimalPDF(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A page with no text layer and no OCR must be reported as unreadable: that
// count is what makes the app offer to install OCR.
func TestParsePDFCountsPagesWithoutText(t *testing.T) {
	path := writeScanPDF(t)

	pages, stats := ParsePDF(t.Context(), path, nil)
	if len(pages) != 0 {
		t.Fatalf("expected no text, got %d pages", len(pages))
	}
	if stats.Pages != 1 {
		t.Fatalf("Pages = %d, want 1", stats.Pages)
	}
	if stats.NoTextPages != 1 {
		t.Fatalf("NoTextPages = %d, want 1", stats.NoTextPages)
	}
	if stats.TextPages != 0 || stats.OCRPages != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

// Enabling OCR without a bundled binary must change nothing: the gate is
// ocr.Installed(), not the config flag alone.
func TestParsePDFWithoutBundleSkipsOCR(t *testing.T) {
	path := writeScanPDF(t)
	appdata.Dir = t.TempDir() // empty plugin dir: no bundle to find
	pages, stats := ParsePDF(t.Context(), path, &PDFOptions{OCR: true, Languages: []string{"eng"}})
	if len(pages) != 0 {
		t.Fatalf("expected no text, got %d pages", len(pages))
	}
	if stats.NoTextPages != 1 {
		t.Fatalf("NoTextPages = %d, want 1", stats.NoTextPages)
	}
}

func TestParsePDFMissingFileIsEmpty(t *testing.T) {
	pages, stats := ParsePDF(t.Context(), filepath.Join(t.TempDir(), "nope.pdf"), nil)
	if len(pages) != 0 {
		t.Fatalf("expected no pages, got %d", len(pages))
	}
	if stats.Pages != 0 {
		t.Fatalf("Pages = %d, want 0", stats.Pages)
	}
}
