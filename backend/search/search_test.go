package search

import (
	"database/sql"
	"path/filepath"
	"testing"

	"vectile/backend/config"
	"vectile/backend/db"
	"vectile/backend/embeddings"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// fakeEmbedder returns a fixed vector so search runs without a real model.
type fakeEmbedder struct{ vec []float32 }

func (f fakeEmbedder) Embed(string) ([]float32, error) { return f.vec, nil }

func seedDoc(t *testing.T, conn *sql.DB, collID, sourceID int64, chunkIndex int, title, content string, vec []float32) {
	t.Helper()
	res, err := conn.Exec(
		"INSERT INTO documents (source_id, collection_id, chunk_index, title, content) VALUES (?,?,?,?,?)",
		sourceID, collID, chunkIndex, title, content,
	)
	if err != nil {
		t.Fatal(err)
	}
	docID, _ := res.LastInsertId()
	if err := db.InsertEmbedding(conn, docID, embeddings.SerializeFloat32(vec)); err != nil {
		t.Fatal(err)
	}
}

// TestHybridSearchRanksFTSAndVector verifies the full pipeline: a doc that
// matches by exact terms (FTS) AND by vector similarity ranks above one that
// matches only one signal.
func TestHybridSearchRanksFTSAndVector(t *testing.T) {
	conn := newTestDB(t)
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

	// query vector with a spike at index 0.
	qv := make([]float32, db.EmbeddingDim)
	qv[0] = 1

	// d1: matches the query term AND has the same vector (distance 0).
	seedDoc(t, conn, collID, sourceID, 0, "Fox doc", "the quick brown fox jumps", qv)
	// d2: unrelated text + a different vector.
	d2v := make([]float32, db.EmbeddingDim)
	d2v[1] = 1
	seedDoc(t, conn, collID, sourceID, 1, "Other doc", "completely unrelated subject matter", d2v)

	sd := config.SearchDefaults{TopK: 10, RRFK: 60, VectorWeight: 0.7, FTSWeight: 0.3}
	resp, err := Search(conn, "quick fox", Filters{TopK: 10}, fakeEmbedder{qv}, sd)
	if err != nil {
		t.Fatal(err)
	}
	results := resp.Results
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].Title != "Fox doc" {
		t.Fatalf("expected 'Fox doc' first, got %q (total %d)", results[0].Title, len(results))
	}
}

// TestSearchFallsBackToFTSWithoutModel verifies Search still returns exact-term
// results when the embedder errors (e.g. no model).
func TestSearchFallsBackToFTSWithoutModel(t *testing.T) {
	conn := newTestDB(t)
	if err := db.InitSchema(conn, db.EmbeddingDim); err != nil {
		t.Fatal(err)
	}
	collID, _ := db.GetOrCreateCollection(conn, "test", "project", nil, nil)
	srcRes, _ := conn.Exec(
		"INSERT INTO sources (collection_id, source_type, source_path) VALUES (?,?,?)",
		collID, "markdown", "/tmp/note.md",
	)
	sourceID, _ := srcRes.LastInsertId()
	seedDoc(t, conn, collID, sourceID, 0, "K8s", "kubernetes deployment strategy for rollout", make([]float32, db.EmbeddingDim))

	sd := config.SearchDefaults{TopK: 10, RRFK: 60, VectorWeight: 0.7, FTSWeight: 0.3}
	resp, err := Search(conn, "kubernetes rollout", Filters{}, failingEmbedder{}, sd)
	if err != nil {
		t.Fatal(err)
	}
	results := resp.Results
	if len(results) == 0 || results[0].Title != "K8s" {
		t.Fatalf("expected FTS fallback to find K8s, got %+v", results)
	}
}

type failingEmbedder struct{}

func (failingEmbedder) Embed(string) ([]float32, error) {
	return nil, sql.ErrNoRows
}
