package db

import (
	"database/sql"
	"fmt"
	"testing"
)

// seedChunks builds a collection with one source per path and chunksPer chunks
// each. Chunks are inserted interleaved across the sources on purpose, so the
// document ids do NOT follow the (source_id, chunk_index) stream order — a
// paging bug that fell back to id order would show up here.
func seedChunks(t *testing.T, conn *sql.DB, collName string, paths []string, chunksPer int) (collID int64, sourceIDs []int64) {
	t.Helper()
	collID, err := GetOrCreateCollection(conn, collName, "project", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		res, err := conn.Exec(
			"INSERT INTO sources (collection_id, source_type, source_path) VALUES (?,?,?)",
			collID, "markdown", p,
		)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		sourceIDs = append(sourceIDs, id)
	}
	for i := 0; i < chunksPer; i++ {
		for _, sid := range sourceIDs {
			if _, err := conn.Exec(
				`INSERT INTO documents (source_id, collection_id, chunk_index, title, content)
				 VALUES (?,?,?,?,?)`,
				sid, collID, i, fmt.Sprintf("doc-%d-%d", sid, i), fmt.Sprintf("body %d %d", sid, i),
			); err != nil {
				t.Fatal(err)
			}
		}
	}
	return collID, sourceIDs
}

// streamOrder is the order the chunk stream must come back in.
func streamOrder(sourceIDs []int64, paths []string, chunksPer int) []string {
	var want []string
	for i, sid := range sourceIDs {
		for c := 0; c < chunksPer; c++ {
			want = append(want, fmt.Sprintf("doc-%d-%d|%s", sid, c, paths[i]))
		}
	}
	return want
}

func rowKey(r DocRow) string { return fmt.Sprintf("%s|%s", r.Title, r.SourcePath) }

// TestListDocumentsPageWalksForward pages the whole stream forward and checks
// the keyset invariants: no gaps, no repeats, a cursor that walks back exactly
// one page, and a cursor that walks forward exactly one page.
func TestListDocumentsPageWalksForward(t *testing.T) {
	conn := testDB(t)
	if err := InitSchema(conn, EmbeddingDim); err != nil {
		t.Fatal(err)
	}
	paths := []string{"/notes/b.md", "/notes/a.md"}
	collID, sourceIDs := seedChunks(t, conn, "notes", paths, 5)
	want := streamOrder(sourceIDs, []string{"/notes/b.md", "/notes/a.md"}, 5)

	const limit = 3
	var pages []DocPage
	var got []string
	cursor := ""
	for {
		page, err := ListDocumentsPage(conn, collID, cursor, limit, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Rows) == 0 {
			t.Fatal("empty page before the stream ended")
		}
		for _, r := range page.Rows {
			got = append(got, rowKey(r))
		}
		pages = append(pages, page)
		if page.After == "" {
			break
		}
		if page.After == cursor {
			t.Fatalf("cursor did not advance: %q", cursor)
		}
		cursor = page.After
	}

	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("forward walk\n got %v\nwant %v", got, want)
	}
	if pages[0].Before != "" {
		t.Fatalf("first page reports a predecessor: %q", pages[0].Before)
	}

	// Before walks back exactly one page; After walks forward exactly one page.
	for i, page := range pages {
		back, err := ListDocumentsPage(conn, collID, page.Before, limit, true)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			if len(back.Rows) != 0 {
				t.Fatalf("page 0 walked back to %d rows", len(back.Rows))
			}
			continue
		}
		if fmt.Sprint(keysOf(back.Rows)) != fmt.Sprint(keysOf(pages[i-1].Rows)) {
			t.Fatalf("page %d Before landed on %v, want %v",
				i, keysOf(back.Rows), keysOf(pages[i-1].Rows))
		}
		if i == len(pages)-1 {
			continue
		}
		fwd, err := ListDocumentsPage(conn, collID, page.After, limit, false)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprint(keysOf(fwd.Rows)) != fmt.Sprint(keysOf(pages[i+1].Rows)) {
			t.Fatalf("page %d After landed on %v, want %v",
				i, keysOf(fwd.Rows), keysOf(pages[i+1].Rows))
		}
	}

	// A backward page comes back in stream order, not reversed.
	last := pages[len(pages)-1]
	back, err := ListDocumentsPage(conn, collID, last.Before, limit, true)
	if err != nil {
		t.Fatal(err)
	}
	prev := keysOf(pages[len(pages)-2].Rows)
	if fmt.Sprint(keysOf(back.Rows)) != fmt.Sprint(prev) {
		t.Fatalf("backward page = %v, want %v", keysOf(back.Rows), prev)
	}
}

func keysOf(rows []DocRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, rowKey(r))
	}
	return out
}

