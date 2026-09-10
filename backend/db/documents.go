package db

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// Paging defaults shared by the Browse chunk stream and the Library source
// list. A page is deliberately small: the frontend keeps a bounded window of
// pages, and the whole point of paging is that no query ever materialises a
// whole collection.
const (
	defaultPageSize = 100
	maxPageSize     = 500
)

func clampPageSize(limit int) int {
	if limit <= 0 {
		return defaultPageSize
	}
	if limit > maxPageSize {
		return maxPageSize
	}
	return limit
}

// DocRow is one chunk row in the Browse stream. It carries no Content and no
// Metadata on purpose: the list is paged indefinitely, so the chunk text is
// fetched per selection by GetDocument.
type DocRow struct {
	ID           int64
	SourceID     int64
	CollectionID int64
	ChunkIndex   int
	Title        string
	SourcePath   string
	SourceType   string
}

// DocPage is one keyset page of a collection's chunk stream, in stream order.
// Before and After are opaque cursors for the neighbouring pages; "" means the
// stream ends on that side, which is how the caller knows it has reached an end.
type DocPage struct {
	Rows   []DocRow
	Before string
	After  string
}

// FullDoc is one complete chunk, content included, for the reading pane.
type FullDoc struct {
	ID           int64
	SourceID     int64
	CollectionID int64
	ChunkIndex   int
	Title        string
	Content      string
	Metadata     string
}

// docCursor is a keyset position in the chunk stream: the (source_id,
// chunk_index) pair of the row it points at. The pair is UNIQUE on documents,
// so it is a total order and needs no id tiebreaker.
type docCursor struct {
	sourceID   int64
	chunkIndex int64
}

// encode renders the cursor as the opaque "<source_id>:<chunk_index>" string
// the frontend stores and hands back. The zero cursor renders as "".
func (c docCursor) encode() string {
	if c.sourceID == 0 && c.chunkIndex == 0 {
		return ""
	}
	return strconv.FormatInt(c.sourceID, 10) + ":" + strconv.FormatInt(c.chunkIndex, 10)
}

// parseDocCursor decodes an opaque cursor. An empty or malformed string means
// "the start of the stream", so a stale cursor degrades to the first page
// instead of failing the query.
func parseDocCursor(s string) docCursor {
	src, chunk, ok := strings.Cut(s, ":")
	if !ok {
		return docCursor{}
	}
	sourceID, err1 := strconv.ParseInt(src, 10, 64)
	chunkIndex, err2 := strconv.ParseInt(chunk, 10, 64)
	if err1 != nil || err2 != nil {
		return docCursor{}
	}
	return docCursor{sourceID: sourceID, chunkIndex: chunkIndex}
}

