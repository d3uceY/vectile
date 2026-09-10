package services

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sort"

	"vectile/backend/config"
	"vectile/backend/db"
	"vectile/backend/indexer"
	"vectile/backend/startup"
)

// IndexService exposes configuration, indexing, and pruning to the frontend.
type IndexService struct{ core *Core }

// NewIndexService creates an IndexService bound to the shared core.
func NewIndexService(core *Core) *IndexService { return &IndexService{core: core} }

// GetConfig returns the current configuration.
func (s *IndexService) GetConfig() *config.Config { return s.core.Cfg }

// SetConfig saves the configuration and applies its side effects
// (start-on-login). Auto-reindex is read live by the loop in main.
func (s *IndexService) SetConfig(cfg config.Config) error {
	// The active model, its batch size, and the display-name fallback are
	// owned by the ModelService (and applied when a model is activated). The
	// Settings form's config draft may carry stale copies, so preserve the
	// live values here rather than let a save revert them.
	cfg.ActiveModel = s.core.Cfg.ActiveModel
	cfg.EmbeddingModel = s.core.Cfg.EmbeddingModel
	cfg.EmbeddingBatchSize = s.core.Cfg.EmbeddingBatchSize
	if err := config.Save(&cfg, s.core.CfgPath); err != nil {
		return err
	}
	s.core.Cfg = &cfg
	s.applyStartup(cfg.GUI.StartOnLogin)
	return nil
}

// AddSourcePath adds a source path. kind is "vault", "calibre", "project", or
// "repo"; name is the collection name for project/repo (ignored otherwise).
func (s *IndexService) AddSourcePath(kind, name, path string) error {
	cfg := s.core.Cfg
	switch kind {
	case "vault":
		cfg.ObsidianVaults = appendUnique(cfg.ObsidianVaults, path)
	case "calibre":
		cfg.CalibreLibraries = appendUnique(cfg.CalibreLibraries, path)
	case "project":
		cfg.Projects = setMapSlice(cfg.Projects, name,
			appendUnique(append([]string(nil), cfg.Projects[name]...), path))
	case "repo":
		cfg.Repositories = setMapSlice(cfg.Repositories, name,
			appendUnique(append([]string(nil), cfg.Repositories[name]...), path))
	default:
		return fmt.Errorf("unknown source kind %q", kind)
	}
	return s.persistConfig()
}

// RemoveSourcePath removes a source path from a config section.
func (s *IndexService) RemoveSourcePath(kind, name, path string) error {
	cfg := s.core.Cfg
	switch kind {
	case "vault":
		cfg.ObsidianVaults = removeStr(cfg.ObsidianVaults, path)
	case "calibre":
		cfg.CalibreLibraries = removeStr(cfg.CalibreLibraries, path)
	case "project":
		cfg.Projects = setMapSlice(cfg.Projects, name,
			removeStr(append([]string(nil), cfg.Projects[name]...), path))
	case "repo":
		cfg.Repositories = setMapSlice(cfg.Repositories, name,
			removeStr(append([]string(nil), cfg.Repositories[name]...), path))
	default:
		return fmt.Errorf("unknown source kind %q", kind)
	}
	return s.persistConfig()
}

// ToggleCollectionEnabled enables/disables a collection for indexing.
func (s *IndexService) ToggleCollectionEnabled(name string, enabled bool) error {
	cfg := s.core.Cfg
	cfg.DisabledCollections = removeStr(cfg.DisabledCollections, name)
	if !enabled {
		cfg.DisabledCollections = append(cfg.DisabledCollections, name)
	}
	return s.persistConfig()
}

// IndexCollection starts indexing one collection in the background, emitting
// indexing:file, indexing:progress, and indexing:complete / indexing:cancelled
// events. Returns false when another index run is already in progress, so the
// frontend never gets stuck in an "indexing" state for a run that never
// actually started.
func (s *IndexService) IndexCollection(name string, force bool) (bool, error) {
	if !s.startIndexRun() {
		return false, nil
	}
	go func() {
		defer s.finishIndexRun(name)
		s.core.resetIndexRun(false)
		ctx := s.core.newIndexContext()
		defer s.core.clearIndexContext()
		s.runIndex(ctx, name, force)
	}()
	return true, nil
}

