package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"runtime"

	"vectile/backend/appdata"
	"vectile/backend/db"
)

// AppService reports app/status/library data.
type AppService struct{ core *Core }

// NewAppService creates an AppService bound to the shared core.
func NewAppService(core *Core) *AppService { return &AppService{core: core} }

// GetVersion returns the app version.
func (s *AppService) GetVersion() string { return Version }

// GetPlatform returns the OS ("windows", "darwin", "linux").
func (s *AppService) GetPlatform() string { return runtime.GOOS }

// GetCPUCount returns the number of logical CPUs available to the process —
// the ceiling the Settings UI uses for the model's CPU-threads slider
// (0 = use all of them).
func (s *AppService) GetCPUCount() int { return runtime.NumCPU() }

// GetStatus returns the status summary: library stats + model state.
func (s *AppService) GetStatus() Status {
	var collections, sources, chunks int
	_ = db.DB.QueryRow("SELECT COUNT(*) FROM collections").Scan(&collections)
	_ = db.DB.QueryRow("SELECT COUNT(*) FROM sources").Scan(&sources)
	_ = db.DB.QueryRow("SELECT COUNT(*) FROM documents").Scan(&chunks)

	var size int64
	if info, err := os.Stat(appdata.DBPath()); err == nil {
		size = info.Size()
	}


	var lastIndexed sql.NullString
	_ = db.DB.QueryRow(`SELECT MAX(last_indexed_at) FROM sources
		WHERE last_indexed_at IS NOT NULL AND last_indexed_at != ''`).Scan(&lastIndexed)

	modelErr := ""
	if err := s.core.Embedder.LoadError(); err != nil {
		modelErr = err.Error()
	}
	modelName := s.core.Cfg.EmbeddingModel
	if active, ok, err := db.GetActiveModel(db.DB); err == nil && ok {
		modelName = active.Name
		if active.Dimensions > 0 {
			modelName = fmt.Sprintf("%s · %dd", active.Name, active.Dimensions)
		}
	}
	return Status{
		Collections: collections,
		Sources:     sources,
		Chunks:      chunks,
		DBSize:      size,
		ModelState:  s.core.Embedder.State(),
		ModelName:   modelName,
		ModelPath:   s.core.Embedder.ModelPath(),
		ModelError:  modelErr,
		LastIndexed: lastIndexed.String,
	}
}

