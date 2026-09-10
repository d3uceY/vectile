package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// SchemaVersion is the current schema version, tracked in the meta table.
const SchemaVersion = 4

// InitSchema creates all tables, virtual tables, and triggers if they don't
// exist. Safe to call on every startup.
func InitSchema(db *sql.DB, embeddingDim int) error {
	schema := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS collections (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			collection_type TEXT NOT NULL DEFAULT 'project',
			description TEXT,
			paths TEXT,
			created_at TEXT DEFAULT (datetime('now'))
		);

		CREATE TABLE IF NOT EXISTS sources (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			collection_id INTEGER NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
			source_type TEXT NOT NULL,
			source_path TEXT NOT NULL,
			file_hash TEXT,
			file_modified_at TEXT,
			last_indexed_at TEXT,
			UNIQUE(collection_id, source_path)
		);

		CREATE TABLE IF NOT EXISTS documents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			source_id INTEGER NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
			collection_id INTEGER NOT NULL REFERENCES collections(id) ON DELETE CASCADE,
			chunk_index INTEGER NOT NULL,
			title TEXT,
			content TEXT NOT NULL,
			metadata TEXT,
			created_at TEXT DEFAULT (datetime('now')),
			UNIQUE(source_id, chunk_index)
		);

		-- Installed embedding models. Rows come from importing a .gguf through
		-- the UI or from scanning the models/ folder; per-model settings live
		-- here and apply when the model is the active one.
		CREATE TABLE IF NOT EXISTS models (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			path TEXT NOT NULL UNIQUE,
			dimensions INTEGER NOT NULL DEFAULT 0,
			context_window INTEGER NOT NULL DEFAULT 0,
			batch_size INTEGER NOT NULL DEFAULT 32,
			threads INTEGER NOT NULL DEFAULT 0,
			is_active INTEGER NOT NULL DEFAULT 0,
			created_at TEXT DEFAULT (datetime('now'))
		);

		-- Cached query embeddings, keyed by the model that produced them, so a
		-- repeated search skips inference. Emptied whenever the model or the
		-- indexed corpus changes. See backend/db/query_cache.go.
		CREATE TABLE IF NOT EXISTS query_cache (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			model_key TEXT NOT NULL,
			query TEXT NOT NULL,
			embedding BLOB NOT NULL,
			created_at TEXT DEFAULT (datetime('now')),
			UNIQUE(model_key, query)
		);

		%s

		CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
			title,
			content,
			content='documents',
			content_rowid='id'
		);

		CREATE TRIGGER IF NOT EXISTS documents_ai AFTER INSERT ON documents BEGIN
			INSERT INTO documents_fts(rowid, title, content)
			VALUES (new.id, new.title, new.content);
		END;

		CREATE TRIGGER IF NOT EXISTS documents_ad AFTER DELETE ON documents BEGIN
			INSERT INTO documents_fts(documents_fts, rowid, title, content)
			VALUES('delete', old.id, old.title, old.content);
		END;

		CREATE TRIGGER IF NOT EXISTS documents_au AFTER UPDATE ON documents BEGIN
			INSERT INTO documents_fts(documents_fts, rowid, title, content)
			VALUES('delete', old.id, old.title, old.content);
			INSERT INTO documents_fts(rowid, title, content)
			VALUES (new.id, new.title, new.content);
		END;

		CREATE TABLE IF NOT EXISTS meta (
			key TEXT PRIMARY KEY,
			value TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_documents_collection_id ON documents(collection_id);

		-- Covers the Browse chunk stream: ordered by (source_id, chunk_index),
		-- filtered by collection. Without it SQLite sorts the whole collection
		-- on every page fetch instead of stopping at LIMIT.
		CREATE INDEX IF NOT EXISTS idx_documents_collection_source_chunk
			ON documents(collection_id, source_id, chunk_index);
	`, vecTablesDDL(embeddingDim))

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	// Track which dimension the vector tables were built at, so a later
	// startup can rebuild them when the active model's dimension differs.
	if _, err := db.Exec("INSERT OR IGNORE INTO meta (key, value) VALUES ('vector_dim', ?)",
		strconv.Itoa(embeddingDim)); err != nil {
		return fmt.Errorf("record vector dim: %w", err)
	}

	// Record schema version once.
	var existing sql.NullString
	err := db.QueryRow("SELECT value FROM meta WHERE key = 'schema_version'").Scan(&existing)
	if err == sql.ErrNoRows {
		_, err = db.Exec("INSERT INTO meta (key, value) VALUES ('schema_version', ?)",
			strconv.Itoa(SchemaVersion))
		if err != nil {
			return fmt.Errorf("set schema version: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("check schema version: %w", err)
	}

	return backfillBinaryVectors(db)
}

// vecTablesDDL returns the two vector virtual-table statements for a given
// embedding dimension. Shared by InitSchema (initial build) and
// RebuildVectorTables (when the active model's dimension changes).
func vecTablesDDL(dim int) string {
	return fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS vec_documents USING vec0(
			embedding float[%d],
			document_id INTEGER
		);

		-- Binary-quantized mirror of vec_documents for fast candidate
		-- retrieval; rowids align with vec_documents so exact float vectors
		-- can be fetched for reranking. See backend/search.
		CREATE VIRTUAL TABLE IF NOT EXISTS vec_documents_bin USING vec0(
			embedding bit[%d],
			document_id INTEGER
		);
	`, dim, dim)
}

