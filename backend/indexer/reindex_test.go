package indexer

import (
	"context"
	"path/filepath"
	"testing"

	"vectile/backend/chunker"
	"vectile/backend/config"
	"vectile/backend/db"
)

// TestReindexKeepsOldVersionWhenChunkFailsToEmbed: a file that can't be fully
// re-indexed keeps its old chunks, so it stays searchable until a replacement lands.
func TestReindexKeepsOldVersionWhenChunkFailsToEmbed(t *testing.T) {
	if err := db.Open(filepath.Join(t.TempDir(), "reindex.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	collID, err := db.GetOrCreateCollection(db.DB, "test", "project", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{EmbeddingBatchSize: 100000}

	item := func(texts ...string) *indexItem {
		it := &indexItem{SourcePath: "C:\\fake\\note.md", SourceType: "markdown"}
		for i, text := range texts {
			it.Chunks = append(it.Chunks, chunker.Chunk{Text: text, ChunkIndex: i})
		}
		return it
	}

	// First pass: the file indexes cleanly, so it is searchable.
	first := item("v1 alpha", "v1 beta")
	result := &IndexResult{}
	indexItemsBatched(context.Background(), db.DB, cfg, collID, "test", 1,
		func(int) *indexItem { return first }, &stubEmbedder{}, result, nil, false)
	if result.Indexed != 1 {
		t.Fatalf("first pass indexed %d items, want 1 (%v)", result.Indexed, result.ErrorMessages)
	}

	// Second pass: the file changed and one of its new chunks cannot be
	// embedded, so the item is dropped instead of being half-stored.
	second := item("v2 alpha", "v2 bad")
	emb := &stubEmbedder{fail: func(text string) bool { return text == "v2 bad" }}
	result = &IndexResult{}
	indexItemsBatched(context.Background(), db.DB, cfg, collID, "test", 1,
		func(int) *indexItem { return second }, emb, result, nil, false)
	if result.Indexed != 0 || result.Errors != 1 {
		t.Fatalf("second pass indexed %d, errors %d, want 0 and 1", result.Indexed, result.Errors)
	}

	var docs, vecs int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM documents").Scan(&docs); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM vec_documents").Scan(&vecs); err != nil {
		t.Fatal(err)
	}
	if docs != 2 || vecs != 2 {
		t.Fatalf("after the failed reindex: %d documents and %d vectors, want the 2 old chunks kept",
			docs, vecs)
	}
	var content string
	if err := db.DB.QueryRow("SELECT content FROM documents WHERE chunk_index = 0").Scan(&content); err != nil {
		t.Fatal(err)
	}
	if content != "v1 alpha" {
		t.Fatalf("chunk 0 = %q, want the old version", content)
	}
}
