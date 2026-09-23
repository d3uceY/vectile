package indexer

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	"vectile/backend/chunker"
	"vectile/backend/config"
	"vectile/backend/parser"
)

var preferredFormats = []string{"EPUB", "PDF"}

// IndexCalibre indexes ebooks from Calibre libraries into the "calibre"
// collection.
func IndexCalibre(ctx context.Context, conn *sql.DB, cfg *config.Config, force bool, progress ProgressCallback, embedder Embedder) *IndexResult {
	collectionID, err := getOrCreate(conn, "calibre", "system")
	if err != nil {
		return failedResult(err)
	}

	type bookEntry struct {
		libraryPath string
		book        *parser.CalibreBook
	}

	var allBooks []bookEntry
	for _, lib := range cfg.CalibreLibraries {
		lib = expandPath(lib)
		info, err := os.Stat(lib)
		if err != nil || !info.IsDir() {
			slog.Warn("calibre library path does not exist", "path", lib)
			continue
		}
		slog.Info("indexing Calibre library", "path", lib)
		books, err := parser.ParseCalibreLibrary(lib)
		if err != nil {
			slog.Error("failed to parse Calibre library", "path", lib, "err", err)
			continue
		}
		for _, b := range books {
			allBooks = append(allBooks, bookEntry{libraryPath: lib, book: b})
		}
	}

	result := &IndexResult{TotalFound: len(allBooks)}
	cleared := clearForRebuild(conn, collectionID, force)

	indexItemsBatched(ctx, conn, cfg, collectionID, "calibre", len(allBooks),
		func(i int) *indexItem {
			return bookToItem(ctx, conn, cfg, collectionID, allBooks[i].libraryPath, allBooks[i].book, force, result)
		},
		embedder, result, progress, cleared)
	return result
}

func buildBookMetadata(book *parser.CalibreBook, libraryPath, format string) map[string]any {
	meta := map[string]any{}
	if len(book.Authors) > 0 {
		meta["authors"] = book.Authors
	}
	if len(book.Tags) > 0 {
		meta["tags"] = book.Tags
	}
	if book.Series != "" {
		meta["series"] = book.Series
	}
	if book.SeriesIndex != 0 {
		meta["series_index"] = book.SeriesIndex
	}
	if book.Publisher != "" {
		meta["publisher"] = book.Publisher
	}
	if book.Pubdate != "" {
		meta["pubdate"] = book.Pubdate
	}
	if book.Rating != 0 {
		meta["rating"] = book.Rating
	}
	if len(book.Languages) > 0 {
		meta["languages"] = book.Languages
	}
	if len(book.Identifiers) > 0 {
		meta["identifiers"] = book.Identifiers
	}
	meta["calibre_id"] = book.BookID
	if format != "" {
		meta["format"] = format
	}
	meta["library"] = libraryPath
	return meta
}

// bookToItem prepares one book for indexing, or returns nil if it should be
// skipped — unchanged, or no extractable content.
func bookToItem(ctx context.Context, conn *sql.DB, cfg *config.Config, collectionID int64, libraryPath string, book *parser.CalibreBook, force bool, res *IndexResult) *indexItem {
	filePath, format := parser.GetBookFilePath(libraryPath, book, preferredFormats)

	var sourcePath, contentHash, sourceType string
	if filePath != "" {
		sourcePath = filePath
		h, err := fileHash(filePath)
		if err != nil {
			slog.Warn("cannot hash book file, skipping", "title", book.Title, "err", err)
			return nil
		}
		contentHash = h
		sourceType = format
	} else {
		if book.Description == "" {
			slog.Warn("book has no EPUB/PDF and no description, skipping", "title", book.Title)
			return nil
		}
		sourcePath = fmt.Sprintf("calibre://%s/%s", libraryPath, book.RelativePath)
		h := sha256.Sum256([]byte(book.Description))
		contentHash = fmt.Sprintf("%x", h)
		sourceType = "calibre-description"
		format = ""
	}

	if !force && isSourceUnchanged(conn, collectionID, sourcePath, contentHash) {
		return nil
	}

	bookMeta := buildBookMetadata(book, libraryPath, format)
	chunks := extractAndChunkBook(ctx, book, filePath, format, cfg, bookMeta, res)
	if len(chunks) == 0 {
		slog.Warn("no content extracted from book, skipping", "title", book.Title)
		return nil
	}

	return &indexItem{
		SourcePath: sourcePath,
		SourceType: sourceType,
		Title:      book.Title,
		Chunks:     chunks,
		FileHash:   contentHash,
		Mtime:      book.LastModified,
	}
}

func extractAndChunkBook(ctx context.Context, book *parser.CalibreBook, filePath, format string, cfg *config.Config, bookMeta map[string]any, res *IndexResult) []chunker.Chunk {
	chunkSize := cfg.ChunkSizeTokens
	overlap := cfg.ChunkOverlapTokens
	var chunks []chunker.Chunk
	chunkIdx := 0

	if filePath != "" {
		switch format {
		case "epub":
			for _, ch := range parser.ParseEPUB(filePath) {
				sectionTitle := fmt.Sprintf("%s (chapter %d)", book.Title, ch.ChapterNumber)
				sectionChunks := chunker.ChunkPlain(ch.Text, sectionTitle, chunkSize, overlap)
				for j := range sectionChunks {
					sectionChunks[j].ChunkIndex = chunkIdx
					meta := copyMeta(bookMeta)
					meta["chapter_number"] = ch.ChapterNumber
					sectionChunks[j].Metadata = meta
					chunks = append(chunks, sectionChunks[j])
					chunkIdx++
				}
			}
		case "pdf":
			pages, stats := parser.ParsePDF(ctx, filePath, pdfOptions(cfg))
			if res != nil {
				res.PDFNoTextPages += stats.NoTextPages
			}
			for _, pg := range pages {
				sectionTitle := fmt.Sprintf("%s (page %d)", book.Title, pg.PageNumber)
				sectionChunks := chunker.ChunkPlain(pg.Text, sectionTitle, chunkSize, overlap)
				for j := range sectionChunks {
					sectionChunks[j].ChunkIndex = chunkIdx
					meta := copyMeta(bookMeta)
					meta["page_number"] = pg.PageNumber
					if pg.OCR {
						meta["ocr"] = true
					}
					sectionChunks[j].Metadata = meta
					chunks = append(chunks, sectionChunks[j])
					chunkIdx++
				}
			}
		}
	}

	if book.Description != "" {
		descChunks := chunker.ChunkPlain(book.Description, fmt.Sprintf("%s (description)", book.Title), chunkSize, overlap)
		for j := range descChunks {
			descChunks[j].ChunkIndex = chunkIdx
			meta := copyMeta(bookMeta)
			meta["chunk_type"] = "description"
			descChunks[j].Metadata = meta
			chunks = append(chunks, descChunks[j])
			chunkIdx++
		}
	}
	return chunks
}

func copyMeta(m map[string]any) map[string]any {
	c := make(map[string]any, len(m))
	for k, v := range m {
		c[k] = v
	}
	return c
}
