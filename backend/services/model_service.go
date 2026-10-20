package services

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"vectile/backend/appdata"
	"vectile/backend/config"
	"vectile/backend/db"
	"vectile/backend/embeddings"
)

// ModelService manages installed embedding models: importing a .gguf through
// the UI, picking the active model, per-model settings, and keeping the
// models table in sync with the models/ folder.
type ModelService struct{ core *Core }

// defaultBatchSize is the embedding batch size applied to models registered by
// the folder scan (and to UI imports). It matches the schema and config
// defaults; setting it explicitly keeps a zero-value Model struct from writing
// a meaningless 0 into batch_size, which would otherwise shadow the DB default.
const defaultBatchSize = 32

// NewModelService creates a ModelService bound to the shared core.
func NewModelService(core *Core) *ModelService { return &ModelService{core: core} }

// SetActiveResult reports the outcome of SetActiveModel. NeedsRebuild means the
// switch was not applied because it would strand embeddings; call again with force.
type SetActiveResult struct {
	NeedsRebuild bool     `json:"needsRebuild"`
	Model        db.Model `json:"model"`
}

// ListModels reconciles the models table with the models/ folder (adding any
// .gguf that isn't registered, removing rows whose file has vanished) and
// returns every installed model.
func (s *ModelService) ListModels() ([]db.Model, error) {
	if _, _, err := s.syncModelsFromFolder(); err != nil {
		return nil, err
	}
	return db.ListModels(db.DB)
}

// ImportModel copies a .gguf into the models/ folder and registers it. The
// file is copied so the app owns it and the model survives the original
// being moved or deleted. It is not auto-activated; the user picks it.
func (s *ModelService) ImportModel(srcPath string) (db.Model, error) {
	if !strings.EqualFold(filepath.Ext(srcPath), ".gguf") {
		return db.Model{}, fmt.Errorf("not a GGUF file (expected a .gguf): %q", srcPath)
	}
	dims, context, err := embeddings.ReadMetadata(srcPath)
	if err != nil {
		return db.Model{}, fmt.Errorf("read model metadata: %w", err)
	}
	if dims <= 0 {
		return db.Model{}, fmt.Errorf("not an embedding model: no embedding dimension in the GGUF metadata")
	}
	dest, err := copyIntoModels(srcPath)
	if err != nil {
		return db.Model{}, err
	}
	id, err := db.UpsertModel(db.DB, db.Model{
		Name: strings.TrimSuffix(filepath.Base(dest), ".gguf"), Path: dest,
		Dimensions: dims, ContextWindow: context, BatchSize: defaultBatchSize,
	})
	if err != nil {
		return db.Model{}, err
	}
	slog.Info("imported model", "id", id, "path", dest, "dims", dims)
	m, ok, err := db.GetModelByPath(db.DB, dest)
	if err != nil || !ok {
		return db.Model{}, fmt.Errorf("reload imported model: %w", err)
	}
	return m, nil
}

// SetActiveModel makes the model at path active. Any model change needs a
// reindex: two models can share a dimension and still map text differently, so
// their vectors are not comparable and the vector tables are rebuilt. With
// embeddings present the switch waits for force, so the UI can confirm first.
func (s *ModelService) SetActiveModel(path string, force bool) (SetActiveResult, error) {
	if s.isIndexing() {
		return SetActiveResult{}, fmt.Errorf("cannot switch models while an index run is in progress")
	}
	m, ok, err := db.GetModelByPath(db.DB, path)
	if err != nil || !ok {
		return SetActiveResult{}, fmt.Errorf("model not found: %q", path)
	}
	if _, err := os.Stat(m.Path); err != nil {
		return SetActiveResult{}, fmt.Errorf("model file missing: %q", m.Path)
	}

	vectorDim, err := db.GetVectorDim(db.DB)
	if err != nil {
		return SetActiveResult{}, err
	}
	dimChange := m.Dimensions > 0 && vectorDim > 0 && m.Dimensions != vectorDim
	modelChange, err := s.activeModelDiffers(m)
	if err != nil {
		return SetActiveResult{}, err
	}
	rebuild := dimChange || modelChange
	if rebuild && !force {
		n, err := db.CountEmbeddings(db.DB)
		if err != nil {
			return SetActiveResult{}, err
		}
		// Nothing embedded means nothing to lose: apply, dimension fix-up included.
		if n > 0 {
			return SetActiveResult{NeedsRebuild: true, Model: m}, nil
		}
	}
	if err := s.applyActive(m, rebuild); err != nil {
		return SetActiveResult{}, err
	}
	return SetActiveResult{Model: m}, nil
}