// IndexAll starts pruning + indexing every enabled, configured collection.
func (s *IndexService) IndexAll(force bool) (bool, error) {
	if !s.startIndexRun() {
		return false, nil
	}
	go func() {
		defer s.finishIndexRun("")
		s.core.resetIndexRun(true)
		ctx := s.core.newIndexContext()
		defer s.core.clearIndexContext()
		if pr := indexer.PruneAll(db.DB, s.core.Cfg); pr.Pruned > 0 {
			s.core.App.Event.Emit("indexing:pruned", pr.Pruned)
		}
		for _, name := range s.configuredCollections() {
			if ctx.Err() != nil {
				break
			}
			s.runIndex(ctx, name, force)
		}
		// The whole index-all run is done (all collections, or the first
		// cancellation): the frontend reloads the library exactly once here
		// instead of once per collection. Only announce a finished run; a
		// cancellation already told the frontend its state.
		if ctx.Err() == nil {
			s.core.App.Event.Emit("indexing:all-done", nil)
			s.core.sendNotification("index-all", "Indexing finished", "All collections indexed")
		}
	}()
	return true, nil
}

// IndexSynchronous indexes one collection in the calling goroutine and
// returns the run summary. It is used by the MCP vectile_index write tool so
// the client gets the result directly. It still emits the indexing events, so
// an open frontend Index view stays in sync. Errors when another index run is
// already in progress.
func (s *IndexService) IndexSynchronous(name string, force bool) (result *indexer.IndexResult, err error) {
	if !s.startIndexRun() {
		return nil, fmt.Errorf("an index run is already in progress")
	}
	defer func() {
		if r := recover(); r != nil {
			s.reportIndexPanic(name, r)
			result = &indexer.IndexResult{Errors: 1, ErrorMessages: []string{fmt.Sprintf("indexing crashed: %v", r)}}
		}
		s.core.clearIndexRun()
		s.unlockIndex()
	}()
	s.core.resetIndexRun(false)
	ctx := s.core.newIndexContext()
	defer s.core.clearIndexContext()
	result = s.runIndex(ctx, name, force)
	return result, nil
}

// CancelIndexing aborts the active index run, if any, and reports whether one
// was running.
func (s *IndexService) CancelIndexing() bool {
	return s.core.cancelIndex()
}

// Prune removes stale sources from a collection ("all" or "" for everything).
func (s *IndexService) Prune(name string) (indexer.PruneResult, error) {
	var res indexer.PruneResult
	if name == "" || name == "all" {
		res = *indexer.PruneAll(db.DB, s.core.Cfg)
	} else {
		res = *indexer.PruneCollection(db.DB, s.core.Cfg, name)
	}
	s.core.clearQueryCache("prune")
	return res, nil
}

// DeleteSource removes one indexed source and everything cascading from it:
// its documents (FTS cleared via the delete trigger) and its float + binary
// embeddings. The source's path stays in config — this is one file among many
// — so the next index pass will re-add it if the file still exists on disk.
// Returns the number of documents removed, and errors if the source does not
// exist or an index run is in progress.
func (s *IndexService) DeleteSource(sourceID int64) (int64, error) {
	if s.IsIndexing() {
		return 0, fmt.Errorf("cannot delete while an index run is in progress")
	}
	n, err := db.DeleteSourceData(db.DB, sourceID)
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, fmt.Errorf("source %d not found", sourceID)
	}
	return n, nil
}

