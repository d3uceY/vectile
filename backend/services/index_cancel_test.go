package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"vectile/backend/config"
	"vectile/backend/db"
	"vectile/backend/embeddings"
)

// eventLog records the Wails events of one run, in order.
type eventLog struct {
	mu     sync.Mutex
	events []string
}

func (l *eventLog) add(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, name)
}

func (l *eventLog) count(name string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, e := range l.events {
		if e == name {
			n++
		}
	}
	return n
}

func waitFor(t *testing.T, what string, within time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// cancelHarness is a real Core (real DB, real bge-m3, real Wails event bus)
// with two project collections on disk. Skips when the model is absent.
func cancelHarness(t *testing.T) (*IndexService, *eventLog, string) {
	t.Helper()
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		t.Skip("no user config dir")
	}
	modelPath := filepath.Join(cfgDir, "vectile", "models", "bge-m3-Q4_K_M.gguf")
	if p := os.Getenv("VECTILE_EMBED_MODEL"); p != "" {
		modelPath = p
	}
	if _, err := os.Stat(modelPath); err != nil {
		t.Skipf("model not present: %v", err)
	}

	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var big strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&big, "## Section %d\n\n", i)
		for j := 0; j < 12; j++ {
			fmt.Fprintf(&big, "Paragraph %d of section %d covers topic %d in some detail, with the usual background and tradeoffs. ", j, i, i*j)
		}
		big.WriteString("\n\n")
	}
	write("one/a-small.md", "# Small\n\nA short note about apples.\n")
	write("one/b-big.md", big.String())
	write("two/c-other.md", "# Other\n\nA note in the second collection.\n")

	if err := os.MkdirAll(filepath.Join(root, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := db.Open(filepath.Join(root, "db", "t.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	cfg := &config.Config{
		EmbeddingBatchSize: 1, ChunkSizeTokens: 200, ChunkOverlapTokens: 20,
		Repositories: map[string][]string{},
		Projects: map[string][]string{
			"one": {filepath.Join(root, "one")},
			"two": {filepath.Join(root, "two")},
		},
	}
	core := NewCore(cfg)
	core.CfgPath = filepath.Join(root, "config.json")
	core.Embedder = embeddings.NewEmbedder(modelPath, 0, 0)
	t.Cleanup(core.Embedder.Close)
	if _, err := core.Embedder.EmbedBatch([]string{"warm up"}); err != nil {
		t.Fatalf("embedder: %v", err)
	}

	app := application.New(application.Options{Name: "vectile-test"})
	core.App = app
	log := &eventLog{}
	for _, name := range []string{"indexing:file", "indexing:complete", "indexing:all-done", "indexing:cancelled", "indexing:failed"} {
		name := name
		app.Event.On(name, func(*application.CustomEvent) { log.add(name) })
	}
	return &IndexService{core: core}, log, root
}

func countRows(t *testing.T, q string, args ...any) int {
	t.Helper()
	var n int
	if err := db.DB.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Cancelling Index all while a big file is mid-embedding stops the run at
// once, announces it exactly once, frees the index lock, keeps finished files,
// leaves nothing of the interrupted file, and a second run completes the rest.
func TestIndexAllCancelMidFile(t *testing.T) {
	s, log, _ := cancelHarness(t)

	started, err := s.IndexAll(false)
	if err != nil || !started {
		t.Fatalf("IndexAll = (%v, %v)", started, err)
	}
	// indexing:file fires per stored file; the first one is a-small.md, so the
	// run is now inside b-big.md.
	waitFor(t, "first file to be indexed", 60*time.Second, func() bool { return log.count("indexing:file") >= 1 })

	t0 := time.Now()
	if !s.CancelIndexing() {
		t.Fatal("CancelIndexing reported no run in progress")
	}
	waitFor(t, "run to stop", 5*time.Second, func() bool { return !s.IsIndexing() })
	t.Logf("cancel -> index lock released in %v", time.Since(t0))
	waitFor(t, "cancelled event", 2*time.Second, func() bool { return log.count("indexing:cancelled") >= 1 })

	time.Sleep(200 * time.Millisecond) // any duplicate event would land by now
	if n := log.count("indexing:cancelled"); n != 1 {
		t.Fatalf("%d cancelled events, want exactly 1", n)
	}
	if n := log.count("indexing:all-done"); n != 0 {
		t.Fatalf("cancelled run still emitted all-done (%d)", n)
	}
	if n := countRows(t, "SELECT COUNT(*) FROM documents WHERE content LIKE ?", "%Paragraph%"); n != 0 {
		t.Fatalf("interrupted file left %d chunks", n)
	}
	if n := countRows(t, "SELECT COUNT(*) FROM sources WHERE source_path LIKE ?", "%b-big.md"); n != 0 {
		t.Fatalf("interrupted file left %d source rows", n)
	}
	if n := countRows(t, "SELECT COUNT(*) FROM sources WHERE source_path LIKE ?", "%a-small.md"); n != 1 {
		t.Fatalf("finished file has %d source rows, want 1", n)
	}
	if n := countRows(t, "SELECT COUNT(*) FROM sources WHERE source_path LIKE ?", "%c-other.md"); n != 0 {
		t.Fatalf("second collection ran after the cancel (%d source rows)", n)
	}

	// The same service can start again right away and finishes the job.
	started, err = s.IndexAll(false)
	if err != nil || !started {
		t.Fatalf("second IndexAll = (%v, %v)", started, err)
	}
	waitFor(t, "second run to finish", 3*time.Minute, func() bool { return log.count("indexing:all-done") >= 1 })
	if n := countRows(t, "SELECT COUNT(*) FROM sources"); n != 3 {
		t.Fatalf("%d sources after the follow-up run, want 3", n)
	}
	docs := countRows(t, "SELECT COUNT(*) FROM documents")
	if vecs := countRows(t, "SELECT COUNT(*) FROM vec_documents"); vecs != docs {
		t.Fatalf("%d documents but %d vectors", docs, vecs)
	}
}

// A Cancel that arrives before any collection has started still ends the run
// and tells the frontend, which would otherwise sit on "indexing" forever.
func TestIndexAllCancelImmediatelyIsAnnounced(t *testing.T) {
	s, log, _ := cancelHarness(t)

	started, err := s.IndexAll(false)
	if err != nil || !started {
		t.Fatalf("IndexAll = (%v, %v)", started, err)
	}
	if !s.CancelIndexing() {
		t.Fatal("a Cancel right after the click was lost")
	}
	waitFor(t, "run to stop", 5*time.Second, func() bool { return !s.IsIndexing() })
	waitFor(t, "cancelled event", 2*time.Second, func() bool { return log.count("indexing:cancelled") >= 1 })
	time.Sleep(200 * time.Millisecond)
	if n := log.count("indexing:cancelled"); n != 1 {
		t.Fatalf("%d cancelled events, want exactly 1", n)
	}
	if n := log.count("indexing:all-done"); n != 0 {
		t.Fatalf("cancelled run emitted all-done (%d)", n)
	}
}
