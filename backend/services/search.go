package services

import (
	"vectile/backend/db"
	"vectile/backend/search"
)

// SearchService exposes hybrid search to the frontend.
type SearchService struct{ core *Core }

// NewSearchService creates a SearchService bound to the shared core.
func NewSearchService(core *Core) *SearchService { return &SearchService{core: core} }

// Search runs hybrid vector + FTS search with the given filters.
func (s *SearchService) Search(query string, filters search.Filters) (search.SearchResponse, error) {
	return search.Search(db.DB, query, filters, s.core.Embedder, s.core.Cfg.SearchDefaults)
}

// GetCacheStats reports what the cached query vectors currently hold.
func (s *SearchService) GetCacheStats() (db.QueryCacheStats, error) {
	return db.GetQueryCacheStats(db.DB)
}

// ClearCache drops every cached query vector and returns how many were
// removed. Searches simply re-embed from here; nothing in the library changes.
func (s *SearchService) ClearCache() (int64, error) {
	return db.ClearQueryCache(db.DB)
}