// DeleteDocuments removes the given chunks (documents) and their float +
// binary embeddings in one pass; FTS is cleared via the delete trigger. The
// source and collection rows stay, so re-indexing restores the deleted
// chunks. An empty list is a no-op. Errors if an index run is in progress.
// Returns the number of documents removed.
func (s *IndexService) DeleteDocuments(docIDs []int64) (int64, error) {
	if s.IsIndexing() {
		return 0, fmt.Errorf("cannot delete while an index run is in progress")
	}
	return db.DeleteDocumentsData(db.DB, docIDs)
}

// DeleteCollection removes a collection and everything cascading from it:
// its sources, documents (FTS cleared via the delete trigger), float + binary
// embeddings, and the collection row. It also removes the collection's config
// entry — an obsidian/calibre collection owns all its vault/library paths, a
// project/repo collection owns its whole group — so it does not silently
// resurrect on the next index pass. Files on disk are never touched. Works
// even when the collection was never indexed (config-only). Returns the
// number of documents removed.
func (s *IndexService) DeleteCollection(name string) (int64, error) {
	if s.IsIndexing() {
		return 0, fmt.Errorf("cannot delete while an index run is in progress")
	}

	// Remove indexed data first (if any), so a failed delete leaves config
	// untouched and the collection still visible.
	var deleted int64
	var id int64
	err := db.DB.QueryRow("SELECT id FROM collections WHERE name = ?", name).Scan(&id)
	switch {
	case err == nil:
		n, derr := db.DeleteCollectionData(db.DB, id)
		if derr != nil {
			return 0, derr
		}
		deleted = n
	case err == sql.ErrNoRows:
		// Never indexed — nothing to delete from the DB, just config below.
	default:
		return 0, err
	}

	// Drop the config entry so the collection doesn't come back on the next
	// index/auto-reindex pass.
	cfg := s.core.Cfg
	switch name {
	case "obsidian":
		cfg.ObsidianVaults = nil
	case "calibre":
		cfg.CalibreLibraries = nil
	default:
		cfg.Projects = withoutMapKey(cfg.Projects, name)
		cfg.Repositories = withoutMapKey(cfg.Repositories, name)
	}
	cfg.DisabledCollections = removeStr(cfg.DisabledCollections, name)
	if err := s.persistConfig(); err != nil {
		return 0, err
	}
	return deleted, nil
}

// IsIndexing reports whether an index run is in progress.
func (s *IndexService) IsIndexing() bool {
	s.core.indexMu.Lock()
	defer s.core.indexMu.Unlock()
	return s.core.indexing
}

// GetIndexingState returns a snapshot of the active index run (if any) so a
// frontend that reloads or reconnects mid-run can rebuild the indexing UI
// instead of showing nothing. Live updates still arrive as events; this only
// seeds the initial state on (re)load.
func (s *IndexService) GetIndexingState() IndexState {
	s.core.indexMu.Lock()
	active := s.core.indexing
	s.core.indexMu.Unlock()

	s.core.progressMu.Lock()
	defer s.core.progressMu.Unlock()
	st := IndexState{Active: active, All: s.core.allRun}
	if len(s.core.progress) > 0 {
		st.Collections = make(map[string]IndexFileProgress, len(s.core.progress))
		for k, v := range s.core.progress {
			st.Collections[k] = v
		}
	}
	return st
}

// resetIndexRun clears the live-progress snapshot at the start of a fresh run.
func (c *Core) resetIndexRun(all bool) {
	c.progressMu.Lock()
	defer c.progressMu.Unlock()
	c.allRun = all
	c.progress = map[string]IndexFileProgress{}
}

// recordIndexProgress stores the latest per-file progress for a collection so
// a (re)connecting frontend can reconstruct it.
func (c *Core) recordIndexProgress(p IndexFileProgress) {
	c.progressMu.Lock()
	defer c.progressMu.Unlock()
	if c.progress != nil {
		c.progress[p.Collection] = p
	}
}

