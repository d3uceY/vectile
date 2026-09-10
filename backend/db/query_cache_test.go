package db

import (
	"database/sql"
	"fmt"
	"testing"
)

// cacheVec returns a full-dim unit vector with its spike at i.
func cacheVec(i int) []float32 {
	v := make([]float32, EmbeddingDim)
	v[i] = 1
	return v
}

func openCacheDB(t *testing.T) *sql.DB {
	t.Helper()
	conn := testDB(t)
	if err := InitSchema(conn, EmbeddingDim); err != nil {
		t.Fatal(err)
	}
	return conn
}

func TestQueryCacheRoundtrip(t *testing.T) {
	conn := openCacheDB(t)

	if _, hit, err := GetCachedQueryVector(conn, "/m/bge.gguf", "how do we ship"); err != nil || hit {
		t.Fatalf("empty cache should miss: hit=%v err=%v", hit, err)
	}
	if err := PutCachedQueryVector(conn, "/m/bge.gguf", "how do we ship", cacheVec(3)); err != nil {
		t.Fatal(err)
	}

	got, hit, err := GetCachedQueryVector(conn, "/m/bge.gguf", "how do we ship")
	if err != nil {
		t.Fatal(err)
	}
	if !hit {
		t.Fatal("expected a cache hit")
	}
	if len(got) != EmbeddingDim || got[3] != 1 {
		t.Fatalf("roundtrip returned %d dims, spike at 3 = %v", len(got), got[3])
	}
}

// TestQueryCacheKeepsModelsApart is the correctness property the model key
// exists for: a vector embedded by one model is never served for another.
func TestQueryCacheKeepsModelsApart(t *testing.T) {
	conn := openCacheDB(t)
	if err := PutCachedQueryVector(conn, "/m/a.gguf", "cats", cacheVec(1)); err != nil {
		t.Fatal(err)
	}
	if _, hit, err := GetCachedQueryVector(conn, "/m/b.gguf", "cats"); err != nil || hit {
		t.Fatalf("another model must not read that entry: hit=%v err=%v", hit, err)
	}
}

func TestQueryCacheDuplicatePutKeepsFirst(t *testing.T) {
	conn := openCacheDB(t)
	if err := PutCachedQueryVector(conn, "/m/a.gguf", "cats", cacheVec(1)); err != nil {
		t.Fatal(err)
	}
	// Two searches can race on the same query; the later write is ignored.
	if err := PutCachedQueryVector(conn, "/m/a.gguf", "cats", cacheVec(2)); err != nil {
		t.Fatal(err)
	}
	if n := countTable(t, conn, "query_cache", "query = ?", "cats"); n != 1 {
		t.Fatalf("expected 1 row, got %d", n)
	}
	got, _, err := GetCachedQueryVector(conn, "/m/a.gguf", "cats")
	if err != nil {
		t.Fatal(err)
	}
	if got[1] != 1 {
		t.Fatalf("expected the first vector to win, spike at 1 = %v", got[1])
	}
}

func TestQueryCacheSkipsEmptyInput(t *testing.T) {
	conn := openCacheDB(t)
	if err := PutCachedQueryVector(conn, "/m/a.gguf", "cats", nil); err != nil {
		t.Fatal(err)
	}
	if err := PutCachedQueryVector(conn, "", "cats", cacheVec(1)); err != nil {
		t.Fatal(err)
	}
	if n := countTable(t, conn, "query_cache", "1 = ?", 1); n != 0 {
		t.Fatalf("expected nothing cached, got %d rows", n)
	}
}

// TestQueryCacheDropsCorruptBlob covers a hand-edited or damaged row: it must
// read as a miss and remove itself, not degrade every later search.
func TestQueryCacheDropsCorruptBlob(t *testing.T) {
	conn := openCacheDB(t)
	if _, err := conn.Exec(
		"INSERT INTO query_cache (model_key, query, embedding) VALUES (?,?,?)",
		"/m/a.gguf", "cats", []byte{1, 2, 3, 4, 5},
	); err != nil {
		t.Fatal(err)
	}
	if _, hit, err := GetCachedQueryVector(conn, "/m/a.gguf", "cats"); err != nil || hit {
		t.Fatalf("corrupt blob must read as a miss: hit=%v err=%v", hit, err)
	}
	if n := countTable(t, conn, "query_cache", "1 = ?", 1); n != 0 {
		t.Fatalf("corrupt row should be deleted, %d rows left", n)
	}
}

func TestQueryCacheClearReturnsCount(t *testing.T) {
	conn := openCacheDB(t)
	for i := 0; i < 3; i++ {
		if err := PutCachedQueryVector(conn, "/m/a.gguf", fmt.Sprintf("q%d", i), cacheVec(i)); err != nil {
			t.Fatal(err)
		}
	}
	n, err := ClearQueryCache(conn)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("cleared %d rows, want 3", n)
	}
	if n, err := ClearQueryCache(conn); err != nil || n != 0 {
		t.Fatalf("second clear removed %d rows (err=%v), want 0", n, err)
	}
}

func TestQueryCacheStats(t *testing.T) {
	conn := openCacheDB(t)
	st, err := GetQueryCacheStats(conn)
	if err != nil {
		t.Fatal(err)
	}
	if st.Entries != 0 || st.Bytes != 0 {
		t.Fatalf("empty stats = %+v", st)
	}

	if err := PutCachedQueryVector(conn, "/m/a.gguf", "cats", cacheVec(1)); err != nil {
		t.Fatal(err)
	}
	st, err = GetQueryCacheStats(conn)
	if err != nil {
		t.Fatal(err)
	}
	if st.Entries != 1 {
		t.Fatalf("entries = %d, want 1", st.Entries)
	}
	if want := int64(EmbeddingDim*4 + len("cats")); st.Bytes != want {
		t.Fatalf("bytes = %d, want %d", st.Bytes, want)
	}
}

func TestQueryCacheEvictsOldest(t *testing.T) {
	conn := openCacheDB(t)
	// Shrink the cap so the test doesn't have to write thousands of rows.
	old := queryCacheMaxEntries
	queryCacheMaxEntries = 5
	t.Cleanup(func() { queryCacheMaxEntries = old })

	over := queryCacheMaxEntries + 3
	for i := 0; i < over; i++ {
		if err := PutCachedQueryVector(conn, "/m/a.gguf", fmt.Sprintf("q%d", i), cacheVec(1)); err != nil {
			t.Fatal(err)
		}
	}
	if n := countTable(t, conn, "query_cache", "1 = ?", 1); n != queryCacheMaxEntries {
		t.Fatalf("cache holds %d rows, want %d", n, queryCacheMaxEntries)
	}
	if _, hit, _ := GetCachedQueryVector(conn, "/m/a.gguf", "q0"); hit {
		t.Fatal("oldest entry should have been evicted")
	}
	if _, hit, _ := GetCachedQueryVector(conn, "/m/a.gguf", fmt.Sprintf("q%d", over-1)); !hit {
		t.Fatal("newest entry should still be cached")
	}
}
