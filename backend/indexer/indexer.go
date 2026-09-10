// Package indexer orchestrates indexing for vectile: each source indexer
// reads content, chunks it, embeds the chunks via the in-process llama.go
// embedder, and stores everything in the SQLite database.
package indexer

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vectile/backend/chunker"
	"vectile/backend/config"
	"vectile/backend/db"
	"vectile/backend/parser"
)

// Embedder is the subset of the llama-go embedder the indexer uses.
type Embedder interface {
	EmbedBatch(texts []string) ([][]float32, error)
}

// ProgressCallback is called per item with (current, total, itemName).
type ProgressCallback func(current, total int, itemName string)

// IndexResult summarises an indexing run.
type IndexResult struct {
	Indexed       int
	Skipped       int
	Errors        int
	TotalFound    int
	ErrorMessages []string
}

func (r *IndexResult) String() string {
	return fmt.Sprintf("Indexed: %d, Skipped: %d, Errors: %d, Total found: %d",
		r.Indexed, r.Skipped, r.Errors, r.TotalFound)
}

// Merge adds another result into this one.
func (r *IndexResult) Merge(other *IndexResult) {
	r.Indexed += other.Indexed
	r.Skipped += other.Skipped
	r.Errors += other.Errors
	r.TotalFound += other.TotalFound
	r.ErrorMessages = append(r.ErrorMessages, other.ErrorMessages...)
}

func getOrCreate(conn *sql.DB, name, collType string) (int64, error) {
	id, err := db.GetOrCreateCollection(conn, name, collType, nil, nil)
	if err != nil {
		slog.Error("failed to get/create collection", "name", name, "err", err)
		return 0, err
	}
	return id, nil
}

func failedResult(err error) *IndexResult {
	return &IndexResult{Errors: 1, ErrorMessages: []string{err.Error()}}
}

// CheckNameConflict fails if this collection name is claimed by more than one
// config section. Collection names are unique, so indexing under an ambiguous
// name would merge two unrelated corpora into one collection.
func CheckNameConflict(cfg *config.Config, name string) error {
	for _, c := range cfg.CollectionNameConflicts() {
		if c.Name == name {
			return fmt.Errorf("collection name conflict: %s — rename one of them in config", c)
		}
	}
	return nil
}

// clearForRebuild wipes a collection's data when force is given, so the
// per-batch purge can be skipped. Returns whether the collection was cleared.
func clearForRebuild(conn *sql.DB, collectionID int64, force bool) bool {
	if !force {
		return false
	}
	if err := db.ClearCollectionData(conn, collectionID); err != nil {
		slog.Error("could not clear collection for rebuild, falling back to per-batch purge", "err", err)
		return false
	}
	return true
}

// clearRepoForRebuild is clearForRebuild scoped to one repository, since a
// code collection can hold many and rebuilding one must not wipe its siblings.
func clearRepoForRebuild(conn *sql.DB, collectionID int64, repoPath string, force bool) bool {
	if !force {
		return false
	}
	prefixes := []string{
		repoPath + string(filepath.Separator),
		"git://" + repoPath + "#",
	}
	if err := db.ClearSourcesWithPrefix(conn, collectionID, prefixes); err != nil {
		slog.Error("could not clear repo for rebuild, falling back to per-batch purge",
			"repo", repoPath, "err", err)
		return false
	}
	return true
}