// binaryBackfillDoneKey marks that vec_documents_bin has been populated from
// the existing vec_documents rows; InitSchema then skips the check.
const binaryBackfillDoneKey = "binary_backfill_done"

// backfillBinaryVectors populates vec_documents_bin with binary-quantized
// copies of every vec_documents row that lacks one, once (guarded by a meta
// flag). New inserts keep the two tables in sync via InsertEmbedding.
func backfillBinaryVectors(db *sql.DB) error {
	var done sql.NullString
	err := db.QueryRow("SELECT value FROM meta WHERE key = ?", binaryBackfillDoneKey).Scan(&done)
	if err == nil && done.Valid && done.String == "1" {
		return nil
	}
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("check backfill flag: %w", err)
	}

	var floatCount, binCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM vec_documents").Scan(&floatCount); err != nil {
		return fmt.Errorf("count vectors: %w", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM vec_documents_bin").Scan(&binCount); err != nil {
		return fmt.Errorf("count binary vectors: %w", err)
	}

	if binCount != floatCount {
		if binCount > 0 {
			if _, err := db.Exec("DELETE FROM vec_documents_bin"); err != nil {
				return fmt.Errorf("clear partial binary vectors: %w", err)
			}
		}
		if floatCount > 0 {
			if _, err := db.Exec(`
				INSERT INTO vec_documents_bin(rowid, embedding, document_id)
				SELECT rowid, vec_quantize_binary(embedding), document_id
				FROM vec_documents
			`); err != nil {
				return fmt.Errorf("insert binary vectors: %w", err)
			}
		}
	}

	_, err = db.Exec("INSERT OR REPLACE INTO meta (key, value) VALUES (?, '1')", binaryBackfillDoneKey)
	return err
}

// InsertEmbedding inserts an embedding for a document into both the float
// vector table and its binary-quantized mirror, keeping rowids aligned.
func InsertEmbedding(conn *sql.DB, documentID int64, vecBytes []byte) error {
	res, err := conn.Exec(
		"INSERT INTO vec_documents (embedding, document_id) VALUES (?, ?)",
		vecBytes, documentID,
	)
	if err != nil {
		return fmt.Errorf("insert vec: %w", err)
	}
	rowid, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("vec rowid: %w", err)
	}
	if _, err := conn.Exec(
		"INSERT INTO vec_documents_bin (rowid, embedding, document_id) VALUES (?, vec_quantize_binary(?), ?)",
		rowid, vecBytes, documentID,
	); err != nil {
		return fmt.Errorf("insert binary vec: %w", err)
	}
	return nil
}