// ListDocumentsPage returns one keyset page of a collection's chunks ordered by
// (source_id, chunk_index). backward=true pages towards the start of the
// stream; cursor is an opaque position handed back by an earlier call ("" for
// the first page). The returned page is always in stream order, so the caller
// appends forward pages and prepends backward ones.
//
// The source path and type come along so the UI can label a group without a
// second query, and content is left out so a page stays small.
func ListDocumentsPage(conn *sql.DB, collectionID int64, cursor string, limit int, backward bool) (DocPage, error) {
	limit = clampPageSize(limit)
	// The empty cursor means "start of the stream", not "end of the stream", so
	// page backwards from it and the answer is nothing. Falling through would
	// run an unfiltered DESC query and hand back the last page.
	if backward && cursor == "" {
		return DocPage{}, nil
	}
	cur := parseDocCursor(cursor)

	query := `SELECT d.id, d.source_id, d.collection_id, d.chunk_index, d.title,
			s.source_path, s.source_type
		FROM documents d
		JOIN sources s ON s.id = d.source_id
		WHERE d.collection_id = ?`
	args := []any{collectionID}

	if cursor != "" {
		if backward {
			query += ` AND (d.source_id, d.chunk_index) < (?, ?)`
		} else {
			query += ` AND (d.source_id, d.chunk_index) > (?, ?)`
		}
		args = append(args, cur.sourceID, cur.chunkIndex)
	}
	if backward {
		query += ` ORDER BY d.source_id DESC, d.chunk_index DESC`
	} else {
		query += ` ORDER BY d.source_id, d.chunk_index`
	}
	// One extra row is the cheapest way to know whether another page exists.
	query += ` LIMIT ?`
	args = append(args, limit+1)

	rows, err := conn.Query(query, args...)
	if err != nil {
		return DocPage{}, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	out := make([]DocRow, 0, limit)
	for rows.Next() {
		var r DocRow
		var title sql.NullString
		if err := rows.Scan(&r.ID, &r.SourceID, &r.CollectionID, &r.ChunkIndex,
			&title, &r.SourcePath, &r.SourceType); err != nil {
			return DocPage{}, fmt.Errorf("scan document row: %w", err)
		}
		r.Title = title.String
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return DocPage{}, fmt.Errorf("read document rows: %w", err)
	}

	more := len(out) > limit
	if more {
		out = out[:limit]
	}
	if backward {
		// The query walked backwards, so flip the page into stream order.
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}

	page := DocPage{Rows: out}
	if len(out) == 0 {
		return page, nil
	}
	first := docCursor{sourceID: out[0].SourceID, chunkIndex: int64(out[0].ChunkIndex)}
	last := docCursor{
		sourceID:   out[len(out)-1].SourceID,
		chunkIndex: int64(out[len(out)-1].ChunkIndex),
	}
	// The probe row only reports the direction we walked. The opposite side is
	// inferred from the cursor we came from: if we paged from a cursor there is
	// a page back there, unless a concurrent delete removed it — mutations
	// reset the caller's window, so this never strands it.
	if backward {
		if more {
			page.Before = first.encode()
		}
		if cursor != "" {
			page.After = last.encode()
		}
		return page, nil
	}
	if more {
		page.After = last.encode()
	}
	if cursor != "" {
		page.Before = first.encode()
	}
	return page, nil
}

// GetDocument returns one complete chunk (content + raw metadata JSON) for the
// reading pane. The boolean is false when the id no longer exists.
func GetDocument(conn *sql.DB, id int64) (FullDoc, bool, error) {
	var d FullDoc
	var title, meta sql.NullString
	err := conn.QueryRow(`SELECT id, source_id, collection_id, chunk_index, title, content, metadata
		FROM documents WHERE id = ?`, id).
		Scan(&d.ID, &d.SourceID, &d.CollectionID, &d.ChunkIndex, &title, &d.Content, &meta)
	switch {
	case err == sql.ErrNoRows:
		return FullDoc{}, false, nil
	case err != nil:
		return FullDoc{}, false, fmt.Errorf("get document: %w", err)
	}
	d.Title = title.String
	d.Metadata = meta.String
	return d, true, nil
}

// SourceRow is one indexed source. Sources are already light, so paging them is
// only about bounding the rendered list.
type SourceRow struct {
	ID           int64
	CollectionID int64
	SourceType   string
	Path         string
	Chunks       int
	LastIndexed  string
}

// SourcePage is one page of a collection's sources, ordered by path.
type SourcePage struct {
	Rows   []SourceRow
	Before string
	After  string
}

// ListSourcesPage returns one page of a collection's sources ordered by
// source_path. That column is UNIQUE per collection, so it is a total order and
// the cursor can be the path itself.
func ListSourcesPage(conn *sql.DB, collectionID int64, cursor string, limit int, backward bool) (SourcePage, error) {
	limit = clampPageSize(limit)
	if backward && cursor == "" {
		return SourcePage{}, nil
	}

	query := `SELECT s.id, s.collection_id, s.source_type, s.source_path,
			(SELECT COUNT(*) FROM documents d WHERE d.source_id = s.id),
			s.last_indexed_at
		FROM sources s
		WHERE s.collection_id = ?`
	args := []any{collectionID}

	if cursor != "" {
		if backward {
			query += ` AND s.source_path < ?`
		} else {
			query += ` AND s.source_path > ?`
		}
		args = append(args, cursor)
	}
	if backward {
		query += ` ORDER BY s.source_path DESC`
	} else {
		query += ` ORDER BY s.source_path`
	}
	query += ` LIMIT ?`
	args = append(args, limit+1)

	rows, err := conn.Query(query, args...)
	if err != nil {
		return SourcePage{}, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()

	out := make([]SourceRow, 0, limit)
	for rows.Next() {
		var s SourceRow
		var last sql.NullString
		if err := rows.Scan(&s.ID, &s.CollectionID, &s.SourceType, &s.Path,
			&s.Chunks, &last); err != nil {
			return SourcePage{}, fmt.Errorf("scan source row: %w", err)
		}
		s.LastIndexed = last.String
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return SourcePage{}, fmt.Errorf("read source rows: %w", err)
	}

	more := len(out) > limit
	if more {
		out = out[:limit]
	}
	if backward {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}

	page := SourcePage{Rows: out}
	if len(out) == 0 {
		return page, nil
	}
	if backward {
		if more {
			page.Before = out[0].Path
		}
		if cursor != "" {
			page.After = out[len(out)-1].Path
		}
		return page, nil
	}
	if more {
		page.After = out[len(out)-1].Path
	}
	if cursor != "" {
		page.Before = out[0].Path
	}
	return page, nil
}
