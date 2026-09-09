# vectile Semantic Test — Query Sets

Test corpus lives in [`text documents/`](./text%20documents/) (17 files, `01`–`17`).

The **target** column shows which file(s) a *good* semantic result should surface.

---

## 🟢 Easy — literal keyword overlap (should hit even with plain full-text search)

These verify the index itself works before you judge semantics.

| # | Query | Target |
|---|-------|--------|
| E1 | `tomato sauce` | 01 |
| E2 | `banana bread` | 02 |
| E3 | `sourdough starter` | 03 |
| E4 | `vegetable soup` | 04 |
| E5 | `sleep` | 12 |
| E6 | `git` | 14 |
| E7 | `Rust command line tools` | 15 |
| E8 | `science fiction reading list` | 16 |

---

## 🟡 Medium — paraphrase / synonym, low shared keywords

Mostly paraphrase, meant to have little literal overlap — but a word-boundary audit shows **several share content words with their targets**: `spotty bananas` → 02, `flour/water/tangy/bread` → 03, `short/bursts` → 05, `cold/weather` → 04, `carry` → 08, `meat/muscle` → 11, `desk` → 13, `history/working/alone` → 14 (M6/M7 are the only truly keyword-free rows). Treat these as "should win even with a keyword bridge." For queries with a keyword that doesn't exist **anywhere**, use the ⚫️ Black set below.

| # | Query | Target |
|---|-------|--------|
| M1 | `quick bread made from spotty bananas` | 02 |
| M2 | `fermented flour and water for tangy bread` | 03 |
| M3 | `high-intensity cardio in short bursts` | 05 |
| M4 | `warm stew for cold weather` | 04 |
| M5 | `what to carry when flying` | 08 |
| M6 | `planning a trip into the mountains` | 07 |
| M7 | `growing food in small outdoor spaces` | 10 |
| M8 | `meat-free meals that build muscle` | 11 |
| M9 | `making the desk stop hurting` | 13 |
| M10 | `keeping code history when working alone` | 14 |

---

## 🔴 Hard — abstract / oblique, needs topical inference

The query shares almost nothing lexically; the embedding has to infer the *topic*. Some are genuinely ambiguous — that's intentional. Watch **which** doc surfaces and whether it's plausibly the best of the set.

| # | Query | Target (or what to look for) |
|---|-------|------------------------------|
| H1 | `what should I bake this weekend for guests` | 02 or 03 (infer baking + weekend) |
| H2 | `preparing for a night away from home` | 08 or 09 (infer travel) |
| H3 | `healthy routines that start the day right` | 05 / 12 (infer morning + health) |
| H4 | `turning a small balcony into a pantry` | 10 (infer growing food in small space) |
| H5 | `cheap ways to get more energy during the day` | 11 or 12 |
| H6 | `escaping the city to clear my head` | 07 (ridge, quiet, no signal) |
| H7 | `fun things to do together on a Friday` | 17 |
| H8 | `quiet hobbies that keep my mind sharp` | 16 or 14 |

---

## ⚫️ Black — no word of the query exists in the target (regex-verifiable)

Same idea as Medium but *impossible to fake with a keyword*. These rows are stricter than the old "one keyword is missing" design: **none of the query's words appear in the target file**. A word-boundary check covers every word of the query, so a plain FTS query cannot rank the target at all — if the target surfaces, only the embedding can be doing the work.

Two caveats shaped the rows:

- vectile's FTS5 table indexes the document **title** as well as the body, and for these `.txt` files the title is the *filename*. A query word is only safe if it is absent from the file body **and** from its filename — so topic words that live only in the filename (e.g. `italian`, `banana`, `movie`) are avoided too.
- Each row keeps a **Missing keyword** — the salient word a keyword searcher would naturally reach for — which is additionally absent from the *entire* corpus (all 17 files: 0 hits). The **Verify** regex spot-checks that one word against the file body; it must match nothing.

The **Verify** regex is the quick per-file spot-check. A ready-to-run check that tests *every* query word against both the body and the filename title for all 17 at once lives in [`verify-black-queries.ps1`](./verify-black-queries.ps1).

| # | Query | Target | Missing keyword | Verify (against target file) |
|---|-------|--------|-----------------|------------------------------|
| B1 | `homemade marinara from ripe garden fruit` | 01 | `marinara` | `(?i)\bmarinara\b` |
| B2 | `ripe yellow fruit quick breakfast cake` | 02 | `cake` | `(?i)\bcake\b` |
| B3 | `natural yeast raised from fermented grain culture` | 03 | `yeast` | `(?i)\byeast\b` |
| B4 | `cozy autumn chowder with root vegetables` | 04 | `chowder` | `(?i)\bchowder\b` |
| B5 | `HIIT sessions to raise your stamina` | 05 | `HIIT` | `(?i)\bhiit\b` |
| B6 | `marathon fuel hydration plan` | 06 | `marathon` | `(?i)\bmarathon\b` |
| B7 | `mountain trail with sweeping scenery` | 07 | `mountain` | `(?i)\bmountain\b` |
| B8 | `airport hand luggage essentials` | 08 | `airport` | `(?i)\bairport\b` |
| B9 | `overnight camping kit that stays light` | 09 | `camping` | `(?i)\bcamping\b` |
| B10 | `homegrown salsa from tiny patio planters` | 10 | `salsa` | `(?i)\bsalsa\b` |
| B11 | `vegan athletes diet strength building` | 11 | `vegan` | `(?i)\bvegan\b` |
| B12 | `evening wind-down ritual for deep rest` | 12 | `ritual` | `(?i)\britual\b` |
| B13 | `office posture tips for long days on screen` | 13 | `posture` | `(?i)\bposture\b` |
| B14 | `version control for personal projects` | 14 | `version` | `(?i)\bversion\b` |
| B15 | `systems programming for terminal utilities` | 15 | `terminal` | `(?i)\bterminal\b` |
| B16 | `dystopian novels envisioning future society` | 16 | `dystopian` | `(?i)\bdystopian\b` |
| B17 | `cartoon marathon with family` | 17 | `cartoon` | `(?i)\bcartoon\b` |

**Reading the results:** if the target surfaces in the top few despite *no* query word existing anywhere in it (body or filename title), the embedding genuinely inferred the topic — the strongest proof the vector side works. A wrong doc (e.g. B6 landing on 05) means the model is drifting on topic.

---

## How to read the results

- **Easy** should be near-perfect. If these miss, the index itself is broken, not the embeddings.
- **Medium** is the real semantic test. Check that the target doc lands in the **top 3**. If it only shows up in Easy queries, the vector side isn't pulling its weight.
- **Hard** is about *relative* relevance: even a "wrong" doc is informative. E.g. for `turning a small balcony into a pantry`, a hit on 10 is ideal; a hit on 04 (soup/vegetables) is defensible; a hit on 15 (Rust) means the embedding is failing you.
- **Black** is the anti-keyword trap: *every* word of the query returns 0 hits in the target (body + filename title), so if the target still surfaces near the top, the vector side — not FTS — did the work.

## Two tips for isolating the vector side

1. vectile's search is hybrid (FTS + vector + RRF). To see what the *embeddings alone* find, drop `fts_weight` to `0` and set `vector_weight` to `1` in Settings → Search, then re-run the Medium/Hard sets. That removes the keyword crutch entirely.
2. Compare each query's top result at `vector_weight=1` vs. the default hybrid — the difference is exactly how much the semantic layer is contributing.