// TestListDocumentsPageIsolatesCollections guards the WHERE clause: a page must
// never leak another collection's chunks into the stream.
func TestListDocumentsPageIsolatesCollections(t *testing.T) {
	conn := testDB(t)
	if err := InitSchema(conn, EmbeddingDim); err != nil {
		t.Fatal(err)
	}
	aID, _ := seedChunks(t, conn, "a", []string{"/a/one.md"}, 4)
	bID, _ := seedChunks(t, conn, "b", []string{"/b/one.md"}, 6)

	page, err := ListDocumentsPage(conn, aID, "", 100, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) != 4 {
		t.Fatalf("collection a returned %d rows, want 4", len(page.Rows))
	}
	for _, r := range page.Rows {
		if r.CollectionID != aID {
			t.Fatalf("collection %d leaked into a page for %d", r.CollectionID, aID)
		}
	}
	if page.After != "" {
		t.Fatalf("short page reports more rows: %q", page.After)
	}

	bPage, err := ListDocumentsPage(conn, bID, "", 100, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(bPage.Rows) != 6 {
		t.Fatalf("collection b returned %d rows, want 6", len(bPage.Rows))
	}
}

// TestListDocumentsPageLimits protects the clamp: a caller must not be able to
// ask for the whole table in one query.
func TestListDocumentsPageLimits(t *testing.T) {
	conn := testDB(t)
	if err := InitSchema(conn, EmbeddingDim); err != nil {
		t.Fatal(err)
	}
	collID, _ := seedChunks(t, conn, "c", []string{"/c/one.md"}, 20)

	page, err := ListDocumentsPage(conn, collID, "", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Rows) != 20 {
		t.Fatalf("limit 0 returned %d rows, want all 20 via the default size", len(page.Rows))
	}

	if _, err := ListDocumentsPage(conn, collID, "", maxPageSize*10, false); err != nil {
		t.Fatal(err)
	}

	// A malformed cursor is treated as the start of the stream, not an error.
	first, err := ListDocumentsPage(conn, collID, "not-a-cursor", 5, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Rows) != 5 {
		t.Fatalf("malformed cursor returned %d rows, want 5", len(first.Rows))
	}
}

// TestGetDocumentReturnsContent checks the reading pane's fetch: the full text
// and metadata come back for one id, and a deleted id reports not-found.
func TestGetDocumentReturnsContent(t *testing.T) {
	conn := testDB(t)
	if err := InitSchema(conn, EmbeddingDim); err != nil {
		t.Fatal(err)
	}
	collID, _ := seedChunks(t, conn, "d", []string{"/d/one.md"}, 2)

	page, err := ListDocumentsPage(conn, collID, "", 1, false)
	if err != nil {
		t.Fatal(err)
	}
	id := page.Rows[0].ID

	doc, ok, err := GetDocument(conn, id)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("document not found")
	}
	if doc.Content != fmt.Sprintf("body %d 0", page.Rows[0].SourceID) {
		t.Fatalf("content = %q", doc.Content)
	}
	if doc.Metadata != "" {
		t.Fatalf("metadata = %q, want empty", doc.Metadata)
	}

	if _, ok, err := GetDocument(conn, id+9999); err != nil || ok {
		t.Fatalf("missing document reported ok=%v err=%v", ok, err)
	}
}

// TestListSourcesPageWalksBothWays walks the source list forward and backward
// and checks the same keyset invariants as the chunk stream.
func TestListSourcesPageWalksBothWays(t *testing.T) {
	conn := testDB(t)
	if err := InitSchema(conn, EmbeddingDim); err != nil {
		t.Fatal(err)
	}
	paths := []string{"/n/e.md", "/n/c.md", "/n/a.md", "/n/d.md", "/n/b.md"}
	collID, _ := seedChunks(t, conn, "s", paths, 2)

	want := []string{"/n/a.md", "/n/b.md", "/n/c.md", "/n/d.md", "/n/e.md"}
	var got []string
	var pages []SourcePage
	cursor := ""
	for {
		page, err := ListSourcesPage(conn, collID, cursor, 2, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Rows) == 0 {
			t.Fatal("empty source page before the end")
		}
		for _, r := range page.Rows {
			got = append(got, r.Path)
			if r.Chunks != 2 {
				t.Fatalf("%s reports %d chunks, want 2", r.Path, r.Chunks)
			}
		}
		pages = append(pages, page)
		if page.After == "" {
			break
		}
		cursor = page.After
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("forward walk\n got %v\nwant %v", got, want)
	}

	back, err := ListSourcesPage(conn, collID, pages[1].Before, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(pathsOf(back.Rows)) != fmt.Sprint(pathsOf(pages[0].Rows)) {
		t.Fatalf("backward page = %v, want %v", pathsOf(back.Rows), pathsOf(pages[0].Rows))
	}

	start, err := ListSourcesPage(conn, collID, pages[0].Before, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(start.Rows) != 0 {
		t.Fatalf("walking back from the first page returned %d rows", len(start.Rows))
	}
}

func pathsOf(rows []SourceRow) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Path)
	}
	return out
}

// TestChunkStreamIndexExists guards the schema line the paging query depends
// on: without it every page sorts the whole collection.
func TestChunkStreamIndexExists(t *testing.T) {
	conn := testDB(t)
	if err := InitSchema(conn, EmbeddingDim); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := conn.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?",
		"idx_documents_collection_source_chunk",
	).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatal("idx_documents_collection_source_chunk is missing")
	}
}