// fileHash computes SHA256 of a file.
func fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// isHidden checks if any path component starts with a dot.
func isHidden(path string) bool {
	for _, part := range strings.Split(path, string(filepath.Separator)) {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

// excludedDirNames turns a list of directory names into a lookup set. Names
// are matched whole (not as substrings) and any depth, so "node_modules"
// prunes every copy in a tree.
func excludedDirNames(names []string) map[string]bool {
	if len(names) == 0 {
		return nil
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		if n = strings.TrimSpace(n); n != "" {
			set[n] = true
		}
	}
	return set
}

// collectFiles walks directories recursively and collects files with supported
// extensions. Cloud placeholders are skipped when skipPlaceholders is true, and
// any directory named in excludeDirs is pruned whole. An explicitly configured
// path is always walked, even if its own name is on the exclusion list.
func collectFiles(paths []string, skipPlaceholders bool, excludeDirs map[string]bool) []string {
	var files []string
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			slog.Warn("path does not exist", "path", p)
			continue
		}
		if !info.IsDir() {
			if !isHidden(p) && parser.SourceTypeForPath(p) != "" {
				if skipPlaceholders && isCloudPlaceholder(info) {
					slog.Warn("skipping cloud-only file (not downloaded)", "path", p)
					continue
				}
				files = append(files, p)
			}
			continue
		}
		filepath.Walk(p, func(fp string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if fi.IsDir() {
				if fp != p && excludeDirs[fi.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if isHidden(fp) {
				return nil
			}
			if parser.SourceTypeForPath(fp) == "" {
				return nil
			}
			if skipPlaceholders && isCloudPlaceholder(fi) {
				return nil
			}
			files = append(files, fp)
			return nil
		})
	}
	return files
}

// parseAndChunk dispatches a file to the right parser and returns chunks.
func parseAndChunk(path, sourceType string, cfg *config.Config) []chunker.Chunk {
	chunkSize := cfg.ChunkSizeTokens
	overlap := cfg.ChunkOverlapTokens
	title := filepath.Base(path)

	switch sourceType {
	case "markdown":
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Error("failed to read markdown file", "path", path, "err", err)
			return nil
		}
		doc := parser.ParseMarkdown(string(data), filepath.Base(path))
		chunks := chunker.ChunkMarkdown(doc.BodyText, doc.Title, chunkSize, overlap)
		for i := range chunks {
			for k, v := range doc.Frontmatter {
				if k == "tags" {
					continue // handled separately below
				}
				chunks[i].Metadata[k] = v
			}
			if len(doc.Tags) > 0 {
				chunks[i].Metadata["tags"] = doc.Tags
			}
			if len(doc.Links) > 0 {
				chunks[i].Metadata["links"] = doc.Links
			}
		}
		return chunks

	case "pdf":
		pages := parser.ParsePDF(path)
		if len(pages) == 0 {
			return nil
		}
		var chunks []chunker.Chunk
		chunkIdx := 0
		for _, page := range pages {
			pageTitle := fmt.Sprintf("%s (page %d)", title, page.PageNumber)
			pageChunks := chunker.ChunkPlain(page.Text, pageTitle, chunkSize, overlap)
			for i := range pageChunks {
				pageChunks[i].ChunkIndex = chunkIdx
				pageChunks[i].Metadata["page_number"] = page.PageNumber
				chunks = append(chunks, pageChunks[i])
				chunkIdx++
			}
		}
		return chunks

	case "docx":
		doc := parser.ParseDocx(path)
		if doc.Text == "" {
			return nil
		}
		return chunker.ChunkPlain(doc.Text, title, chunkSize, overlap)

	case "html":
		text := parser.ParseHTML(path)
		if text == "" {
			return nil
		}
		return chunker.ChunkPlain(text, title, chunkSize, overlap)

	case "xlsx":
		doc := parser.ParseXLSX(path)
		if strings.TrimSpace(doc.Text) == "" {
			return nil
		}
		// Each sheet is a "## Sheet: <name>" section, so ChunkMarkdown splits
		// the workbook sheet by sheet and labels chunks with the sheet name.
		return chunker.ChunkMarkdown(doc.Text, title, chunkSize, overlap)

	case "pptx":
		doc := parser.ParsePPTX(path)
		if strings.TrimSpace(doc.Text) == "" {
			return nil
		}
		return chunker.ChunkMarkdown(doc.Text, title, chunkSize, overlap)

	case "ipynb":
		doc := parser.ParseNotebook(path)
		if strings.TrimSpace(doc.Text) == "" {
			return nil
		}
		return chunker.ChunkMarkdown(doc.Text, title, chunkSize, overlap)

	case "xml":
		doc := parser.ParseXMLFile(path)
		if doc.Text == "" {
			return nil
		}
		return chunker.ChunkPlain(doc.Text, title, chunkSize, overlap)

	case "sql":
		doc := parser.ParseSQLFile(path)
		if doc.Text == "" {
			return nil
		}
		return chunker.ChunkPlain(doc.Text, title, chunkSize, overlap)

	case "shell":
		doc := parser.ParseShellFile(path)
		if doc.Text == "" {
			return nil
		}
		return chunker.ChunkPlain(doc.Text, title, chunkSize, overlap)

	case "plaintext":
		// CSV and JSON get structured normalization (still stored as
		// plaintext); every other text file is indexed raw.
		var text string
		switch strings.ToLower(filepath.Ext(path)) {
		case ".csv":
			text = parser.ParseCSVText(path)
		case ".json":
			text = parser.ParseJSONText(path)
		default:
			text = parser.ParsePlaintext(path)
		}
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return chunker.ChunkPlain(text, title, chunkSize, overlap)
	}

	slog.Warn("unknown source type", "type", sourceType, "path", path)
	return nil
}

// isSourceCurrent reports whether a source is already indexed at the given
// modification time — a cheap pre-check that runs before hashing.
func isSourceCurrent(conn *sql.DB, collectionID int64, sourcePath, mtime string) bool {
	if mtime == "" {
		return false
	}
	var storedMtime sql.NullString
	err := conn.QueryRow(
		"SELECT file_modified_at FROM sources WHERE collection_id = ? AND source_path = ?",
		collectionID, sourcePath,
	).Scan(&storedMtime)
	if err != nil {
		return false
	}
	return storedMtime.Valid && storedMtime.String == mtime
}

// isSourceUnchanged checks if a source's file hash matches.
func isSourceUnchanged(conn *sql.DB, collectionID int64, sourcePath, currentHash string) bool {
	var storedHash sql.NullString
	err := conn.QueryRow(
		"SELECT file_hash FROM sources WHERE collection_id = ? AND source_path = ?",
		collectionID, sourcePath,
	).Scan(&storedHash)
	if err != nil {
		return false
	}
	return storedHash.Valid && storedHash.String == currentHash
}

// upsertSourceRow inserts or updates a source row. When purged is true the
// caller already deleted the old documents and vectors for the batch.
func upsertSourceRow(conn *sql.DB, collectionID int64, sourcePath, sourceType, fileH, mtime string, purged bool) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	var existingID sql.NullInt64
	err := conn.QueryRow(
		"SELECT id FROM sources WHERE collection_id = ? AND source_path = ?",
		collectionID, sourcePath,
	).Scan(&existingID)

	if err == nil && existingID.Valid {
		sourceID := existingID.Int64
		if !purged {
			deleteOldDocs(conn, sourceID)
		}
		_, err = conn.Exec(
			"UPDATE sources SET file_hash = ?, file_modified_at = ?, last_indexed_at = ?, source_type = ? WHERE id = ?",
			fileH, mtime, now, sourceType, sourceID,
		)
		if err != nil {
			return 0, err
		}
		return sourceID, nil
	}

	res, err := conn.Exec(
		"INSERT INTO sources (collection_id, source_type, source_path, file_hash, file_modified_at, last_indexed_at) VALUES (?, ?, ?, ?, ?, ?)",
		collectionID, sourceType, sourcePath, fileH, mtime, now,
	)
	if err != nil {
		return 0, fmt.Errorf("insert source: %w", err)
	}
	return res.LastInsertId()
}

