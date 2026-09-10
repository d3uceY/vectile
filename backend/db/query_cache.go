package db

import (
	"database/sql"
	"fmt"

	"vectile/backend/embeddings"
)

// queryCacheMaxEntries bounds the query-vector cache. Cached entries are cheap
// to rebuild, so the oldest are dropped once the table grows past this; at
// about 4 KB per vector that caps the table near 8 MB. It is a var only so a
// test can shrink it.
var queryCacheMaxEntries = 2000

// GetCachedQueryVector returns the embedding cached for a query under the
// given model key. The second result is false when there is no usable entry.
// A row holding something other than a float32 blob is deleted and reported as
// a miss, so one corrupt row can't quietly degrade every later search.
func GetCachedQueryVector(conn *sql.DB, modelKey, query string) ([]float32, bool, error) {
	var blob []byte
	err := conn.QueryRow(
		"SELECT embedding FROM query_cache WHERE model_key = ? AND query = ?",
		modelKey, query,
	).Scan(&blob)
	switch {
	case err == sql.ErrNoRows:
		return nil, false, nil
	case err != nil:
		return nil, false, fmt.Errorf("read cached query vector: %w", err)
	}
	if len(blob) == 0 || len(blob)%4 != 0 {
		_, _ = conn.Exec("DELETE FROM query_cache WHERE model_key = ? AND query = ?", modelKey, query)
		return nil, false, nil
	}
	return embeddings.DeserializeFloat32(blob), true, nil
}

// PutCachedQueryVector stores an embedding for a query under a model key.
// Duplicate writes are ignored, so two searches racing on the same query are
// harmless. An empty vector or model key is not cached at all.
func PutCachedQueryVector(conn *sql.DB, modelKey, query string, vec []float32) error {
	if modelKey == "" || len(vec) == 0 {
		return nil
	}
	if _, err := conn.Exec(
		"INSERT OR IGNORE INTO query_cache (model_key, query, embedding) VALUES (?, ?, ?)",
		modelKey, query, embeddings.SerializeFloat32(vec),
	); err != nil {
		return fmt.Errorf("store cached query vector: %w", err)
	}
	return evictQueryCache(conn)
}

// evictQueryCache keeps the newest queryCacheMaxEntries rows and drops the
// rest. It is FIFO rather than LRU on purpose: tracking recency would mean a
// write on every cache hit, and a hit is otherwise a pure read.
//
// The cutoff id is the one sitting queryCacheMaxEntries rows from the end, so
// a cache under the cap yields NULL and deletes nothing.
func evictQueryCache(conn *sql.DB) error {
	_, err := conn.Exec(
		`DELETE FROM query_cache WHERE id <= (
			SELECT id FROM query_cache ORDER BY id DESC LIMIT 1 OFFSET ?
		)`, queryCacheMaxEntries)
	if err != nil {
		return fmt.Errorf("evict query cache: %w", err)
	}
	return nil
}

// ClearQueryCache drops every cached query vector and returns how many rows
// were removed. Callers clear it whenever the embedding model or the indexed
// corpus changes.
func ClearQueryCache(conn *sql.DB) (int64, error) {
	res, err := conn.Exec("DELETE FROM query_cache")
	if err != nil {
		return 0, fmt.Errorf("clear query cache: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// QueryCacheStats summarises the query-vector cache for the Settings panel.
type QueryCacheStats struct {
	Entries int   `json:"entries"`
	Bytes   int64 `json:"bytes"`
}

// GetQueryCacheStats reports how much the query-vector cache holds.
func GetQueryCacheStats(conn *sql.DB) (QueryCacheStats, error) {
	var st QueryCacheStats
	err := conn.QueryRow(`
		SELECT COUNT(*),
		       COALESCE(SUM(LENGTH(embedding) + LENGTH(query)), 0)
		FROM query_cache`).Scan(&st.Entries, &st.Bytes)
	if err != nil {
		return QueryCacheStats{}, fmt.Errorf("read query cache stats: %w", err)
	}
	return st, nil
}