// PruneSources deletes the given sources with their documents and embeddings
// (float + binary mirror) in one transaction. Deleting from the vec0 tables
// filters on document_id, so doing it once per source would full-scan the
// vector tables per delete; batching keeps it to one scan each.
func PruneSources(conn *sql.DB, sourceIDs []int64) error {
	if len(sourceIDs) == 0 {
		return nil
	}
	inList := intList(sourceIDs)
	docSubquery := "SELECT id FROM documents WHERE source_id IN (" + inList + ")"

	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("begin prune tx: %w", err)
	}
	// Vector rows resolve document ids from documents, so they go first.
	stmts := []string{
		"DELETE FROM vec_documents_bin WHERE document_id IN (" + docSubquery + ")",
		"DELETE FROM vec_documents WHERE document_id IN (" + docSubquery + ")",
		"DELETE FROM documents WHERE source_id IN (" + inList + ")",
		"DELETE FROM sources WHERE id IN (" + inList + ")",
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("prune delete: %w", err)
		}
	}
	return tx.Commit()
}

// intList renders int64 IDs as a comma-separated SQL literal list. IDs are
// internal primary keys, so inlining them is safe from injection.
func intList(ids []int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(parts, ",")
}

// DeleteEmbeddings removes embeddings for the given document IDs from both
// the float vector table and its binary-quantized mirror.
func DeleteEmbeddings(conn *sql.DB, documentIDs []any) error {
	if len(documentIDs) == 0 {
		return nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(documentIDs)), ",")
	if _, err := conn.Exec(
		"DELETE FROM vec_documents_bin WHERE document_id IN ("+placeholders+")", documentIDs...,
	); err != nil {
		return fmt.Errorf("delete binary vecs: %w", err)
	}
	if _, err := conn.Exec(
		"DELETE FROM vec_documents WHERE document_id IN ("+placeholders+")", documentIDs...,
	); err != nil {
		return fmt.Errorf("delete vecs: %w", err)
	}
	return nil
}

// ClearCollectionData removes every source, document, and embedding of a
// collection, keeping the collection row. Used for a full --force rebuild.
func ClearCollectionData(conn *sql.DB, collectionID int64) error {
	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("begin clear tx: %w", err)
	}
	defer tx.Rollback()

	docSubquery := "SELECT id FROM documents WHERE collection_id = ?"
	for _, table := range []string{"vec_documents_bin", "vec_documents"} {
		if _, err := tx.Exec(
			"DELETE FROM "+table+" WHERE document_id IN ("+docSubquery+")", collectionID,
		); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}
	if _, err := tx.Exec("DELETE FROM documents WHERE collection_id = ?", collectionID); err != nil {
		return fmt.Errorf("clear documents: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM sources WHERE collection_id = ?", collectionID); err != nil {
		return fmt.Errorf("clear sources: %w", err)
	}
	return tx.Commit()
}

// ClearSourcesWithPrefix removes sources (and their documents/embeddings)
// whose source_path starts with any of the given prefixes. A code collection
// can hold several repositories, so a rebuild of one must not wipe its
// siblings.
func ClearSourcesWithPrefix(conn *sql.DB, collectionID int64, prefixes []string) error {
	if len(prefixes) == 0 {
		return nil
	}

	var where strings.Builder
	args := []any{collectionID}
	where.WriteString("collection_id = ? AND (")
	for i, prefix := range prefixes {
		if i > 0 {
			where.WriteString(" OR ")
		}
		// substr rather than LIKE: paths routinely contain _ and %, which LIKE
		// would treat as wildcards.
		where.WriteString("substr(source_path, 1, ?) = ?")
		args = append(args, len(prefix), prefix)
	}
	where.WriteString(")")

	tx, err := conn.Begin()
	if err != nil {
		return fmt.Errorf("begin clear tx: %w", err)
	}
	defer tx.Rollback()

	docSubquery := "SELECT id FROM documents WHERE source_id IN (SELECT id FROM sources WHERE " + where.String() + ")"
	for _, table := range []string{"vec_documents_bin", "vec_documents"} {
		if _, err := tx.Exec("DELETE FROM "+table+" WHERE document_id IN ("+docSubquery+")", args...); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}
	if _, err := tx.Exec("DELETE FROM sources WHERE "+where.String(), args...); err != nil {
		return fmt.Errorf("clear sources: %w", err)
	}
	return tx.Commit()
}