// activeModelDiffers reports whether m is not the active model ("none active"
// counts as different).
func (s *ModelService) activeModelDiffers(m db.Model) (bool, error) {
	active, ok, err := db.GetActiveModel(db.DB)
	if err != nil {
		return false, err
	}
	return !ok || active.Path != m.Path, nil
}

// DeleteModel removes a model from the table and, when the file lives in the
// models/ folder, deletes the file too so the folder scan doesn't re-add it.
// The active model cannot be deleted.
func (s *ModelService) DeleteModel(path string) error {
	if s.isIndexing() {
		return fmt.Errorf("cannot delete a model while an index run is in progress")
	}
	m, ok, err := db.GetModelByPath(db.DB, path)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if m.IsActive {
		return fmt.Errorf("cannot delete the active model; switch to another model first")
	}
	if _, err := db.DeleteModelByPath(db.DB, m.Path); err != nil {
		return err
	}
	if filepath.Dir(m.Path) == appdata.ModelsDir() {
		_ = os.Remove(m.Path)
	}
	return nil
}

// UpdateModelSettings updates a model's per-model settings (context window in
// tokens, embedding batch size, CPU threads). When the edited model is the
// active one, the new settings apply to the running embedder immediately.
func (s *ModelService) UpdateModelSettings(id int64, contextWindow, batchSize, threads int) error {
	if err := db.UpdateModelSettings(db.DB, id, contextWindow, batchSize, threads); err != nil {
		return err
	}
	active, ok, err := db.GetActiveModel(db.DB)
	if err != nil {
		return err
	}
	if ok && active.ID == id {
		if err := s.core.Embedder.SetModel(active.Path, contextWindow, threads); err != nil {
			return err
		}
		s.core.clearQueryCache("active model settings changed")
		if batchSize > 0 {
			if err := s.core.UpdateConfig(func(c *config.Config) {
				c.EmbeddingBatchSize = batchSize
			}); err != nil {
				return err
			}
		}
		return nil
	}
	return nil
}

// ApplyActiveModel runs at startup: it reconciles the models folder, then
// loads the active model (or the configured default) with its per-model
// settings, rebuilding the vector tables if their dimension no longer matches
// the active model. Returns an error only when the DB/embedder can't be made
// consistent; a missing model falls back to the default path.
func (s *ModelService) ApplyActiveModel() error {
	if _, _, err := s.syncModelsFromFolder(); err != nil {
		return err
	}
	active, ok, err := db.GetActiveModel(db.DB)
	if err != nil {
		return err
	}
	cfg := s.core.Config()
	path := ""
	if ok {
		path = active.Path
	} else if cfg != nil && cfg.ActiveModel != "" {
		path = cfg.ActiveModel
	}
	if path == "" {
		return s.core.Embedder.SetModel(appdata.ModelPath(), 0, 0)
	}
	m, has, err := db.GetModelByPath(db.DB, path)
	if err != nil {
		return err
	}
	if !has {
		// Configured model vanished (reconcile already dropped the row).
		if cfg != nil && cfg.ActiveModel != "" {
			_ = s.core.UpdateConfig(func(c *config.Config) { c.ActiveModel = "" })
		}
		return s.core.Embedder.SetModel(appdata.ModelPath(), 0, 0)
	}
	vectorDim, err := db.GetVectorDim(db.DB)
	if err != nil {
		return err
	}
	if m.Dimensions > 0 && vectorDim > 0 && m.Dimensions != vectorDim {
		if err := db.RebuildVectorTables(db.DB, m.Dimensions); err != nil {
			return err
		}
	}
	return s.applyActive(m, false)
}

