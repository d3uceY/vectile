package search

import (
	"database/sql"

	"vectile/backend/config"
)

// Related returns the documents nearest to a seed embedding, excluding the
// seed chunk itself. It reuses the exact binary-pool plus float-rerank path
// that search uses, so the two never drift apart.
//
// Scores are squared L2 distances: lower is closer, unlike a fused search
// score. The seed is excluded by id, so the list is what a client wants to
// read next.
func Related(conn *sql.DB, seedVec []float32, excludeDocID int64, topK int, filters Filters, sd config.SearchDefaults) ([]SearchResult, error) {
	if topK <= 0 {
		topK = sd.TopK
	}
	if topK <= 0 {
		topK = 10
	}

	// Over-fetch by one so the seed itself cannot consume a result slot.
	ranked, err := vectorSearch(conn, seedVec, topK+1, &filters)
	if err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, topK)
	for _, r := range ranked {
		if r.docID == excludeDocID {
			continue
		}
		res, err := fetchResult(conn, r.docID, r.score)
		if err != nil {
			continue
		}
		results = append(results, *res)
		if len(results) >= topK {
			break
		}
	}
	return results, nil
}