// OrphanedVectorStats reports vectors that no longer belong to any document.
type OrphanedVectorStats struct {
	Documents int
	Vectors   int
	Orphaned  int
}

// CountOrphanedVectors reports embeddings whose document no longer exists.
func CountOrphanedVectors(conn *sql.DB) (OrphanedVectorStats, error) {
	var st OrphanedVectorStats
	if err := conn.QueryRow("SELECT COUNT(*) FROM documents").Scan(&st.Documents); err != nil {
		return st, fmt.Errorf("count documents: %w", err)
	}
	if err := conn.QueryRow("SELECT COUNT(*) FROM vec_documents").Scan(&st.Vectors); err != nil {
		return st, fmt.Errorf("count vectors: %w", err)
	}
	ids, err := orphanedVectorRowIDs(conn)
	if err != nil {
		return st, err
	}
	st.Orphaned = len(ids)
	return st, nil
}

// orphanedVectorRowIDs returns the rowids of vec_documents rows whose
// document_id is absent from the documents table.
func orphanedVectorRowIDs(conn *sql.DB) ([]int64, error) {
	live := make(map[int64]struct{})
	rows, err := conn.Query("SELECT id FROM documents")
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			live[id] = struct{}{}
		}
	}
	rows.Close()

	var orphans []int64
	rows, err = conn.Query("SELECT rowid, document_id FROM vec_documents")
	if err != nil {
		return nil, fmt.Errorf("list vectors: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var rowid, docID int64
		if err := rows.Scan(&rowid, &docID); err != nil {
			continue
		}
		if _, ok := live[docID]; !ok {
			orphans = append(orphans, rowid)
		}
	}
	return orphans, rows.Err()
}

// DeleteOrphanedVectors removes embeddings whose document no longer exists,
// returning how many were deleted. Deletion is by rowid (the vec0 primary
// key); vec_documents_bin shares rowids, so the same ids clear both.
func DeleteOrphanedVectors(conn *sql.DB) (int, error) {
	orphans, err := orphanedVectorRowIDs(conn)
	if err != nil {
		return 0, err
	}
	if len(orphans) == 0 {
		return 0, nil
	}

	const chunk = 500
	deleted := 0
	for start := 0; start < len(orphans); start += chunk {
		end := start + chunk
		if end > len(orphans) {
			end = len(orphans)
		}
		batch := orphans[start:end]

		args := make([]any, len(batch))
		for i, id := range batch {
			args[i] = id
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",")

		for _, table := range []string{"vec_documents_bin", "vec_documents"} {
			if _, err := conn.Exec(
				"DELETE FROM "+table+" WHERE rowid IN ("+placeholders+")", args...,
			); err != nil {
				return deleted, fmt.Errorf("delete orphans from %s: %w", table, err)
			}
		}
		deleted += len(batch)
	}
	return deleted, nil
}

// ErrCollectionTypeConflict is returned when a collection name is already in
// use by a different kind of source.
var ErrCollectionTypeConflict = errors.New("collection name already used by a different source type")

