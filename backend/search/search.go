// Package search implements hybrid vector + FTS5 search with Reciprocal Rank
// Fusion. The query embedding comes from the in-process llama.go embedder.
package search

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"

	"vectile/backend/config"
	"vectile/backend/embeddings"
)

// SearchResult is a single search result, mirrored to the frontend.
type SearchResult struct {
	Content    string         `json:"content"`
	Title      string         `json:"title"`
	Metadata   map[string]any `json:"metadata"`
	Score      float64        `json:"score"`
	Collection string         `json:"collection"`
	SourcePath string         `json:"sourcePath"`
	SourceType string         `json:"sourceType"`
}

// Filters holds optional filters for search queries.
type Filters struct {
	Collection      string            `json:"collection"`
	SourceType      string            `json:"sourceType"`
	Path            string            `json:"path"`
	DateFrom        string            `json:"dateFrom"`
	DateTo          string            `json:"dateTo"`
	Sender          string            `json:"sender"`
	Author          string            `json:"author"`
	MetadataFilters map[string]string `json:"metadataFilters"`
	TopK            int               `json:"topK"`
}

func (f *Filters) hasFilters() bool {
	if f == nil {
		return false
	}
	return f.Collection != "" || f.SourceType != "" || f.Path != "" || f.Sender != "" ||
		f.Author != "" || f.DateFrom != "" || f.DateTo != "" || len(f.MetadataFilters) > 0
}

type rankedResult struct {
	docID int64
	score float64
}

// collectionTypes is the set of valid collection type names.
var collectionTypes = map[string]bool{
	"system":  true,
	"project": true,
	"code":    true,
}

// QueryEmbedder embeds the search query on demand.
type QueryEmbedder interface {
	Embed(text string) ([]float32, error)
}

// Search runs hybrid search (vector + FTS5 fused with RRF) for query.
func Search(db *sql.DB, query string, filters Filters, embedder QueryEmbedder, sd config.SearchDefaults) ([]SearchResult, error) {
	topK := filters.TopK
	if topK <= 0 {
		topK = sd.TopK
	}
	if topK <= 0 {
		topK = 10
	}

	vecResults, err := vectorSearch(db, query, topK, &filters, embedder)
	if err != nil {
		// Vector search failing (e.g. no model) shouldn't kill FTS.
		slog.Warn("vector search failed, falling back to FTS only", "err", err)
		vecResults = nil
	}

	ftsResults, err := ftsSearch(db, query, topK, &filters)
	if err != nil {
		return nil, err
	}

	merged := RRFMerge(vecResults, ftsResults, sd.RRFK, sd.VectorWeight, sd.FTSWeight)

	results := make([]SearchResult, 0, len(merged))
	for _, r := range merged {
		if res, err := fetchResult(db, r.docID, r.score); err == nil {
			results = append(results, *res)
		}
	}
	return results, nil
}

// vectorCandidatePool returns how many binary-quantized candidates to retrieve
// before reranking. Binary (Hamming) search is cheap, so we over-fetch a
// generous pool and rerank it with exact float distances. When filters are
// active we widen the pool further, since filtering happens after retrieval.
func vectorCandidatePool(topK int, filters *Filters) int {
	pool := topK * 20
	if pool < 200 {
		pool = 200
	}
	if filters.hasFilters() {
		pool = topK * 100
		if pool < 1000 {
			pool = 1000
		}
	}
	if pool > 4000 {
		pool = 4000
	}
	return pool
}

// binaryCandidate is a candidate surfaced by the binary-quantized search,
// carrying the vec_documents rowid used to fetch its exact float vector.
type binaryCandidate struct {
	rowid int64
	docID int64
}

