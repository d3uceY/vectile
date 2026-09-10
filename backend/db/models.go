package db

import (
	"database/sql"
	"fmt"
	"strconv"
)

// Model is one installed embedding model. Rows are created by importing a
// .gguf through the UI or by scanning the models/ folder; per-model settings
// live here and apply when the model is the active one.
type Model struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	Dimensions    int    `json:"dimensions"`
	ContextWindow int    `json:"contextWindow"`
	BatchSize     int    `json:"batchSize"`
	Threads       int    `json:"threads"`
	IsActive      bool   `json:"isActive"`
	Created       string `json:"created"`
}

const vectorDimKey = "vector_dim"

// DefaultVectorDim is the dimension used for a brand-new database.
const DefaultVectorDim = 1024

// UpsertModel inserts a model row (by unique path) or updates the fields that
// may change on a folder re-scan (name, dims) while preserving per-model
// settings the user may have tuned. On conflict it also backfills native
// defaults the row may be missing: a context_window of 0 is filled from the
// file's metadata when the file reports one, and a batch_size of 0 (the
// legacy folder-scan zero-value bug) is repaired. Returns the row id.
func UpsertModel(conn *sql.DB, m Model) (int64, error) {
	res, err := conn.Exec(`
		INSERT INTO models (name, path, dimensions, context_window, batch_size, threads, is_active)
		VALUES (?, ?, ?, ?, ?, ?, 0)
		ON CONFLICT(path) DO UPDATE SET
			name = excluded.name,
			dimensions = CASE WHEN excluded.dimensions > 0 THEN excluded.dimensions ELSE models.dimensions END,
			context_window = CASE WHEN models.context_window = 0 AND excluded.context_window > 0
				THEN excluded.context_window ELSE models.context_window END,
			batch_size = CASE WHEN models.batch_size <= 0 AND excluded.batch_size > 0
				THEN excluded.batch_size ELSE models.batch_size END`,
		m.Name, m.Path, m.Dimensions, m.ContextWindow, m.BatchSize, m.Threads,
	)
	if err != nil {
		return 0, fmt.Errorf("upsert model: %w", err)
	}
	return res.LastInsertId()
}

// ListModels returns all installed models, newest first.
func ListModels(conn *sql.DB) ([]Model, error) {
	rows, err := conn.Query(`SELECT id, name, path, dimensions, context_window,
		batch_size, threads, is_active, created_at FROM models ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanModels(rows)
}

// GetModelByPath returns the model with the given path, if present.
func GetModelByPath(conn *sql.DB, path string) (Model, bool, error) {
	row := conn.QueryRow(`SELECT id, name, path, dimensions, context_window,
		batch_size, threads, is_active, created_at FROM models WHERE path = ?`, path)
	return scanModel(row)
}

// GetActiveModel returns the currently active model, if any.
func GetActiveModel(conn *sql.DB) (Model, bool, error) {
	row := conn.QueryRow(`SELECT id, name, path, dimensions, context_window,
		batch_size, threads, is_active, created_at FROM models WHERE is_active = 1`)
	return scanModel(row)
}

// SetActiveModelByPath makes the model at path the single active model.
func SetActiveModelByPath(conn *sql.DB, path string) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE models SET is_active = 0`); err != nil {
		return fmt.Errorf("clear active model: %w", err)
	}
	res, err := tx.Exec(`UPDATE models SET is_active = 1 WHERE path = ?`, path)
	if err != nil {
		return fmt.Errorf("set active model: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("no model at %q", path)
	}
	return tx.Commit()
}

// ClearActiveModel clears the active-model flag without setting a new one.
func ClearActiveModel(conn *sql.DB) error {
	_, err := conn.Exec(`UPDATE models SET is_active = 0`)
	return err
}

// CountEmbeddings returns how many documents have embeddings, so model
// activation can tell whether rebuilding the vector tables would lose data.
func CountEmbeddings(conn *sql.DB) (int, error) {
	var n int
	err := conn.QueryRow(`SELECT COUNT(*) FROM vec_documents`).Scan(&n)
	return n, err
}

// DeleteModelByPath removes a model row. Returns whether a row was removed.
func DeleteModelByPath(conn *sql.DB, path string) (bool, error) {
	res, err := conn.Exec(`DELETE FROM models WHERE path = ?`, path)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// UpdateModelSettings updates a model's per-model settings. contextWindow 0
// means "use the model's native maximum"; threads 0 means runtime.NumCPU().
func UpdateModelSettings(conn *sql.DB, id int64, contextWindow, batchSize, threads int) error {
	res, err := conn.Exec(`UPDATE models SET context_window = ?, batch_size = ?, threads = ?
		WHERE id = ?`, contextWindow, batchSize, threads, id)
	if err != nil {
		return fmt.Errorf("update model settings: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("no model with id %d", id)
	}
	return nil
}

// GetVectorDim returns the dimension the vector tables were built at.
func GetVectorDim(conn *sql.DB) (int, error) {
	var v sql.NullString
	if err := conn.QueryRow("SELECT value FROM meta WHERE key = ?", vectorDimKey).Scan(&v); err != nil {
		return DefaultVectorDim, nil
	}
	if !v.Valid {
		return DefaultVectorDim, nil
	}
	n, err := strconv.Atoi(v.String)
	if err != nil || n <= 0 {
		return DefaultVectorDim, nil
	}
	return n, nil
}

// RebuildVectorTables drops and recreates the two vector tables at a new
// dimension. Destructive: every stored embedding is lost until re-indexed,
// so this is only called after the user confirms a dimension change.
func RebuildVectorTables(conn *sql.DB, dim int) error {
	tx, err := conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DROP TABLE IF EXISTS vec_documents_bin`); err != nil {
		return fmt.Errorf("drop binary vec table: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE IF EXISTS vec_documents`); err != nil {
		return fmt.Errorf("drop vec table: %w", err)
	}
	if _, err := tx.Exec(vecTablesDDL(dim)); err != nil {
		return fmt.Errorf("recreate vec tables: %w", err)
	}
	// Binary mirror is empty again; let the startup backfill re-check.
	if _, err := tx.Exec(`DELETE FROM meta WHERE key = ?`, binaryBackfillDoneKey); err != nil {
		return fmt.Errorf("reset backfill flag: %w", err)
	}
	// Cached query vectors were built for the old dimension.
	if _, err := tx.Exec(`DELETE FROM query_cache`); err != nil {
		return fmt.Errorf("clear query cache: %w", err)
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)`,
		vectorDimKey, strconv.Itoa(dim)); err != nil {
		return fmt.Errorf("record vector dim: %w", err)
	}
	return tx.Commit()
}

func scanModels(rows *sql.Rows) ([]Model, error) {
	var out []Model
	for rows.Next() {
		var m Model
		var created sql.NullString
		if err := rows.Scan(&m.ID, &m.Name, &m.Path, &m.Dimensions, &m.ContextWindow,
			&m.BatchSize, &m.Threads, &m.IsActive, &created); err != nil {
			return nil, err
		}
		if created.Valid {
			m.Created = created.String
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func scanModel(row *sql.Row) (Model, bool, error) {
	var m Model
	var created sql.NullString
	err := row.Scan(&m.ID, &m.Name, &m.Path, &m.Dimensions, &m.ContextWindow,
		&m.BatchSize, &m.Threads, &m.IsActive, &created)
	if err == sql.ErrNoRows {
		return m, false, nil
	}
	if err != nil {
		return m, false, err
	}
	if created.Valid {
		m.Created = created.String
	}
	return m, true, nil
}
