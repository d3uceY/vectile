package indexer

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"vectile/backend/chunker"
	"vectile/backend/config"
	"vectile/backend/db"
)

// blockingEmbedder holds any EmbedBatch call whose text contains blockOn until
// release is closed, like a long llama.cpp call that cannot be interrupted.
type blockingEmbedder struct {
	blockOn string
	started chan struct{}
	release chan struct{}
	once    sync.Once
	inner   stubEmbedder
}

func (e *blockingEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	for _, t := range texts {
		if e.blockOn != "" && strings.Contains(t, e.blockOn) {
			e.once.Do(func() { close(e.started) })
			<-e.release
			break
		}
	}
	return e.inner.EmbedBatch(texts)
}

func countOf(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.DB.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Cancel must return control at once even though the model call is still
// running: the call is abandoned, not waited for.
func TestCancelDoesNotWaitForTheEmbedder(t *testing.T) {
	openTestDB(t)

	items := makeItems(2, 3)
	cfg := &config.Config{EmbeddingBatchSize: 100000}
	emb := &blockingEmbedder{blockOn: "chunk-", started: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(func() { close(emb.release) })

	ctx, cancel := context.WithCancel(context.Background())
	result := &IndexResult{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		indexItemsBatched(ctx, db.DB, cfg, 1, "test", len(items),
			func(i int) *indexItem { return items[i] }, emb, result, nil, false)
	}()

	<-emb.started
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("run kept waiting for the in-flight embedding after cancel")
	}
	if result.Indexed != 0 {
		t.Fatalf("indexed %d items after cancel, want 0", result.Indexed)
	}
}

// A file cancelled mid-embedding leaves nothing behind: no chunks, no vectors
// and no source row (so no hash that would make the next run skip it). The next
// run indexes it, and finished files are not redone.
func TestCancelledFileLeavesNoTraceAndIsIndexedNextRun(t *testing.T) {
	openTestDB(t)

	dir := writeTree(t, map[string]string{
		"a.md": "alpha note about apples",
		"b.md": "BETA note about bananas",
	})
	cfg := &config.Config{EmbeddingBatchSize: 1, ChunkSizeTokens: 500, ChunkOverlapTokens: 50}

	emb := &blockingEmbedder{blockOn: "BETA", started: make(chan struct{}), release: make(chan struct{})}
	t.Cleanup(func() { close(emb.release) })

	ctx, cancel := context.WithCancel(context.Background())
	var result *IndexResult
	done := make(chan struct{})
	go func() {
		defer close(done)
		result = IndexProject(ctx, db.DB, cfg, "proj", []string{dir}, false, nil, emb)
	}()

	<-emb.started // b.md is mid-embedding
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("IndexProject did not return after cancel")
	}

	if result.Indexed != 1 {
		t.Fatalf("indexed %d files before cancel, want only a.md", result.Indexed)
	}
	if n := countOf(t, "SELECT COUNT(*) FROM sources WHERE source_path LIKE ?", "%b.md"); n != 0 {
		t.Fatalf("cancelled file left %d source row(s)", n)
	}
	if n := countOf(t, "SELECT COUNT(*) FROM documents WHERE content LIKE ?", "%BETA%"); n != 0 {
		t.Fatalf("cancelled file left %d chunk(s)", n)
	}
	docs := countOf(t, "SELECT COUNT(*) FROM documents")
	if vecs := countOf(t, "SELECT COUNT(*) FROM vec_documents"); vecs != docs {
		t.Fatalf("%d documents but %d vectors", docs, vecs)
	}

	next := IndexProject(context.Background(), db.DB, cfg, "proj", []string{dir}, false, nil, &stubEmbedder{})
	if next.Indexed != 1 || next.Skipped != 1 {
		t.Fatalf("next run indexed %d / skipped %d, want 1 / 1 (b.md redone, a.md kept)", next.Indexed, next.Skipped)
	}
	if n := countOf(t, "SELECT COUNT(*) FROM documents WHERE content LIKE ?", "%BETA%"); n == 0 {
		t.Fatal("b.md was not indexed by the next run")
	}
}

// countdownCtx reports "not cancelled" for the first n Err() calls and
// cancelled afterwards, so a cancel can be landed at an exact step of a write.
type countdownCtx struct {
	context.Context
	mu sync.Mutex
	n  int
}

func (c *countdownCtx) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.n > 0 {
		c.n--
		return nil
	}
	return context.Canceled
}