// vectorSearch embeds the query, runs binary-quantized Hamming KNN to gather
// a candidate pool, then reranks the pool with exact float L2 distances.
func vectorSearch(db *sql.DB, query string, topK int, filters *Filters, embedder QueryEmbedder) ([]rankedResult, error) {
	queryVec, err := embedder.Embed(query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	// TEMP DEBUG: verify the query actually produced a real embedding.
	// slog.Info("[vector-debug] embedded query",
	// 	"query", query,
	// 	"dim", len(queryVec),
	// 	"l2norm", vectorNorm(queryVec),
	// 	"first8", queryVec[:min(8, len(queryVec))],
	// )
	queryBlob := embeddings.SerializeFloat32(queryVec)
	pool := vectorCandidatePool(topK, filters)

	// Stage 1: Hamming-distance KNN over the binary mirror.
	rows, err := db.Query(
		`SELECT rowid, document_id
		 FROM vec_documents_bin
		 WHERE embedding MATCH vec_quantize_binary(?) AND k = ?
		 ORDER BY distance`,
		queryBlob, pool,
	)
	if err != nil {
		return nil, fmt.Errorf("binary vector search: %w", err)
	}
	defer rows.Close()

	var candidates []binaryCandidate
	for rows.Next() {
		var c binaryCandidate
		if err := rows.Scan(&c.rowid, &c.docID); err != nil {
			return nil, fmt.Errorf("scan binary candidate: %w", err)
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		slog.Info("[vector-debug] no binary candidates", "query", query)
		return nil, nil
	}
	slog.Info("[vector-debug] binary candidates", "count", len(candidates))

	// Stage 2: fetch exact float vectors for the candidates by rowid (point
	// lookups, no full scan) and rerank with squared L2.
	rowidList := make([]string, len(candidates))
	docIDByRowid := make(map[int64]int64, len(candidates))
	for i, c := range candidates {
		rowidList[i] = strconv.FormatInt(c.rowid, 10)
		docIDByRowid[c.rowid] = c.docID
	}

	frows, err := db.Query(fmt.Sprintf(
		"SELECT rowid, embedding FROM vec_documents WHERE rowid IN (%s)",
		strings.Join(rowidList, ","),
	))
	if err != nil {
		return nil, fmt.Errorf("fetch candidate vectors: %w", err)
	}
	defer frows.Close()

	reranked := make([]rankedResult, 0, len(candidates))
	for frows.Next() {
		var rowid int64
		var blob []byte
		if err := frows.Scan(&rowid, &blob); err != nil {
			return nil, fmt.Errorf("scan candidate vector: %w", err)
		}
		vec := embeddings.DeserializeFloat32(blob)
		reranked = append(reranked, rankedResult{
			docID: docIDByRowid[rowid],
			score: squaredL2(queryVec, vec),
		})
	}
	if err := frows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(reranked, func(i, j int) bool { return reranked[i].score < reranked[j].score })

	// TEMP DEBUG: show the closest matches by exact squared-L2 distance.
	for i := 0; i < min(5, len(reranked)); i++ {
		slog.Info("[vector-debug] rerank",
			"rank", i,
			"docID", reranked[i].docID,
			"squaredL2", reranked[i].score,
		)
	}

	// Stage 3: apply filters and truncate to topK.
	results := make([]rankedResult, 0, topK)
	for _, r := range reranked {
		if !filters.hasFilters() || passesFilters(db, r.docID, filters) {
			results = append(results, r)
			if len(results) >= topK {
				break
			}
		}
	}
	return results, nil
}

// squaredL2 returns the squared Euclidean distance between two vectors.
// Mismatched lengths yield +Inf so the pair sorts last rather than panicking.
func squaredL2(a, b []float32) float64 {
	if len(a) != len(b) {
		return math.Inf(1)
	}
	var sum float64
	for i := range a {
		d := float64(a[i]) - float64(b[i])
		sum += d * d
	}
	return sum
}

// vectorNorm returns the L2 norm of v (TEMPORARY debug helper).
func vectorNorm(v []float32) float64 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	return math.Sqrt(sum)
}

// escapeFTSQuery wraps each token in double quotes for safe FTS5 queries.
func escapeFTSQuery(query string) string {
	tokens := strings.Fields(query)
	if len(tokens) == 0 {
		return ""
	}
	quoted := make([]string, len(tokens))
	for i, t := range tokens {
		quoted[i] = `"` + t + `"`
	}
	return strings.Join(quoted, " ")
}

// ftsSearch runs full-text search via FTS5.
func ftsSearch(db *sql.DB, queryText string, topK int, filters *Filters) ([]rankedResult, error) {
	safeQuery := escapeFTSQuery(queryText)
	if safeQuery == "" {
		return nil, nil
	}

	candidateLimit := topK * 3
	if filters.hasFilters() {
		candidateLimit = topK * 50
	}

	rows, err := db.Query(
		`SELECT rowid, rank
		 FROM documents_fts
		 WHERE documents_fts MATCH ?
		 ORDER BY rank
		 LIMIT ?`,
		safeQuery, candidateLimit,
	)
	if err != nil {
		slog.Warn("FTS query failed", "query", safeQuery, "err", err)
		return nil, nil // Non-fatal: return empty results.
	}
	defer rows.Close()

	var results []rankedResult
	for rows.Next() {
		var docID int64
		var rank float64
		if err := rows.Scan(&docID, &rank); err != nil {
			return nil, fmt.Errorf("scan fts result: %w", err)
		}
		if !filters.hasFilters() || passesFilters(db, docID, filters) {
			results = append(results, rankedResult{docID: docID, score: rank})
			if len(results) >= topK {
				break
			}
		}
	}
	return results, rows.Err()
}

// passesFilters checks if a document passes the given filters.
func passesFilters(db *sql.DB, documentID int64, filters *Filters) bool {
	if filters == nil {
		return true
	}

	var metadataStr sql.NullString
	var collectionName, collectionType, sourceType, sourcePath string

	err := db.QueryRow(
		`SELECT d.metadata, c.name, c.collection_type, s.source_type, s.source_path
		 FROM documents d
		 JOIN collections c ON d.collection_id = c.id
		 JOIN sources s ON d.source_id = s.id
		 WHERE d.id = ?`,
		documentID,
	).Scan(&metadataStr, &collectionName, &collectionType, &sourceType, &sourcePath)
	if err != nil {
		return false
	}

	if filters.Collection != "" {
		if collectionTypes[filters.Collection] {
			if collectionType != filters.Collection {
				return false
			}
		} else if collectionName != filters.Collection {
			return false
		}
	}

	if filters.SourceType != "" && sourceType != filters.SourceType {
		return false
	}

	if filters.Path != "" && !strings.Contains(strings.ToLower(sourcePath), strings.ToLower(filters.Path)) {
		return false
	}

	needsMetadata := filters.Sender != "" || filters.Author != "" ||
		filters.DateFrom != "" || filters.DateTo != "" || len(filters.MetadataFilters) > 0
	if !needsMetadata {
		return true
	}

	var metadata map[string]any
	if metadataStr.Valid {
		_ = json.Unmarshal([]byte(metadataStr.String), &metadata)
	}
	if metadata == nil {
		metadata = make(map[string]any)
	}

	if filters.Sender != "" {
		sender, _ := metadata["sender"].(string)
		if !strings.Contains(strings.ToLower(sender), strings.ToLower(filters.Sender)) {
			return false
		}
	}

	if filters.Author != "" {
		authorLower := strings.ToLower(filters.Author)
		authors, _ := metadata["authors"].([]any)
		found := false
		for _, a := range authors {
			if s, ok := a.(string); ok && strings.Contains(strings.ToLower(s), authorLower) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	docDate, _ := metadata["date"].(string)
	if filters.DateFrom != "" && docDate != "" && docDate < filters.DateFrom {
		return false
	}
	if filters.DateTo != "" && docDate != "" && docDate > filters.DateTo {
		return false
	}

	for key, filterVal := range filters.MetadataFilters {
		raw, exists := metadata[key]
		if !exists {
			return false
		}
		filterLower := strings.ToLower(filterVal)
		switch v := raw.(type) {
		case string:
			if !strings.Contains(strings.ToLower(v), filterLower) {
				return false
			}
		case []any:
			found := false
			for _, elem := range v {
				if s, ok := elem.(string); ok && strings.Contains(strings.ToLower(s), filterLower) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		default:
			if fmt.Sprintf("%v", v) != filterVal {
				return false
			}
		}
	}
	return true
}

// RRFMerge merges two ranked lists using Reciprocal Rank Fusion, then
// normalizes the fused scores to [0, 1] so the best possible hit (rank 0 in
// both lists) reads as ~1.0. Ordering is unchanged — only the magnitude the
// frontend displays changes.
func RRFMerge(vecResults, ftsResults []rankedResult, k int, vectorWeight, ftsWeight float64) []rankedResult {
	scores := make(map[int64]float64)

	for rank, r := range vecResults {
		scores[r.docID] += vectorWeight / float64(k+rank+1)
	}
	for rank, r := range ftsResults {
		scores[r.docID] += ftsWeight / float64(k+rank+1)
	}

	// Normalize by the maximum achievable score: rank 0 in both lists.
	denomBase := float64(k + 1)
	if denomBase <= 0 {
		denomBase = 1
	}
	denom := (vectorWeight + ftsWeight) / denomBase
	if denom <= 0 {
		denom = 1
	}

	merged := make([]rankedResult, 0, len(scores))
	for docID, score := range scores {
		merged = append(merged, rankedResult{docID: docID, score: score / denom})
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].score > merged[j].score })
	return merged
}

// fetchResult loads a SearchResult from the database for a document ID.
func fetchResult(db *sql.DB, docID int64, score float64) (*SearchResult, error) {
	var content, collectionName, sourcePath, sourceType string
	var title sql.NullString
	var metadataStr sql.NullString

	err := db.QueryRow(
		`SELECT d.content, d.title, d.metadata,
		        c.name, s.source_path, s.source_type
		 FROM documents d
		 JOIN collections c ON d.collection_id = c.id
		 JOIN sources s ON d.source_id = s.id
		 WHERE d.id = ?`,
		docID,
	).Scan(&content, &title, &metadataStr, &collectionName, &sourcePath, &sourceType)
	if err != nil {
		return nil, err
	}

	titleText := ""
	if title.Valid {
		titleText = title.String
	}

	var metadata map[string]any
	if metadataStr.Valid && metadataStr.String != "" {
		_ = json.Unmarshal([]byte(metadataStr.String), &metadata)
	}
	if metadata == nil {
		metadata = map[string]any{}
	}

	return &SearchResult{
		Content:    content,
		Title:      titleText,
		Metadata:   metadata,
		Score:      score,
		Collection: collectionName,
		SourcePath: sourcePath,
		SourceType: sourceType,
	}, nil
}