// deleteOldDocs removes documents and their vector entries for a source.
func deleteOldDocs(conn *sql.DB, sourceID int64) {
	rows, err := conn.Query("SELECT id FROM documents WHERE source_id = ?", sourceID)
	if err != nil {
		return
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	if len(ids) > 0 {
		args := make([]any, len(ids))
		for i, id := range ids {
			args[i] = id
		}
		_ = db.DeleteEmbeddings(conn, args)
	}
	_, _ = conn.Exec("DELETE FROM documents WHERE source_id = ?", sourceID)
}

// fileToItem prepares one file for indexing, or returns nil if it should be
// skipped — unchanged since the last run, or nothing extractable. The cheap
// checks run first: stat, then hash, then parse (the expensive step).
func fileToItem(conn *sql.DB, cfg *config.Config, filePath string, collectionID int64, force bool) *indexItem {
	absPath, _ := filepath.Abs(filePath)

	info, statErr := os.Stat(filePath)
	mtime := ""
	if statErr == nil {
		mtime = info.ModTime().UTC().Format(time.RFC3339)
	}

	if !force && isSourceCurrent(conn, collectionID, absPath, mtime) {
		return nil
	}

	fh, err := fileHash(filePath)
	if err != nil {
		slog.Warn("cannot hash file, skipping", "path", filePath, "err", err)
		return nil
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	sourceType := parser.ExtensionMap[ext]
	if sourceType == "" {
		sourceType = "plaintext"
	}

	// mtime moved but content identical — record the new mtime, don't re-embed.
	if !force && isSourceUnchanged(conn, collectionID, absPath, fh) {
		_, _ = conn.Exec(
			"UPDATE sources SET file_modified_at = ?, last_indexed_at = ? WHERE collection_id = ? AND source_path = ?",
			mtime, time.Now().UTC().Format(time.RFC3339), collectionID, absPath,
		)
		return nil
	}

	chunks := parseAndChunk(filePath, sourceType, cfg)
	if len(chunks) == 0 {
		slog.Warn("no content extracted, skipping", "path", filePath)
		return nil
	}

	return &indexItem{
		SourcePath: absPath,
		SourceType: sourceType,
		Chunks:     chunks,
		FileHash:   fh,
		Mtime:      mtime,
	}
}

// IndexProject indexes documents from file paths into a named project
// collection.
func IndexProject(ctx context.Context, conn *sql.DB, cfg *config.Config, collectionName string, paths []string, force bool, progress ProgressCallback, embedder Embedder) *IndexResult {
	if err := CheckNameConflict(cfg, collectionName); err != nil {
		slog.Error("refusing to index", "name", collectionName, "err", err)
		return failedResult(err)
	}

	collectionID, err := db.GetOrCreateCollection(conn, collectionName, "project", nil, nil)
	if err != nil {
		slog.Error("failed to get/create collection", "name", collectionName, "err", err)
		return failedResult(err)
	}

	files := collectFiles(paths, cfg.SkipCloudPlaceholders, excludedDirNames(cfg.ProjectExcludeFolders))
	result := &IndexResult{TotalFound: len(files)}
	cleared := clearForRebuild(conn, collectionID, force)

	slog.Info("project indexer: found files", "count", len(files), "collection", collectionName)
	indexItemsBatched(ctx, conn, cfg, collectionID, collectionName, len(files),
		func(i int) *indexItem { return fileToItem(conn, cfg, files[i], collectionID, force) },
		embedder, result, progress, cleared)
	return result
}

// isCloudPlaceholder reports whether fi is a cloud-only (not locally
// downloaded) file. Only macOS exposes this flag; everywhere else it is
// always false.
func isCloudPlaceholder(fi os.FileInfo) bool {
	_ = fi
	return false
}

// expandPath expands a leading ~/ to the home directory.
func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

// fileExists reports whether path exists and is not a directory.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// deleteSourceByID deletes a source and all its documents and vectors.
func deleteSourceByID(conn *sql.DB, sourceID int64) {
	deleteOldDocs(conn, sourceID)
	_, _ = conn.Exec("DELETE FROM sources WHERE id = ?", sourceID)
}

// deleteSource deletes a source by collection ID and source path.
func deleteSource(conn *sql.DB, collectionID int64, sourcePath string) {
	var sourceID sql.NullInt64
	err := conn.QueryRow(
		"SELECT id FROM sources WHERE collection_id = ? AND source_path = ?",
		collectionID, sourcePath,
	).Scan(&sourceID)
	if err != nil || !sourceID.Valid {
		return
	}
	deleteSourceByID(conn, sourceID.Int64)
}
