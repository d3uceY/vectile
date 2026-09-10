package search

import (
	"database/sql"
	"testing"

	"vectile/backend/config"
	"vectile/backend/db"
	"vectile/backend/embeddings"
)

// countingEmbedder is a cache-capable test double: it stands for a named model,
// consults the cache the way the real embedder does, and counts how often it
// actually had to embed.
type countingEmbedder struct {
	model string
	vec   []float32
	calls int
}

func (c *countingEmbedder) Embed(string) ([]float32, error) {
	c.calls++
	return c.vec, nil
}

func (c *countingEmbedder) QueryEmbed(text string, cache embeddings.QueryCache) ([]float32, bool, error) {
	if vec, ok := cache.Get(c.model, text); ok {
		return vec, true, nil
	}
	c.calls++
	cache.Put(c.model, text, c.vec)
	return c.vec, false, nil
}

var cacheTestDefaults = config.SearchDefaults{TopK: 10, RRFK: 60, VectorWeight: 0.7, FTSWeight: 0.3}

// cacheTestCorpus seeds one collection holding one document, and returns the
// query vector that document was embedded with.
func cacheTestCorpus(t *testing.T, conn *sql.DB) []float32 {
	t.Helper()
	if err := db.InitSchema(conn, db.EmbeddingDim); err != nil {
		t.Fatal(err)
	}
	collID, err := db.GetOrCreateCollection(conn, "test", "project", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	srcRes, err := conn.Exec(
		"INSERT INTO sources (collection_id, source_type, source_path) VALUES (?,?,?)",
		collID, "markdown", "/tmp/note.md",
	)
	if err != nil {
		t.Fatal(err)
	}
	sourceID, _ := srcRes.LastInsertId()

	qv := make([]float32, db.EmbeddingDim)
	qv[0] = 1
	seedDoc(t, conn, collID, sourceID, 0, "Fox doc", "the quick brown fox jumps", qv)
	return qv
}

func cacheRows(t *testing.T, conn *sql.DB) int {
	t.Helper()
	var n int
	if err := conn.QueryRow("SELECT COUNT(*) FROM query_cache").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// TestSearchReusesCachedQueryVector is the whole point of the feature: the
// second search of the same text must not touch the embedder, and must return
// the same ranking.
func TestSearchReusesCachedQueryVector(t *testing.T) {
	conn := newTestDB(t)
	qv := cacheTestCorpus(t, conn)
	emb := &countingEmbedder{model: "/m/bge.gguf", vec: qv}

	first, err := Search(conn, "quick fox", Filters{TopK: 10}, emb, cacheTestDefaults)
	if err != nil {
		t.Fatal(err)
	}
	if first.Cached {
		t.Fatal("the first search of a query can't come from the cache")
	}
	if emb.calls != 1 {
		t.Fatalf("embed calls = %d, want 1", emb.calls)
	}
	if len(first.Results) == 0 {
		t.Fatal("expected at least one result")
	}

	second, err := Search(conn, "quick fox", Filters{TopK: 10}, emb, cacheTestDefaults)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Cached {
		t.Fatal("expected the second search to report a cache hit")
	}
	if emb.calls != 1 {
		t.Fatalf("cached search re-embedded the query: calls = %d", emb.calls)
	}
	if len(second.Results) != len(first.Results) || second.Results[0].Title != first.Results[0].Title {
		t.Fatalf("cached ranking differs: %+v vs %+v", first.Results, second.Results)
	}
}

// TestSearchCacheIsPerModel guards the silent-wrongness case: a vector embedded
// by one model is never reused for another.
func TestSearchCacheIsPerModel(t *testing.T) {
	conn := newTestDB(t)
	qv := cacheTestCorpus(t, conn)

	a := &countingEmbedder{model: "/m/a.gguf", vec: qv}
	if _, err := Search(conn, "quick fox", Filters{}, a, cacheTestDefaults); err != nil {
		t.Fatal(err)
	}
	if cacheRows(t, conn) != 1 {
		t.Fatalf("expected 1 cached query, got %d", cacheRows(t, conn))
	}

	b := &countingEmbedder{model: "/m/b.gguf", vec: qv}
	resp, err := Search(conn, "quick fox", Filters{}, b, cacheTestDefaults)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Cached {
		t.Fatal("another model must not reuse that entry")
	}
	if b.calls != 1 {
		t.Fatalf("second model embed calls = %d, want 1", b.calls)
	}
	if cacheRows(t, conn) != 2 {
		t.Fatalf("expected one entry per model, got %d", cacheRows(t, conn))
	}
}

// TestSearchCacheIgnoresFilters: the cached vector is a function of the text
// and the model only, so a different filter set is still a cache hit.
func TestSearchCacheIgnoresFilters(t *testing.T) {
	conn := newTestDB(t)
	qv := cacheTestCorpus(t, conn)
	emb := &countingEmbedder{model: "/m/bge.gguf", vec: qv}

	if _, err := Search(conn, "quick fox", Filters{TopK: 10}, emb, cacheTestDefaults); err != nil {
		t.Fatal(err)
	}
	resp, err := Search(conn, "quick fox", Filters{Collection: "test", TopK: 24}, emb, cacheTestDefaults)
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Cached {
		t.Fatal("filters don't change the query vector, so this should hit the cache")
	}
	if emb.calls != 1 {
		t.Fatalf("embed calls = %d, want 1", emb.calls)
	}
}

func TestSearchDoesNotCacheBlankQuery(t *testing.T) {
	conn := newTestDB(t)
	qv := cacheTestCorpus(t, conn)
	emb := &countingEmbedder{model: "/m/bge.gguf", vec: qv}

	if _, err := Search(conn, "   ", Filters{}, emb, cacheTestDefaults); err != nil {
		t.Fatal(err)
	}
	if n := cacheRows(t, conn); n != 0 {
		t.Fatalf("a blank query should not be cached, got %d rows", n)
	}
}

// TestSearchWithoutCacheCapableEmbedder keeps the plain Embed-only contract
// working (existing fakes, and any embedder that can't name its model).
func TestSearchWithoutCacheCapableEmbedder(t *testing.T) {
	conn := newTestDB(t)
	qv := cacheTestCorpus(t, conn)

	if _, err := Search(conn, "quick fox", Filters{}, fakeEmbedder{qv}, cacheTestDefaults); err != nil {
		t.Fatal(err)
	}
	if n := cacheRows(t, conn); n != 0 {
		t.Fatalf("an embedder with no model identity must not write cache rows, got %d", n)
	}
}