// clearIndexProgress drops a collection from the live snapshot once its run
// completes (or is cancelled).
func (c *Core) clearIndexProgress(collection string) {
	c.progressMu.Lock()
	defer c.progressMu.Unlock()
	if c.progress != nil {
		delete(c.progress, collection)
	}
}

// clearIndexRun drops the whole live snapshot at the end of a run.
func (c *Core) clearIndexRun() {
	c.progressMu.Lock()
	defer c.progressMu.Unlock()
	c.allRun = false
	c.progress = nil
}

func (s *IndexService) lockIndex() bool {
	s.core.indexMu.Lock()
	defer s.core.indexMu.Unlock()
	if s.core.indexing {
		return false
	}
	s.core.indexing = true
	return true
}

// startIndexRun claims the index lock and drops the cached query vectors, so a
// run that rewrites the corpus never leaves vectors cached from before it.
// Returns false when another run already holds the lock.
func (s *IndexService) startIndexRun() bool {
	if !s.lockIndex() {
		return false
	}
	s.core.clearQueryCache("index run")
	return true
}

// clearQueryCache drops the cached query vectors. A query vector is valid only
// for the model that produced it, so every path that changes the embedding
// model or rewrites the indexed corpus calls this. Best effort: a failed clear
// is logged rather than returned, because the cache is only an optimization.
func (c *Core) clearQueryCache(reason string) {
	if db.DB == nil {
		return
	}
	n, err := db.ClearQueryCache(db.DB)
	if err != nil {
		slog.Warn("clear query cache failed", "reason", reason, "err", err)
		return
	}
	if n > 0 {
		slog.Info("query cache cleared", "reason", reason, "rows", n)
	}
}

func (s *IndexService) unlockIndex() {
	s.core.indexMu.Lock()
	s.core.indexing = false
	s.core.indexMu.Unlock()
}

// finishIndexRun is deferred by the background index goroutines so a panic in
// the indexing pipeline is caught and reported instead of crashing the app
// (an unrecovered panic in a goroutine terminates the whole process). It then
// clears the run state and releases the index lock.
func (s *IndexService) finishIndexRun(name string) {
	if r := recover(); r != nil {
		s.reportIndexPanic(name, r)
	}
	s.core.clearIndexRun()
	s.unlockIndex()
}

// reportIndexPanic logs a caught index panic (with its stack) and tells the
// frontend, so the Index view does not sit stuck on "indexing".
func (s *IndexService) reportIndexPanic(name string, r any) {
	slog.Error("index run panicked", "collection", name, "panic", r, "stack", string(debug.Stack()))
	if s.core.App != nil {
		s.core.App.Event.Emit("indexing:failed", IndexFailed{
			Collection: name,
			Message:    fmt.Sprintf("indexing crashed: %v", r),
		})
	}
}

