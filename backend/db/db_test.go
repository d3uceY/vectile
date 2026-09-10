package db

import (
	"database/sql"
	"path/filepath"
	"testing"

	"vectile/backend/embeddings"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestInitSchemaIdempotent(t *testing.T) {
	conn := testDB(t)
	if err := InitSchema(conn, EmbeddingDim); err != nil {
		t.Fatal(err)
	}
	// Calling again on startup must be a no-op.
	if err := InitSchema(conn, EmbeddingDim); err != nil {
		t.Fatal(err)
	}

	var version string
	if err := conn.QueryRow("SELECT value FROM meta WHERE key = 'schema_version'").Scan(&version); err != nil {
		t.Fatalf("schema version not recorded: %v", err)
	}
	if version != "4" {
		t.Fatalf("schema version = %q", version)
	}
}

// TestVectorAndFTSRoundtrip verifies the vec0 tables (float + binary mirror)
// and the FTS5 triggers all work end to end on modernc.
func TestVectorAndFTSRoundtrip(t *testing.T) {
	conn := testDB(t)
	if err := InitSchema(conn, EmbeddingDim); err != nil {
		t.Fatal(err)
	}

	collID, err := GetOrCreateCollection(conn, "test", "project", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	srcRes, err := conn.Exec(
		"INSERT INTO sources (collection_id, source_type, source_path) VALUES (?,?,?)",
		collID, "markdown", "/tmp/note.md",
	)
	if err != nil {
		t.Fatal(err)
	}
	sourceID, _ := srcRes.LastInsertId()

	docRes, err := conn.Exec(
		"INSERT INTO documents (source_id, collection_id, chunk_index, title, content) VALUES (?,?,?,?,?)",
		sourceID, collID, 0, "Note", "the quick brown fox jumps over the lazy dog",
	)
	if err != nil {
		t.Fatal(err)
	}
	docID, _ := docRes.LastInsertId()

	// The AFTER INSERT trigger keeps FTS5 in sync.
	var fts int
	if err := conn.QueryRow(
		"SELECT COUNT(*) FROM documents_fts WHERE documents_fts MATCH '\"quick\"'",
	).Scan(&fts); err != nil {
		t.Fatal(err)
	}
	if fts != 1 {
		t.Fatalf("expected 1 FTS hit, got %d", fts)
	}

	// Insert a full-dim embedding; both tables get a row, rowids aligned.
	vec := make([]float32, EmbeddingDim)
	vec[0], vec[1] = 1, 0.5
	if err := InsertEmbedding(conn, docID, embeddings.SerializeFloat32(vec)); err != nil {
		t.Fatal(err)
	}

	var fRows, bRows int
	if err := conn.QueryRow("SELECT COUNT(*) FROM vec_documents WHERE document_id = ?", docID).Scan(&fRows); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow("SELECT COUNT(*) FROM vec_documents_bin WHERE document_id = ?", docID).Scan(&bRows); err != nil {
		t.Fatal(err)
	}
	if fRows != 1 || bRows != 1 {
		t.Fatalf("expected 1 float + 1 binary row, got %d + %d", fRows, bRows)
	}

	// Binary-quantized KNN must return the row (vec_quantize_binary support).
	rows, err := conn.Query(
		"SELECT rowid FROM vec_documents_bin WHERE embedding MATCH vec_quantize_binary(?) ORDER BY distance LIMIT 5",
		embeddings.SerializeFloat32(vec),
	)
	if err != nil {
		t.Fatalf("vec_quantize_binary unsupported: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("binary KNN returned no rows")
	}
}

func TestGetOrCreateCollectionConflict(t *testing.T) {
	conn := testDB(t)
	if err := InitSchema(conn, EmbeddingDim); err != nil {
		t.Fatal(err)
	}
	if _, err := GetOrCreateCollection(conn, "x", "project", nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := GetOrCreateCollection(conn, "x", "code", nil, nil); err == nil {
		t.Fatal("expected ErrCollectionTypeConflict")
	}
}

// seedDoc inserts a collection + source + one document with a full-dim
// embedding, so delete helpers can be verified against all tables at once.
func seedDoc(t *testing.T, conn *sql.DB, collName, sourcePath string) (collID, sourceID, docID int64) {
	t.Helper()
	collID, err := GetOrCreateCollection(conn, collName, "project", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	srcRes, err := conn.Exec(
		"INSERT INTO sources (collection_id, source_type, source_path) VALUES (?,?,?)",
		collID, "markdown", sourcePath,
	)
	if err != nil {
		t.Fatal(err)
	}
	sourceID, _ = srcRes.LastInsertId()
	docRes, err := conn.Exec(
		"INSERT INTO documents (source_id, collection_id, chunk_index, title, content) VALUES (?,?,?,?,?)",
		sourceID, collID, 0, "Note", "the quick brown fox jumps over the lazy dog",
	)
	if err != nil {
		t.Fatal(err)
	}
	docID, _ = docRes.LastInsertId()
	vec := make([]float32, EmbeddingDim)
	vec[0] = 1
	if err := InsertEmbedding(conn, docID, embeddings.SerializeFloat32(vec)); err != nil {
		t.Fatal(err)
	}
	return
}

func countTable(t *testing.T, conn *sql.DB, table, where string, arg any) int {
	t.Helper()
	var n int
	if err := conn.QueryRow("SELECT COUNT(*) FROM "+table+" WHERE "+where, arg).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// TestDeleteSourceDataCascade verifies deleting one source removes its
// documents, FTS rows, and float + binary embeddings (the vec0 tables have no
// FK, so they must be cleaned manually), while leaving the collection intact.
func TestDeleteSourceDataCascade(t *testing.T) {
	conn := testDB(t)
	if err := InitSchema(conn, EmbeddingDim); err != nil {
		t.Fatal(err)
	}
	_, sourceID, docID := seedDoc(t, conn, "notes", "/tmp/note.md")

	n, err := DeleteSourceData(conn, sourceID)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 document deleted, got %d", n)
	}
	if got := countTable(t, conn, "sources", "id = ?", sourceID); got != 0 {
		t.Fatalf("source still present (%d rows)", got)
	}
	if got := countTable(t, conn, "documents", "source_id = ?", sourceID); got != 0 {
		t.Fatalf("documents still present (%d rows)", got)
	}
	if got := countTable(t, conn, "documents_fts", "rowid = ?", docID); got != 0 {
		t.Fatalf("fts rows still present (%d rows)", got)
	}
	if got := countTable(t, conn, "vec_documents", "document_id = ?", docID); got != 0 {
		t.Fatalf("float vectors still present (%d rows)", got)
	}
	if got := countTable(t, conn, "vec_documents_bin", "document_id = ?", docID); got != 0 {
		t.Fatalf("binary vectors still present (%d rows)", got)
	}
	// The collection itself survives a source delete.
	if got := countTable(t, conn, "collections", "name = 'notes'", nil); got != 1 {
		t.Fatalf("collection should survive source delete, got %d rows", got)
	}
}

// TestDeleteCollectionDataCascade verifies deleting a collection removes its
// sources, documents, FTS rows, embeddings, and the collection row itself.
func TestDeleteCollectionDataCascade(t *testing.T) {
	conn := testDB(t)
	if err := InitSchema(conn, EmbeddingDim); err != nil {
		t.Fatal(err)
	}
	collID, _, _ := seedDoc(t, conn, "notes", "/tmp/note.md")

	n, err := DeleteCollectionData(conn, collID)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 document deleted, got %d", n)
	}
	for _, table := range []string{"collections", "sources", "documents", "documents_fts", "vec_documents", "vec_documents_bin"} {
		var total int
		if err := conn.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&total); err != nil {
			t.Fatal(err)
		}
		if total != 0 {
			t.Fatalf("%s not empty after collection delete (%d rows)", table, total)
		}
	}
}

// TestDeleteDocumentsDataCascade verifies deleting a subset of a source's
// documents removes only those chunks, their FTS rows, and their float +
// binary embeddings, leaving the source, collection, and the other chunks
// intact.
func TestDeleteDocumentsDataCascade(t *testing.T) {
	conn := testDB(t)
	if err := InitSchema(conn, EmbeddingDim); err != nil {
		t.Fatal(err)
	}
	_, sourceID, keepID := seedDoc(t, conn, "notes", "/tmp/note.md")

	// A second chunk in the same source; this one gets deleted.
	docRes, err := conn.Exec(
		"INSERT INTO documents (source_id, collection_id, chunk_index, title, content) VALUES (?,?,?,?,?)",
		sourceID, 0, 1, "Note 2", "a second chunk of content",
	)
	if err != nil {
		t.Fatal(err)
	}
	dropID, _ := docRes.LastInsertId()
	vec := make([]float32, EmbeddingDim)
	vec[1] = 1
	if err := InsertEmbedding(conn, dropID, embeddings.SerializeFloat32(vec)); err != nil {
		t.Fatal(err)
	}

	n, err := DeleteDocumentsData(conn, []int64{dropID})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 document deleted, got %d", n)
	}
	// Dropped chunk is gone everywhere: documents row, FTS row, both vector
	// tables. The kept chunk's rows all survive.
	if got := countTable(t, conn, "documents", "id = ?", dropID); got != 0 {
		t.Fatalf("dropped document still present (%d rows)", got)
	}
	if got := countTable(t, conn, "documents_fts", "rowid = ?", dropID); got != 0 {
		t.Fatalf("dropped fts row still present (%d rows)", got)
	}
	if got := countTable(t, conn, "vec_documents", "document_id = ?", dropID); got != 0 {
		t.Fatalf("dropped float vector still present (%d rows)", got)
	}
	if got := countTable(t, conn, "vec_documents_bin", "document_id = ?", dropID); got != 0 {
		t.Fatalf("dropped binary vector still present (%d rows)", got)
	}
	if got := countTable(t, conn, "vec_documents", "document_id = ?", keepID); got != 1 {
		t.Fatalf("kept float vector missing (%d rows)", got)
	}
	if got := countTable(t, conn, "vec_documents_bin", "document_id = ?", keepID); got != 1 {
		t.Fatalf("kept binary vector missing (%d rows)", got)
	}
	// The kept chunk is still there; source + collection survive.
	if got := countTable(t, conn, "documents", "id = ?", keepID); got != 1 {
		t.Fatalf("kept chunk missing (%d rows)", got)
	}
	if got := countTable(t, conn, "sources", "id = ?", sourceID); got != 1 {
		t.Fatalf("source should survive chunk delete, got %d rows", got)
	}
	// An empty id list is a harmless no-op.
	if _, err := DeleteDocumentsData(conn, nil); err != nil {
		t.Fatalf("empty delete failed: %v", err)
	}
}