// GetOrCreateCollection returns the ID of an existing collection or creates a
// new one. It fails with ErrCollectionTypeConflict rather than reuse a
// collection that belongs to a different source type.
func GetOrCreateCollection(db *sql.DB, name, collectionType string, description *string, paths []string) (int64, error) {
	var pathsJSON *string
	if len(paths) > 0 {
		b, err := json.Marshal(paths)
		if err != nil {
			return 0, fmt.Errorf("marshal paths: %w", err)
		}
		s := string(b)
		pathsJSON = &s
	}

	var id int64
	var existingType string
	err := db.QueryRow("SELECT id, collection_type FROM collections WHERE name = ?", name).Scan(&id, &existingType)
	if err == nil {
		if existingType != collectionType {
			return 0, fmt.Errorf(
				"%w: %q exists as %q but is being indexed as %q — rename one of them in config",
				ErrCollectionTypeConflict, name, existingType, collectionType)
		}
		if pathsJSON != nil {
			if _, err := db.Exec("UPDATE collections SET paths = ? WHERE id = ?", *pathsJSON, id); err != nil {
				return 0, fmt.Errorf("update collection paths: %w", err)
			}
		}
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("query collection: %w", err)
	}

	result, err := db.Exec(
		"INSERT INTO collections (name, collection_type, description, paths) VALUES (?, ?, ?, ?)",
		name, collectionType, description, pathsJSON,
	)
	if err != nil {
		return 0, fmt.Errorf("insert collection: %w", err)
	}
	id, err = result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get collection id: %w", err)
	}
	return id, nil
}

// GetCollectionPaths returns the stored paths for a collection.
func GetCollectionPaths(db *sql.DB, name string) ([]string, error) {
	var pathsJSON sql.NullString
	err := db.QueryRow("SELECT paths FROM collections WHERE name = ?", name).Scan(&pathsJSON)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("collection %q not found", name)
	}
	if err != nil {
		return nil, fmt.Errorf("query collection paths: %w", err)
	}
	if !pathsJSON.Valid || pathsJSON.String == "" {
		return nil, nil
	}
	var paths []string
	if err := json.Unmarshal([]byte(pathsJSON.String), &paths); err != nil {
		return nil, fmt.Errorf("unmarshal paths: %w", err)
	}
	return paths, nil
}

// SetCollectionPaths stores paths for an existing collection.
func SetCollectionPaths(db *sql.DB, name string, paths []string) error {
	var pathsJSON *string
	if len(paths) > 0 {
		b, err := json.Marshal(paths)
		if err != nil {
			return fmt.Errorf("marshal paths: %w", err)
		}
		s := string(b)
		pathsJSON = &s
	}

	result, err := db.Exec("UPDATE collections SET paths = ? WHERE name = ?", pathsJSON, name)
	if err != nil {
		return fmt.Errorf("update collection paths: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("collection %q not found", name)
	}
	return nil
}

// RewriteSourcePaths replaces oldPrefix with newPrefix in source_path for all
// sources of the named collection. Returns the number of rows updated.
func RewriteSourcePaths(d *sql.DB, collectionName, oldPrefix, newPrefix string) (int64, error) {
	var collID int64
	err := d.QueryRow("SELECT id FROM collections WHERE name = ?", collectionName).Scan(&collID)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("collection %q not found", collectionName)
	}
	if err != nil {
		return 0, fmt.Errorf("lookup collection: %w", err)
	}

	result, err := d.Exec(`
		UPDATE sources
		SET source_path = ? || SUBSTR(source_path, ? + 1)
		WHERE collection_id = ?
		  AND source_path LIKE ? || '%'
	`, newPrefix, len(oldPrefix), collID, oldPrefix)
	if err != nil {
		return 0, fmt.Errorf("rewrite source paths: %w", err)
	}
	return result.RowsAffected()
}

