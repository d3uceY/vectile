package indexer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vectile/backend/config"
	"vectile/backend/db"
)

// An edit that lands in the same wall-clock second as the last index must
// still be picked up. With mtimes stored at second precision, the precheck
// saw an unchanged mtime and skipped the file before hashing it.
func TestSubsecondEditIsReindexed(t *testing.T) {
	dir := writeTree(t, map[string]string{"note.md": "# first\n\nold text\n"})
	note := filepath.Join(dir, "note.md")
	base := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(note, base, base.Add(100*time.Millisecond)); err != nil {
		t.Fatal(err)
	}

	if err := db.Open(filepath.Join(t.TempDir(), "mtime.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	cfg := &config.Config{EmbeddingBatchSize: 32, ChunkSizeTokens: 500, ChunkOverlapTokens: 50}
	index := func() *IndexResult {
		return IndexProject(context.Background(), db.DB, cfg, "test", []string{dir}, false, nil, &stubEmbedder{})
	}

	if r := index(); r.Indexed != 1 {
		t.Fatalf("first run indexed %d files, want 1 (errors: %v)", r.Indexed, r.ErrorMessages)
	}

	if err := os.WriteFile(note, []byte("# first\n\nnew text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(note, base, base.Add(900*time.Millisecond)); err != nil {
		t.Fatal(err)
	}

	if r := index(); r.Indexed != 1 {
		t.Fatalf("second run indexed %d files, want 1: a same-second edit was skipped", r.Indexed)
	}
}