// ListCollections returns all indexed collections with counts, ordered by name.
func (s *AppService) ListCollections() ([]Collection, error) {
	rows, err := db.DB.Query(`
		SELECT c.id, c.name, c.collection_type, c.description, c.created_at,
			(SELECT COUNT(*) FROM sources s WHERE s.collection_id = c.id),
			(SELECT COUNT(*) FROM documents d WHERE d.collection_id = c.id),
			(SELECT MAX(s2.last_indexed_at) FROM sources s2
			  WHERE s2.collection_id = c.id AND s2.last_indexed_at IS NOT NULL AND s2.last_indexed_at != ''),
			CASE WHEN EXISTS(SELECT 1 FROM documents d WHERE d.collection_id = c.id)
			      AND NOT EXISTS(SELECT 1 FROM documents d JOIN vec_documents v
			                     ON v.document_id = d.id WHERE d.collection_id = c.id)
			     THEN 1 ELSE 0 END
		FROM collections c
		ORDER BY c.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Collection
	for rows.Next() {
		var c Collection
		var desc, created, last sql.NullString
		var needsReindex int
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &desc, &created, &c.Sources, &c.Chunks, &last, &needsReindex); err != nil {
			return nil, err
		}
		if desc.Valid {
			c.Description = desc.String
		}
		if created.Valid {
			c.Created = created.String
		}
		c.LastIndexed = last.String
		c.NeedsReindex = needsReindex == 1
		c.Enabled = s.core.Cfg.IsCollectionEnabled(c.Name)
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListSourcesPage returns one page of a collection's sources ordered by path.
// backward=true pages towards the start of the list; cursor is opaque and comes
// from a previous page ("" for the first page).
func (s *AppService) ListSourcesPage(collectionID int64, cursor string, backward bool) (SourcePage, error) {
	page, err := db.ListSourcesPage(db.DB, collectionID, cursor, 0, backward)
	if err != nil {
		return SourcePage{}, err
	}
	out := SourcePage{
		Sources: make([]Source, 0, len(page.Rows)),
		Before:  page.Before,
		After:   page.After,
	}
	for _, r := range page.Rows {
		out.Sources = append(out.Sources, Source{
			ID:           r.ID,
			CollectionID: r.CollectionID,
			SourceType:   r.SourceType,
			Path:         r.Path,
			Chunks:       r.Chunks,
			LastIndexed:  r.LastIndexed,
		})
	}
	return out, nil
}

// ListDocumentsPage returns one page of a collection's chunk stream, ordered by
// source then chunk. The rows deliberately carry no content: the stream is
// paged indefinitely and the selected chunk's text comes from GetDocument.
func (s *AppService) ListDocumentsPage(collectionID int64, cursor string, backward bool) (DocumentPage, error) {
	page, err := db.ListDocumentsPage(db.DB, collectionID, cursor, 0, backward)
	if err != nil {
		return DocumentPage{}, err
	}
	out := DocumentPage{
		Documents: make([]DocumentSummary, 0, len(page.Rows)),
		Before:    page.Before,
		After:     page.After,
	}
	for _, r := range page.Rows {
		out.Documents = append(out.Documents, DocumentSummary{
			ID:           r.ID,
			SourceID:     r.SourceID,
			CollectionID: r.CollectionID,
			ChunkIndex:   r.ChunkIndex,
			Title:        r.Title,
			SourcePath:   r.SourcePath,
			SourceType:   r.SourceType,
		})
	}
	return out, nil
}

// GetDocument returns one chunk with its full text and metadata, for the
// reading pane.
func (s *AppService) GetDocument(id int64) (Document, error) {
	d, ok, err := db.GetDocument(db.DB, id)
	if err != nil {
		return Document{}, err
	}
	if !ok {
		return Document{}, fmt.Errorf("chunk %d no longer exists", id)
	}
	return Document{
		ID:           d.ID,
		SourceID:     d.SourceID,
		CollectionID: d.CollectionID,
		ChunkIndex:   d.ChunkIndex,
		Title:        d.Title,
		Content:      d.Content,
		Metadata:     parseMetaString(d.Metadata),
	}, nil
}

// GetModelError returns the embedder's load error (for the status pill).
func (s *AppService) GetModelError() string {
	if err := s.core.Embedder.LoadError(); err != nil {
		return err.Error()
	}
	return ""
}

// OpenFile opens a file or folder with the OS default application.
func (s *AppService) OpenFile(path string) error {
	if err := ensurePathExists(path); err != nil {
		return err
	}
	return openPath(path)
}

// RevealInFolder selects a file in the OS file manager (opens the parent
// folder for a directory).
func (s *AppService) RevealInFolder(path string) error {
	if err := ensurePathExists(path); err != nil {
		return err
	}
	return revealPath(path)
}

// ensurePathExists guards the open/reveal helpers so a stale indexed path
// gets a useful error instead of a silent no-op.
func ensurePathExists(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("path not found: %s", path)
	}
	return nil
}

// parseMeta decodes a JSON metadata column into a value the frontend can use.
func parseMeta(ns sql.NullString) any {
	if !ns.Valid {
		return nil
	}
	return parseMetaString(ns.String)
}

// parseMetaString does the decoding; an empty or unparsable value falls back to
// nil / the raw string so one bad row can't blank the reading pane.
func parseMetaString(raw string) any {
	if raw == "" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return raw
	}
	return m
}