// applyActive registers the model as active, loads it into the embedder with
// its per-model settings, mirrors its batch size into config, and persists
// the active-model path. rebuild is true when the vector tables were (or must
// be) rebuilt for a new dimension.
func (s *ModelService) applyActive(m db.Model, rebuild bool) error {
	if rebuild {
		if err := db.RebuildVectorTables(db.DB, m.Dimensions); err != nil {
			return fmt.Errorf("rebuild vector tables: %w", err)
		}
	}
	if err := db.SetActiveModelByPath(db.DB, m.Path); err != nil {
		return err
	}
	// Cached query vectors belong to the model that produced them, so a real
	// model change (or a rebuilt vector space) drops them. Re-applying the same
	// model and dimension at startup must NOT clear, or the cache would never
	// survive a restart.
	modelChanged := rebuild || s.core.Embedder.ModelPath() != m.Path
	if err := s.core.Embedder.SetModel(m.Path, m.ContextWindow, m.Threads); err != nil {
		return err
	}
	if modelChanged {
		s.core.clearQueryCache("active model changed")
	}
	if err := s.core.UpdateConfig(func(c *config.Config) {
		if m.BatchSize > 0 {
			c.EmbeddingBatchSize = m.BatchSize
		}
		c.ActiveModel = m.Path
		c.EmbeddingModel = m.Name // keep the display-name fallback accurate
	}); err != nil {
		return err
	}
	slog.Info("active model", "path", m.Path, "dims", m.Dimensions, "rebuild", rebuild)
	if s.core.App != nil {
		s.core.App.Event.Emit("model:changed", m)
	}
	return nil
}

// syncModelsFromFolder reconciles the models table with the models/ folder:
// any .gguf not registered is added, and any registered model whose file has
// vanished is removed (clearing the active selection if it was active).
func (s *ModelService) syncModelsFromFolder() (added, removed int, err error) {
	dir := appdata.ModelsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, fmt.Errorf("read models dir: %w", err)
	}
	// Remove partial downloads left by a crash mid-download.
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".part") {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".gguf") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		seen[p] = true
		dims, context, _ := embeddings.ReadMetadata(p)
		if dims <= 0 {
			continue // not a text-embedding model; leave it out of the list
		}
		if _, err := db.UpsertModel(db.DB, db.Model{
			Name:          strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())),
			Path:          p,
			Dimensions:    dims,
			ContextWindow: context,
			BatchSize:     defaultBatchSize,
		}); err != nil {
			return added, removed, err
		}
		added++
	}

	models, err := db.ListModels(db.DB)
	if err != nil {
		return added, removed, err
	}
	for _, m := range models {
		if seen[m.Path] {
			continue
		}
		if _, statErr := os.Stat(m.Path); statErr == nil {
			continue // still on disk (path outside models/); leave it
		}
		if m.IsActive {
			if err := db.ClearActiveModel(db.DB); err != nil {
				return added, removed, err
			}
			s.core.clearQueryCache("active model removed")
			_ = s.core.UpdateConfig(func(c *config.Config) { c.ActiveModel = "" })
		}
		if _, err := db.DeleteModelByPath(db.DB, m.Path); err != nil {
			return added, removed, err
		}
		removed++
	}
	return added, removed, nil
}

func (s *ModelService) isIndexing() bool {
	s.core.indexMu.Lock()
	defer s.core.indexMu.Unlock()
	return s.core.indexing
}

// copyIntoModels copies a .gguf into the models/ folder (or returns its path
// unchanged when it's already there), uniquifying the name on collision so the
// original file is never overwritten.
func copyIntoModels(src string) (string, error) {
	dir := appdata.ModelsDir()
	if filepath.Dir(src) == dir {
		return filepath.Join(dir, filepath.Base(src)), nil
	}
	base := filepath.Base(src)
	dest := filepath.Join(dir, base)
	if _, err := os.Stat(dest); err == nil {
		ext := filepath.Ext(base)
		stem := strings.TrimSuffix(base, ext)
		for i := 1; ; i++ {
			cand := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", stem, i, ext))
			if _, err := os.Stat(cand); os.IsNotExist(err) {
				dest = cand
				break
			}
		}
	}
	in, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("open source model: %w", err)
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("create copy in models/: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dest)
		return "", fmt.Errorf("copy model: %w", err)
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	return dest, nil
}
