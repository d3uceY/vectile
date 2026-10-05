# Reciprocal rank fusion (RRF)

How vectile turns two separate search result lists into one ranked list. If you have not opened `backend/search/search.go` yet, read this first; it explains the idea before the code, then points at the exact functions.

## The problem this solves

A search in vectile runs twice.

1. **Full-text search (FTS5).** It matches the words you typed. Precise, but blind to meaning.
2. **Vector search.** It embeds your query into 1024 numbers and finds stored chunks whose vectors point in the same direction. It catches meaning, but it is fuzzy.

Both searches return a list of chunks, best first. The two lists overlap but are not identical, and you can't just add the two scores together:

- FTS5 gives each chunk a BM25 rank, a small negative number that only means something relative to the other FTS hits.
- Vector search gives each chunk a squared L2 distance, where **smaller is better** and the scale depends on the data.

An FTS score of `-4.2` and a vector distance of `183.7` are not comparable. There is no shared unit.

RRF sidesteps that by ignoring the scores entirely and using only the **position** of each chunk in each list. Positions are always comparable, because every list is just "1st, 2nd, 3rd...".

## The formula

For each chunk, RRF adds up one small contribution per list it appears in:

$$
\text{score}(d) = \frac{w_{vec}}{k + \text{rank}_{vec} + 1} + \frac{w_{fts}}{k + \text{rank}_{fts} + 1}
$$

The parts:

- **rank** is the chunk's zero-based position in that list. First place is rank 0, second is rank 1, and so on. The `+1` is there because the code uses Go slice indexes, which start at 0.
- **k** is a smoothing constant. It decides how much early positions matter more than later ones. vectile's default is 60.
- **w_vec** and **w_fts** are the weights of the two lists. vectile's defaults are 0.7 and 0.3, so the vector list counts for more.

Every chunk that appears in a list gets a contribution. A chunk that appears in both lists gets two contributions added together. A chunk in only one list still keeps whatever it earned there.

The shape of `1 / (k + rank + 1)` is the whole trick: because k is large, the score slides down gently instead of dropping off a cliff. Rank 0 and rank 1 are worth nearly the same; only a big gap in rank creates a big gap in score.

## A worked example

Defaults: k = 60, w_vec = 0.7, w_fts = 0.3.

| Chunk | Vector rank | FTS rank | Vector term | FTS term | Raw sum |
| --- | --- | --- | --- | --- | --- |
| A | 0 | 4 | 0.7 / 61 = 0.011475 | 0.3 / 65 = 0.004615 | 0.016090 |
| B | 2 | not found | 0.7 / 63 = 0.011111 | 0 | 0.011111 |
| C | not found | 1 | 0 | 0.3 / 62 = 0.004839 | 0.004839 |

Chunk A wins: it is first in the vector list and still near the top in the text list, so it collects a little from both. Chunk B is only in the vector list. Chunk C is only in the text list, and at 0.3 weight its contribution is small.

That is the behaviour you want from fusion. A chunk that both searches agree on should beat a chunk that only one search likes.

## The normalisation step

The raw scores above are tiny (around 0.016) and would look meaningless in the UI. So after merging, `RRFMerge` divides every score by the largest score it is theoretically possible to get: rank 0 in both lists.

$$
\text{denom} = \frac{w_{vec} + w_{fts}}{k + 1}
$$

With the defaults that is `1.0 / 61 = 0.016393`. Divide the table above by it and you get the number the UI shows:

- A: 0.016090 / 0.016393 = **0.98** → displayed as `98%`
- B: 0.011111 / 0.016393 = **0.68** → `68%`
- C: 0.004839 / 0.016393 = **0.30** → `30%`

Only a chunk that is rank 0 in *both* lists reaches 100%. Nothing else can, which is why scores cluster well below 100% in practice. The normalisation changes the magnitude only; the ordering is already settled before it runs.

Watch out for one detail: the ranks are positions inside each search's **own short list** (the top-k the backend asked that search for), not positions in the whole library. If `top_k` is 10, the vector list is 10 chunks long and the FTS list is 10 chunks long, and the ranks run 0 to 9. A chunk at position 500 in the raw index but position 3 in the vector shortlist is rank 3 for RRF.

## Where the code lives

All in `backend/search/search.go`.

| What | Where |
| --- | --- |
| The fusion itself | `RRFMerge`, line 496. Two loops over the lists, a map of docID to score, a sort. |
| Where it is called | `Search`, line 112, after both searches and their filters have run. |
| Vector list | `vectorSearch`, line 205. Binary-quantised candidate pool, float rerank, filters, truncated to top-k. Smaller distance is better. |
| Text list | `ftsSearch`, line 315. FTS5 BM25, filters, truncated to top-k. Lower BM25 rank is better. |
| Reading the chunk back | `fetchResult`, line 525. RRF only returns ids and scores; this joins the chunk's content, title, collection, and path onto it. |

The defaults live in `backend/config/config.go`: the `SearchDefaults` struct at line 18 (`TopK`, `RRFK`, `VectorWeight`, `FTSWeight`) and their values at line 286: top-k 10, k 60, weights 0.7 and 0.3.

## Where each setting is editable

- Settings > Search has **RRF constant (k)** and the two weights (`frontend/src/components/settings/sections/SearchSection.tsx`, around line 35).
- Their bounds are in `frontend/src/components/settings/bounds.ts`: k is clamped to 1..200 (line 23), each weight to 0..1.
- The displayed score is `SearchResult.Score` (`backend/search/search.go`, the struct at the top of the file). The card prints `Math.round(score * 100)%` and has a toggle to show the plain rank instead: `frontend/src/components/search/ResultCard.tsx` line 52, with the toggle stored in `frontend/src/lib/store.tsx` around line 164.

## What k actually does

Small k makes winning the top spot pay a lot: at k = 1, rank 0 scores `w/2` and rank 1 scores `w/3`, a 50% gap. Large k flattens that: at k = 60, rank 0 scores `w/61` and rank 1 `w/62`, about a 1.6% gap. So k controls how much you trust a single search's confidence in a chunk's exact position. The default 60 is the value from the original RRF paper and the usual choice; there is rarely a reason to move it.

The weights do something different. They say which kind of match you care about more. Setting `fts_weight` to 0 makes search purely semantic; setting it to 1 makes it purely keyword. The two are independent, so they do not have to add up to 1.

## Things worth knowing before you change this code

- **A chunk in both lists always benefits.** With 0.7 and 0.3, one list alone gives at most 0.7 or 0.3 of the theoretical max, while agreement from both lists can approach 1.0. That is deliberate.
- **Ties are possible.** Two chunks can land on the same score (for example, both rank 1 in the same single list). The sort is not stable, so the order between exact equals is arbitrary. It does not matter, because equal scores mean the searches thought they were equally good.
- **A doc ID with no row is dropped.** `Search` calls `fetchResult` for each merged id and silently skips any that fails. That should not happen, since the ids came from the database a moment earlier, but it keeps a stale id from failing the whole search.
- **The vector side can fail without failing the search.** If no model is loaded, `Search` logs a warning and runs with an empty vector list, so RRF just sees the FTS list. See `search_test.go`, `TestSearchFallsBackToFTSWithoutModel`, and `TestHybridSearchRanksFTSAndVector` for the both-lists case.
- **Ranks come from already-filtered lists.** Filters are applied inside each search before RRF runs, so a chunk hidden by a filter never gets a rank and never appears in the fused list.