// A cancel that lands in the middle of writing a batch rolls the whole batch
// back, including the purge of the old chunks, so re-indexed files keep their
// previous version and nothing is half-written.
func TestCancelMidWriteRollsBackTheBatch(t *testing.T) {
	openTestDB(t)
	collID, err := db.GetOrCreateCollection(db.DB, "test", "project", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{EmbeddingBatchSize: 100000}

	item := func(path string, texts ...string) *indexItem {
		it := &indexItem{SourcePath: path, SourceType: "markdown", FileHash: "h-" + texts[0]}
		for i, text := range texts {
			it.Chunks = append(it.Chunks, chunker.Chunk{Text: text, ChunkIndex: i})
		}
		return it
	}

	oldA, oldB := item("C:\\fake\\a.md", "a-v1"), item("C:\\fake\\b.md", "b-v1", "b-v1-2")
	indexItemsBatched(context.Background(), db.DB, cfg, collID, "test", 2,
		func(i int) *indexItem { return []*indexItem{oldA, oldB}[i] }, &stubEmbedder{}, &IndexResult{}, nil, false)
	if n := countOf(t, "SELECT COUNT(*) FROM documents"); n != 3 {
		t.Fatalf("setup stored %d chunks, want 3", n)
	}

	newA, newB := item("C:\\fake\\a.md", "a-v2"), item("C:\\fake\\b.md", "b-v2", "b-v2-2")
	batch := &itemBatch{items: []*indexItem{newA, newB}, texts: []string{"a-v2", "b-v2", "b-v2-2"}}
	for range batch.texts {
		v, _ := (&stubEmbedder{}).EmbedBatch([]string{"x"})
		batch.vecs = append(batch.vecs, v[0])
	}

	// Err() is polled once before the transaction, then once per item: a.md is
	// stored, and the cancel lands before b.md.
	ctx := &countdownCtx{Context: context.Background(), n: 2}
	result := &IndexResult{}
	if names := writeItemBatch(ctx, db.DB, collID, batch, result, false); names != nil {
		t.Fatalf("cancelled write reported stored items: %v", names)
	}
	if result.Indexed != 0 {
		t.Fatalf("cancelled write counted %d indexed", result.Indexed)
	}

	rows, err := db.DB.Query("SELECT content FROM documents ORDER BY content")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		got = append(got, c)
	}
	if strings.Join(got, ",") != "a-v1,b-v1,b-v1-2" {
		t.Fatalf("chunks after the cancelled write = %v, want the untouched v1 set", got)
	}
	if vecs := countOf(t, "SELECT COUNT(*) FROM vec_documents"); vecs != 3 {
		t.Fatalf("%d vectors after rollback, want 3", vecs)
	}
}

// wrongDimEmbedder returns a vector the vec0 table rejects for chunks marked bad.
type wrongDimEmbedder struct{ inner stubEmbedder }

func (e *wrongDimEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	out, err := e.inner.EmbedBatch(texts)
	for i, t := range texts {
		if strings.Contains(t, "BAD") {
			out[i] = []float32{1, 2, 3}
		}
	}
	return out, err
}

// An item that fails while being stored must not leave some of its chunks
// behind, and must not stop the rest of the batch.
func TestFailedStoreLeavesNoPartialItem(t *testing.T) {
	openTestDB(t)
	collID, err := db.GetOrCreateCollection(db.DB, "test", "project", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{EmbeddingBatchSize: 100000}

	mk := func(path string, texts ...string) *indexItem {
		it := &indexItem{SourcePath: path, SourceType: "markdown", FileHash: "h"}
		for i, text := range texts {
			it.Chunks = append(it.Chunks, chunker.Chunk{Text: text, ChunkIndex: i})
		}
		return it
	}
	items := []*indexItem{
		mk("C:\\fake\\bad.md", "fine first chunk", "BAD second chunk"),
		mk("C:\\fake\\good.md", "good chunk"),
	}
	result := &IndexResult{}
	indexItemsBatched(context.Background(), db.DB, cfg, collID, "test", len(items),
		func(i int) *indexItem { return items[i] }, &wrongDimEmbedder{}, result, nil, false)

	if result.Indexed != 1 || result.Errors != 1 {
		t.Fatalf("indexed %d, errors %d, want 1 and 1 (%v)", result.Indexed, result.Errors, result.ErrorMessages)
	}
	if n := countOf(t, "SELECT COUNT(*) FROM documents WHERE content LIKE ?", "%chunk%"); n != 1 {
		t.Fatalf("%d chunks stored, want only good.md's single chunk", n)
	}
	if n := countOf(t, "SELECT COUNT(*) FROM sources WHERE source_path LIKE ?", "%bad.md"); n != 0 {
		t.Fatalf("failed item left %d source row(s)", n)
	}
}