// runIndex dispatches one collection to its indexer and emits progress events.
// It returns the run summary so a synchronous caller (the MCP write tool) can
// report it; the async IndexCollection/IndexAll discard the value.
func (s *IndexService) runIndex(ctx context.Context, name string, force bool) *indexer.IndexResult {
	cfg := s.core.Cfg
	progress := func(current, total int, item string) {
		// Per-file event: the frontend increments its per-collection count.
		p := IndexFileProgress{Collection: name, File: item, Indexed: current, Total: total}
		s.core.recordIndexProgress(p)
		s.core.App.Event.Emit("indexing:file", p)
		// Throttled aggregate progress (the old shape), kept for summaries.
		if current == total || current%5 == 0 {
			s.core.App.Event.Emit("indexing:progress", IndexProgress{
				Collection: name, Current: current, Total: total, Item: item,
			})
		}
	}

	var result *indexer.IndexResult
	switch {
	case name == "obsidian":
		result = indexer.IndexObsidian(ctx, db.DB, cfg, force, progress, s.core.Embedder)
	case name == "calibre":
		result = indexer.IndexCalibre(ctx, db.DB, cfg, force, progress, s.core.Embedder)
	case len(cfg.Repositories[name]) > 0:
		agg := &indexer.IndexResult{}
		for _, repo := range indexer.ResolveRepoPaths(cfg.Repositories[name]) {
			agg.Merge(indexer.IndexGitRepo(ctx, db.DB, cfg, repo, name, force,
				cfg.GitHistoryInMonths > 0, progress, s.core.Embedder))
			if ctx.Err() != nil {
				break
			}
		}
		result = agg
	case len(cfg.Projects[name]) > 0:
		result = indexer.IndexProject(ctx, db.DB, cfg, name, cfg.Projects[name], force, progress, s.core.Embedder)
	default:
		result = &indexer.IndexResult{Errors: 1, ErrorMessages: []string{"collection not configured: " + name}}
	}

	if ctx.Err() != nil {
		s.core.App.Event.Emit("indexing:cancelled", IndexCancelled{
			Collection: name, Indexed: result.Indexed, Skipped: result.Skipped, Errors: result.Errors,
		})
		s.core.clearIndexProgress(name)
		return result
	}

	s.core.App.Event.Emit("indexing:complete", IndexComplete{
		Collection: name,
		Indexed:    result.Indexed,
		Skipped:    result.Skipped,
		Errors:     result.Errors,
		Messages:   result.ErrorMessages,
	})
	// A single-collection run notifies here; an Index All notifies once at
	// indexing:all-done below.
	if !s.core.isAllRun() && s.core.App != nil {
		s.core.sendNotification("index-"+name, "Indexing finished",
			fmt.Sprintf("Indexed %s · %d new", name, result.Indexed))
	}
	s.core.clearIndexProgress(name)
	return result
}

// configuredCollections returns the enabled, configured collections in a
// deterministic order: system (obsidian, calibre), then repos, then projects.
func (s *IndexService) configuredCollections() []string {
	cfg := s.core.Cfg
	var names []string
	if cfg.IsCollectionEnabled("obsidian") && len(cfg.ObsidianVaults) > 0 {
		names = append(names, "obsidian")
	}
	if cfg.IsCollectionEnabled("calibre") && len(cfg.CalibreLibraries) > 0 {
		names = append(names, "calibre")
	}
	repos := sortedKeys(cfg.Repositories)
	for _, n := range repos {
		if len(cfg.Repositories[n]) > 0 && cfg.IsCollectionEnabled(n) {
			names = append(names, n)
		}
	}
	projects := sortedKeys(cfg.Projects)
	for _, n := range projects {
		if len(cfg.Projects[n]) > 0 && cfg.IsCollectionEnabled(n) {
			names = append(names, n)
		}
	}
	return names
}

func (s *IndexService) persistConfig() error {
	return config.Save(s.core.Cfg, s.core.CfgPath)
}

func (s *IndexService) applyStartup(enabled bool) {
	cur, err := startup.IsEnabled()
	if err != nil {
		return
	}
	if enabled && !cur {
		_ = startup.Enable()
	}
	if !enabled && cur {
		_ = startup.Disable()
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func appendUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

func removeStr(list []string, v string) []string {
	out := list[:0]
	for _, x := range list {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

// setMapSlice returns a shallow copy of m with key set to val. The Projects /
// Repositories maps are shared between the index goroutine and frontend request
// handlers, so they must never be mutated in place: writing to a map while
// another goroutine reads or iterates it is a fatal "concurrent map read and
// map write" that crashes the whole process.
func setMapSlice(m map[string][]string, key string, val []string) map[string][]string {
	out := make(map[string][]string, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	out[key] = val
	return out
}

// withoutMapKey returns a shallow copy of m with key removed (or m itself when
// the key is absent), again so a config map is never mutated while the index
// goroutine may be reading it.
func withoutMapKey(m map[string][]string, key string) map[string][]string {
	if _, ok := m[key]; !ok {
		return m
	}
	out := make(map[string][]string, len(m))
	for k, v := range m {
		if k != key {
			out[k] = v
		}
	}
	return out
}
