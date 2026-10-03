package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"vectile/backend/config"
	"vectile/backend/db"
	"vectile/backend/embeddings"
	"vectile/backend/services"
)

// testCore opens a temp database, swaps it in as db.DB, and seeds a small
// library: one collection with two sources, plus a second collection that
// reuses one of the paths so ambiguity can be exercised.
func testCore(t *testing.T) *services.Core {
	t.Helper()

	conn, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(4)
	if err := db.InitSchema(conn, db.EmbeddingDim); err != nil {
		t.Fatal(err)
	}

	prev := db.DB
	db.DB = conn
	t.Cleanup(func() {
		db.DB = prev
		_ = conn.Close()
	})

	seedLibrary(t, conn)

	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	return &services.Core{Cfg: cfg}
}

func seedLibrary(t *testing.T, conn *sql.DB) {
	t.Helper()

	notes, err := db.GetOrCreateCollection(conn, "notes", "project", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	books, err := db.GetOrCreateCollection(conn, "books", "project", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	a := insertSource(t, conn, notes, "markdown", "/vault/a.md", "2026-01-02 03:00:00")
	insertDocument(t, conn, a, notes, 0, "a.md", "intro alpha content", `{"heading_path":"Intro","tags":["work"]}`)
	insertDocument(t, conn, a, notes, 1, "a.md", "second beta paragraph", `{"heading_path":"Body"}`)

	b := insertSource(t, conn, notes, "markdown", "/vault/b.md", "2026-06-01 09:00:00")
	insertDocument(t, conn, b, notes, 0, "b.md", "dated note gamma", `{"date":"2026-05-01"}`)

	// Same path in a second collection: read_source must report both.
	dup := insertSource(t, conn, books, "markdown", "/vault/a.md", "2026-02-01 00:00:00")
	insertDocument(t, conn, dup, books, 0, "a.md", "book copy of a", `{}`)
}

func insertSource(t *testing.T, conn *sql.DB, collectionID int64, sourceType, path, lastIndexed string) int64 {
	t.Helper()
	res, err := conn.Exec(
		`INSERT INTO sources (collection_id, source_type, source_path, last_indexed_at)
		 VALUES (?, ?, ?, ?)`, collectionID, sourceType, path, lastIndexed)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func insertDocument(t *testing.T, conn *sql.DB, sourceID, collectionID int64, chunkIndex int, title, content, metadata string) int64 {
	t.Helper()
	res, err := conn.Exec(
		`INSERT INTO documents (source_id, collection_id, chunk_index, title, content, metadata)
		 VALUES (?, ?, ?, ?, ?, ?)`, sourceID, collectionID, chunkIndex, title, content, metadata)
	if err != nil {
		t.Fatal(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// call runs one handler and decodes its JSON text result.
func call(t *testing.T, h server.ToolHandlerFunc, args map[string]any) map[string]any {
	t.Helper()
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Name: "test", Arguments: args}}
	res, err := h(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %s", resultText(t, res))
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	return out
}

func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatal("empty tool result")
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("unexpected content type %T", res.Content[0])
	}
	return tc.Text
}

func firstChunkID(t *testing.T, conn *sql.DB, content string) int64 {
	t.Helper()
	var id int64
	if err := conn.QueryRow("SELECT id FROM documents WHERE content = ?", content).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestGetChunkReadsNeighbours(t *testing.T) {
	core := testCore(t)
	seed := firstChunkID(t, db.DB, "second beta paragraph")

	out := call(t, handleGetChunk(core), map[string]any{
		"id": float64(seed), "before": 1, "after": 1,
	})
	chunks, ok := out["chunks"].([]any)
	if !ok || len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %v", out["chunks"])
	}
	first := chunks[0].(map[string]any)
	if first["content"] != "intro alpha content" {
		t.Fatalf("neighbour not included: %v", first["content"])
	}
}

func TestReadSourceReassemblesAndDisambiguates(t *testing.T) {
	core := testCore(t)

	// Two collections hold /vault/a.md, so the bare path is ambiguous.
	out := call(t, handleReadSource(core), map[string]any{"source_path": "/vault/a.md"})
	if out["candidates"] == nil {
		t.Fatalf("expected candidates, got %v", out)
	}

	out = call(t, handleReadSource(core), map[string]any{
		"source_path": "/vault/a.md", "collection": "books",
	})
	if out["content"] != "book copy of a" {
		t.Fatalf("wrong source read: %v", out["content"])
	}

	// A heading filter narrows a multi-chunk source.
	out = call(t, handleReadSource(core), map[string]any{
		"source_path": "/vault/a.md", "collection": "notes", "heading": "Intro",
	})
	if out["content"] != "intro alpha content" {
		t.Fatalf("expected the Intro chunk only, got %v", out["content"])
	}
}

func TestGrepFindsLiteralAndRegex(t *testing.T) {
	core := testCore(t)

	out := call(t, handleGrep(core), map[string]any{"pattern": "beta"})
	matches := out["matches"].([]any)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if got := matches[0].(map[string]any)["context"].(string); got != "second beta paragraph" {
		t.Fatalf("context = %q", got)
	}

	out = call(t, handleGrep(core), map[string]any{"pattern": "ALPHA", "regex": true})
	if len(out["matches"].([]any)) != 0 {
		t.Fatalf("regex should be case-sensitive, got %v", out["matches"])
	}

	out = call(t, handleGrep(core), map[string]any{"pattern": `beta\s+paragraph`, "regex": true})
	if len(out["matches"].([]any)) != 1 {
		t.Fatalf("regex match failed: %v", out["matches"])
	}

	out = call(t, handleGrep(core), map[string]any{"pattern": "alpha", "source_type": "code"})
	if len(out["matches"].([]any)) != 0 {
		t.Fatal("source_type filter did not apply")
	}
}

func TestListSourcesAndFacets(t *testing.T) {
	core := testCore(t)

	out := call(t, handleListSources(core), map[string]any{"collection": "notes"})
	if n := len(out["sources"].([]any)); n != 2 {
		t.Fatalf("expected 2 sources, got %d", n)
	}
	out = call(t, handleListSources(core), map[string]any{"collection": "notes", "match": "b.md"})
	if n := len(out["sources"].([]any)); n != 1 {
		t.Fatalf("match filter should leave 1 source, got %d", n)
	}

	facets := call(t, handleFacets(core), map[string]any{})
	if facets["keys"] == nil {
		t.Fatalf("expected metadata keys, got %v", facets)
	}
	values, ok := facets["values"].(map[string]any)
	if !ok || values["tags"] == nil {
		t.Fatalf("expected top tags, got %v", facets["values"])
	}
}

func TestFindRelatedExcludesSeed(t *testing.T) {
	core := testCore(t)

	vec := make([]float32, db.EmbeddingDim)
	for i := range vec {
		vec[i] = 0.5
	}
	blob := embeddings.SerializeFloat32(vec)
	a := firstChunkID(t, db.DB, "intro alpha content")
	b := firstChunkID(t, db.DB, "second beta paragraph")
	if err := db.InsertEmbedding(db.DB, a, blob); err != nil {
		t.Fatal(err)
	}
	if err := db.InsertEmbedding(db.DB, b, blob); err != nil {
		t.Fatal(err)
	}

	out := call(t, handleFindRelated(core), map[string]any{"chunk_id": float64(a), "top_k": 5})
	results := out["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("expected only the other chunk, got %d", len(results))
	}
	if got := results[0].(map[string]any)["id"].(float64); int64(got) != b {
		t.Fatalf("seed was not excluded: %v", results)
	}
}

func TestTimelineOrdersByDate(t *testing.T) {
	core := testCore(t)

	out := call(t, handleTimeline(core), map[string]any{"limit": 10})
	docs := out["documents"].([]any)
	if len(docs) != 4 {
		t.Fatalf("expected 4 documents, got %d", len(docs))
	}
	// b.md carries a 2026-05-01 date, ahead of the fallback index times.
	if got := docs[0].(map[string]any)["source_path"]; got != "/vault/b.md" {
		t.Fatalf("newest first expected b.md, got %v", got)
	}

	out = call(t, handleTimeline(core), map[string]any{"date_from": "2026-05-01", "date_to": "2026-05-31"})
	if n := len(out["documents"].([]any)); n != 1 {
		t.Fatalf("date range should leave 1 doc, got %d", n)
	}
}
