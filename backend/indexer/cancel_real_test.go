package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"vectile/backend/config"
	"vectile/backend/db"
	"vectile/backend/embeddings"
)

// cancelOnCall cancels the run when the nth EmbedBatch call starts, then lets
// that call run on, so the cancel lands while llama.cpp is mid-inference.
type cancelOnCall struct {
	inner    Embedder
	n        int32
	calls    atomic.Int32
	cancel   context.CancelFunc
	canceled atomic.Int64 // unix nanos when cancel was issued
	callTime atomic.Int64 // how long the in-flight call took, nanos
}

func (e *cancelOnCall) EmbedBatch(texts []string) ([][]float32, error) {
	if e.calls.Add(1) == e.n {
		e.canceled.Store(time.Now().UnixNano())
		e.cancel()
		start := time.Now()
		defer func() { e.callTime.Store(int64(time.Since(start))) }()
	}
	return e.inner.EmbedBatch(texts)
}

// With the real model: cancelling in the middle of a big file returns long
// before the in-flight inference ends, stores nothing from that file, and the
// next run indexes it whole. Skips when the model file is not present.
func TestCancelMidFileWithRealModel(t *testing.T) {
	modelPath := integrationModelPath()
	if _, err := os.Stat(modelPath); err != nil {
		t.Skipf("model not present: %v", err)
	}

	var big strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&big, "## Section %d\n\n", i)
		for j := 0; j < 12; j++ {
			fmt.Fprintf(&big, "Paragraph %d of section %d discusses topic %d in some detail, covering the usual background, the tradeoffs involved, and what to watch for when running it in production. ", j, i, i*j)
		}
		big.WriteString("\n\n")
	}
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a-small.md"), "# Small\n\nA short note about apples.\n")
	writeFile(t, filepath.Join(dir, "b-big.md"), big.String())

	openTestDB(t)
	cfg := &config.Config{
		EmbeddingBatchSize: 1, ChunkSizeTokens: 200, ChunkOverlapTokens: 20,
		Repositories: map[string][]string{}, Projects: map[string][]string{},
	}
	real := embeddings.NewEmbedder(modelPath, 0, 0)
	t.Cleanup(real.Close)

	// Warm the model so load time does not count as embedding time.
	if _, err := real.EmbedBatch([]string{"warm up"}); err != nil {
		t.Fatalf("embedder: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	emb := &cancelOnCall{inner: real, n: 3, cancel: cancel} // call 1 = small file, 2 = first big slice, 3 = second
	res := IndexProject(ctx, db.DB, cfg, "proj", []string{dir}, false, nil, emb)
	returned := time.Now().UnixNano()

	if emb.canceled.Load() == 0 {
		t.Fatalf("run finished before the cancel point; chunks too few (indexed=%d)", res.Indexed)
	}
	lag := time.Duration(returned - emb.canceled.Load())
	t.Logf("cancel -> IndexProject returned in %v", lag)
	if lag > 500*time.Millisecond {
		t.Fatalf("cancel took %v to take effect", lag)
	}

	if n := countOf(t, "SELECT COUNT(*) FROM sources WHERE source_path LIKE ?", "%b-big.md"); n != 0 {
		t.Fatalf("cancelled big file left %d source row(s)", n)
	}
	if n := countOf(t, "SELECT COUNT(*) FROM documents WHERE content LIKE ?", "%Paragraph%"); n != 0 {
		t.Fatalf("cancelled big file left %d chunks", n)
	}
	if n := countOf(t, "SELECT COUNT(*) FROM sources WHERE source_path LIKE ?", "%a-small.md"); n != 1 {
		t.Fatalf("finished small file has %d source rows, want 1", n)
	}

	start := time.Now()
	next := IndexProject(context.Background(), db.DB, cfg, "proj", []string{dir}, false, nil, real)
	t.Logf("follow-up run took %v (includes waiting for the abandoned call)", time.Since(start))
	if next.Indexed != 1 || next.Skipped != 1 || next.Errors != 0 {
		t.Fatalf("follow-up indexed %d, skipped %d, errors %d (%v), want 1/1/0", next.Indexed, next.Skipped, next.Errors, next.ErrorMessages)
	}
	docs := countOf(t, "SELECT COUNT(*) FROM documents WHERE content LIKE ?", "%Paragraph%")
	vecs := countOf(t, "SELECT COUNT(*) FROM vec_documents")
	total := countOf(t, "SELECT COUNT(*) FROM documents")
	if docs < 24 || vecs != total {
		t.Fatalf("after the follow-up: %d big-file chunks, %d vectors for %d documents", docs, vecs, total)
	}
}
