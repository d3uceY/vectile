package services

import (
	"fmt"
	"testing"

	"vectile/backend/db"
)

// seedQueryCache puts n cached query vectors in the shared DB.
func seedQueryCache(t *testing.T, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		vec := make([]float32, db.EmbeddingDim)
		vec[i] = 1
		if err := db.PutCachedQueryVector(db.DB, "/m/a.gguf", fmt.Sprintf("q%d", i), vec); err != nil {
			t.Fatal(err)
		}
	}
}

func cacheRows(t *testing.T) int {
	t.Helper()
	var n int
	if err := db.DB.QueryRow("SELECT COUNT(*) FROM query_cache").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// registerModel adds a fake .gguf on disk and registers it.
func registerModel(t *testing.T, name string) db.Model {
	t.Helper()
	path := writeFakeModel(t, name, db.EmbeddingDim)
	if _, err := db.UpsertModel(db.DB, db.Model{
		Name: name, Path: path, Dimensions: db.EmbeddingDim, BatchSize: 32,
	}); err != nil {
		t.Fatal(err)
	}
	m, ok, err := db.GetModelByPath(db.DB, path)
	if err != nil || !ok {
		t.Fatalf("model not registered: ok=%v err=%v", ok, err)
	}
	return m
}

// TestSetActiveModelClearsQueryCache covers the correctness trigger: vectors
// embedded by one model must not survive into another model's results. It also
// pins the exception: re-activating the same model at startup keeps the cache,
// so cached queries survive a restart.
func TestSetActiveModelClearsQueryCache(t *testing.T) {
	svc := modelTestEnv(t)
	a := registerModel(t, "a.gguf")
	if _, err := svc.SetActiveModel(a.Path, false); err != nil {
		t.Fatal(err)
	}

	seedQueryCache(t, 2)
	if _, err := svc.SetActiveModel(a.Path, false); err != nil {
		t.Fatal(err)
	}
	if n := cacheRows(t); n != 2 {
		t.Fatalf("re-applying the same model should keep the cache, got %d rows", n)
	}

	b := registerModel(t, "b.gguf")
	if _, err := svc.SetActiveModel(b.Path, false); err != nil {
		t.Fatal(err)
	}
	if n := cacheRows(t); n != 0 {
		t.Fatalf("switching models must clear the cache, got %d rows", n)
	}
}

func TestUpdateModelSettingsClearsQueryCache(t *testing.T) {
	svc := modelTestEnv(t)
	a := registerModel(t, "a.gguf")
	if _, err := svc.SetActiveModel(a.Path, false); err != nil {
		t.Fatal(err)
	}

	seedQueryCache(t, 1)
	if err := svc.UpdateModelSettings(a.ID, 4096, 32, 4); err != nil {
		t.Fatal(err)
	}
	if n := cacheRows(t); n != 0 {
		t.Fatalf("retuning the active model must clear the cache, got %d rows", n)
	}
}

// TestStartIndexRunClearsQueryCache covers the whole re-index family: every
// entry point claims the run through startIndexRun.
func TestStartIndexRunClearsQueryCache(t *testing.T) {
	svc := modelTestEnv(t)
	idx := NewIndexService(svc.core)

	seedQueryCache(t, 2)
	if !idx.startIndexRun() {
		t.Fatal("expected the index run to start")
	}
	if n := cacheRows(t); n != 0 {
		t.Fatalf("an index run must clear the cache, got %d rows", n)
	}
	if idx.startIndexRun() {
		t.Fatal("a second run should be refused while the first holds the lock")
	}
}

func TestPruneClearsQueryCache(t *testing.T) {
	svc := modelTestEnv(t)
	idx := NewIndexService(svc.core)

	seedQueryCache(t, 2)
	if _, err := idx.Prune("all"); err != nil {
		t.Fatal(err)
	}
	if n := cacheRows(t); n != 0 {
		t.Fatalf("pruning must clear the cache, got %d rows", n)
	}
}

func TestSearchServiceCacheStatsAndClear(t *testing.T) {
	svc := modelTestEnv(t)
	ss := NewSearchService(svc.core)

	st, err := ss.GetCacheStats()
	if err != nil {
		t.Fatal(err)
	}
	if st.Entries != 0 || st.Bytes != 0 {
		t.Fatalf("empty stats = %+v", st)
	}

	seedQueryCache(t, 3)
	st, err = ss.GetCacheStats()
	if err != nil {
		t.Fatal(err)
	}
	if st.Entries != 3 {
		t.Fatalf("entries = %d, want 3", st.Entries)
	}
	if st.Bytes <= 0 {
		t.Fatalf("bytes = %d, want > 0", st.Bytes)
	}

	n, err := ss.ClearCache()
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("cleared %d rows, want 3", n)
	}
	st, err = ss.GetCacheStats()
	if err != nil {
		t.Fatal(err)
	}
	if st.Entries != 0 {
		t.Fatalf("entries after clear = %d, want 0", st.Entries)
	}
}