// DeleteSourceData removes one source and everything cascading from it: its
// documents (which clears FTS via the AFTER DELETE trigger) and its float +
// binary embeddings. The vec0 tables are not foreign-key linked, so their
// rows are deleted explicitly, in the same transaction, before the documents
// they reference disappear. Returns the number of documents removed.
func DeleteSourceData(conn *sql.DB, sourceID int64) (int64, error) {
	tx, err := conn.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin delete source tx: %w", err)
	}
	defer tx.Rollback()

	var docs int64
	if err := tx.QueryRow("SELECT COUNT(*) FROM documents WHERE source_id = ?", sourceID).Scan(&docs); err != nil {
		return 0, fmt.Errorf("count documents: %w", err)
	}
	docSubquery := "SELECT id FROM documents WHERE source_id = ?"
	for _, table := range []string{"vec_documents_bin", "vec_documents"} {
		if _, err := tx.Exec(
			"DELETE FROM "+table+" WHERE document_id IN ("+docSubquery+")", sourceID,
		); err != nil {
			return 0, fmt.Errorf("delete %s: %w", table, err)
		}
	}
	if _, err := tx.Exec("DELETE FROM documents WHERE source_id = ?", sourceID); err != nil {
		return 0, fmt.Errorf("delete documents: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM sources WHERE id = ?", sourceID); err != nil {
		return 0, fmt.Errorf("delete source: %w", err)
	}
	return docs, tx.Commit()
}

// DeleteCollectionData removes a collection and everything cascading from it:
// its sources, documents (clearing FTS via the AFTER DELETE trigger), float +
// binary embeddings, and the collection row itself. The vec0 tables are not
// foreign-key linked, so their rows are deleted explicitly in the same
// transaction before the documents they reference disappear. Returns the
// number of documents removed.
func DeleteCollectionData(conn *sql.DB, collectionID int64) (int64, error) {
	tx, err := conn.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin delete collection tx: %w", err)
	}
	defer tx.Rollback()

	var docs int64
	if err := tx.QueryRow("SELECT COUNT(*) FROM documents WHERE collection_id = ?", collectionID).Scan(&docs); err != nil {
		return 0, fmt.Errorf("count documents: %w", err)
	}
	docSubquery := "SELECT id FROM documents WHERE collection_id = ?"
	for _, table := range []string{"vec_documents_bin", "vec_documents"} {
		if _, err := tx.Exec(
			"DELETE FROM "+table+" WHERE document_id IN ("+docSubquery+")", collectionID,
		); err != nil {
			return 0, fmt.Errorf("delete %s: %w", table, err)
		}
	}
	if _, err := tx.Exec("DELETE FROM documents WHERE collection_id = ?", collectionID); err != nil {
		return 0, fmt.Errorf("delete documents: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM sources WHERE collection_id = ?", collectionID); err != nil {
		return 0, fmt.Errorf("delete sources: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM collections WHERE id = ?", collectionID); err != nil {
		return 0, fmt.Errorf("delete collection: %w", err)
	}
	return docs, tx.Commit()
}

// DeleteDocumentsData removes the given documents (chunks) and their float +
// binary embeddings in one transaction, leaving the source and collection
// rows intact. The vec0 tables are not foreign-key linked, so their rows are
// deleted explicitly, in the same transaction, before the documents they
// reference disappear. An empty id list is a no-op. Returns the number of
// documents removed.
func DeleteDocumentsData(conn *sql.DB, docIDs []int64) (int64, error) {
	if len(docIDs) == 0 {
		return 0, nil
	}
	tx, err := conn.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin delete documents tx: %w", err)
	}
	defer tx.Rollback()

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(docIDs)), ",")
	args := make([]any, len(docIDs))
	for i, id := range docIDs {
		args[i] = id
	}
	for _, table := range []string{"vec_documents_bin", "vec_documents"} {
		if _, err := tx.Exec(
			"DELETE FROM "+table+" WHERE document_id IN ("+placeholders+")", args...,
		); err != nil {
			return 0, fmt.Errorf("delete %s: %w", table, err)
		}
	}
	res, err := tx.Exec("DELETE FROM documents WHERE id IN ("+placeholders+")", args...)
	if err != nil {
		return 0, fmt.Errorf("delete documents: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, tx.Commit()
}
