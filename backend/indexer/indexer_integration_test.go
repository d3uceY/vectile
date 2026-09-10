package indexer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"vectile/backend/config"
	"vectile/backend/db"
	"vectile/backend/embeddings"
	"vectile/backend/search"
)

func integrationModelPath() string {
	if p := os.Getenv("VECTILE_EMBED_MODEL"); p != "" {
		return p
	}
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(cfgDir, "vectile", "models", "bge-m3-Q4_K_M.gguf")
}

// TestIndexAndSearchEndToEnd exercises the full pipeline without the GUI:
// write files -> IndexProject (chunk + embed via real llama.go) -> hybrid
// search -> prune. Skips when the model file isn't present.
func TestIndexAndSearchEndToEnd(t *testing.T) {
	modelPath := integrationModelPath()
	if _, err := os.Stat(modelPath); err != nil {
		t.Skipf("model not present: %v", err)
	}

	// A small doc folder.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "notes", "kubernetes.md"),
		"# Deployment strategy\n\nWe run three small clusters instead of one big one.\nThe argument is blast radius.\n")
	writeFile(t, filepath.Join(dir, "notes", "camera.md"),
		"# Field notes\n\nAperture, shutter speed, and ISO are the three dials.\n")
	writeFile(t, filepath.Join(dir, "spec.txt"),
		"Specification: the exporter emits one metric per request.\n")

	if err := db.Open(filepath.Join(t.TempDir(), "e2e.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	conn := db.DB

	cfg := &config.Config{
		EmbeddingBatchSize:    32,
		ChunkSizeTokens:       500,
		ChunkOverlapTokens:    50,
		SkipCloudPlaceholders: true,
		Repositories:          map[string][]string{},
		Projects:              map[string][]string{},
	}
	embedder := embeddings.NewEmbedder(modelPath, 0, 0)
	t.Cleanup(embedder.Close)

	result := IndexProject(context.Background(), conn, cfg, "test", []string{dir}, false, nil, embedder)
	if result.Errors > 0 {
		t.Fatalf("index errors: %v", result.ErrorMessages)
	}
	if result.Indexed == 0 {
		t.Fatal("expected at least one indexed file")
	}

	// Hybrid search should find the markdown by exact term.
	sd := config.SearchDefaults{TopK: 10, RRFK: 60, VectorWeight: 0.7, FTSWeight: 0.3}
	res, err := search.Search(conn, "kubernetes deployment", search.Filters{}, embedder, sd)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Results) == 0 || res.Results[0].Collection != "test" {
		t.Fatalf("expected results from 'test', got %+v", res.Results)
	}

	// Deleting a file then pruning drops its rows.
	if err := os.Remove(filepath.Join(dir, "spec.txt")); err != nil {
		t.Fatal(err)
	}
	pr := PruneCollection(conn, cfg, "test")
	if pr.Pruned == 0 {
		t.Fatal("expected prune to remove the deleted file")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
