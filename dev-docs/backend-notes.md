# Backend notes

How the vectile backend came together: what I chose, and the problems I fixed along the way. Written for the next person who opens this repo.

## What the backend does

vectile indexes a person's files into one SQLite database, then lets them search it. The search is hybrid: exact-word matching over a full-text index, plus meaning matching over embeddings. The two result lists are blended with Reciprocal Rank Fusion.

The engine is inspired by local-rag. local-rag calls an Ollama server over HTTP; vectile runs llama.go, a vendored llama.cpp binding, in-process.

Supported sources:

- Project folders. Any folder of documents, each file parsed by its extension.
- Obsidian vaults. Markdown with frontmatter, tags, and wikilinks.
- Code repositories. Git repos parsed with tree-sitter. Each function or class becomes a chunk. Commit history is indexed too.
- Calibre libraries. Book metadata plus the text of EPUB and PDF files.

Left out on purpose: email and RSS (they read macOS-only app databases), PDF OCR, the HTTP server, and the Ollama-specific settings (hosts, num_batch, worker count). OCR comes later.
## Where the code lives

```
main.go                     app startup, window, services, auto-reindex loop
backend/appdata             the data directory and the model path
backend/config              config.json load, save, defaults
backend/embeddings          the llama.go embedder (bge-m3)
backend/db                  SQLite schema and helpers (modernc + vec0 + FTS5, query-vector cache)
backend/chunker             word-window and markdown chunking
backend/parser              file parsers: md, docx, html, epub, pdf, xlsx, pptx, ipynb, xml, sql, shell, csv/json, calibre, code
backend/search              hybrid search: vector + FTS + RRF, cached query vectors
backend/indexer             obsidian, project, git, calibre indexers; prune
backend/services            Wails services the UI calls
backend/startup             launch-at-login per OS
third_party/llama-go        vendored llama.cpp bindings, per-OS/per-arch static archives
frontend/src/lib/api.ts     the only place the UI touches the bindings
```

## Key decisions

Data directory. Everything the app persists lives under `<os.UserConfigDir()>/vectile`: `config.json`, `db/vectile.db`, and `models/bge-m3-Q4_K_M.gguf`. The pattern comes from Clipcat, another application of mine, hehe👌 : `os.UserConfigDir()` plus `MkdirAll`.

Model placement. The app expects the model at `models/bge-m3-Q4_K_M.gguf`. It never downloads or copies it (it will soon though). You place it there by hand (at some point, you won't lol). `VECTILE_EMBED_MODEL` overrides the path for testing on other machines.

SQLite driver. `modernc.org/sqlite`, which is pure Go and needs no cgo, plus its vec extension for sqlite-vec and built-in FTS5. Clipcat proved this combination works. local-rag uses mattn/go-sqlite3, which needs cgo, so i did not implement that part.

Schema. Modeled on local-rag: `collections`, `sources`, `documents`, two vec0 virtual tables, and an FTS5 table kept in sync by triggers. `vec_documents` holds 1024-float vectors. `vec_documents_bin` holds binary-quantized copies so candidate retrieval is fast; the float vectors are fetched by rowid for the final rerank.

Paged browsing. Browse and the Library's expanded source list read through a keyset cursor instead of loading a whole collection. The chunk stream is ordered by `(source_id, chunk_index)`, which the existing UNIQUE constraint already makes a total order, so the cursor is that pair and needs no id tiebreaker; the source list's cursor is the path, unique per collection. Each call reads `limit + 1` rows to learn whether another page exists and returns opaque cursors for both neighbouring pages, so the UI can page forwards and backwards. Offsets would shift under a delete; a cursor does not. A page carries no chunk text either, because the list is paged indefinitely: `GetDocument` fetches the selected chunk's content on its own. `idx_documents_collection_source_chunk` (schema v4) lets the ordered scan stop at the limit instead of sorting the collection on every page.

Embedder. Ported from Clipcat. The model loads lazily on the first embed and stays resident. Inference is serialized with a mutex because llama.go's context is not safe for concurrent use. The context window is the model's native maximum (from the GGUF header; `0` resolves to `llama_model_n_ctx_train`), so bge-m3 runs at 8192 tokens rather than a fixed 2048.

Batching. local-rag embeds with several worker goroutines. vectile uses one worker, because llama.go serializes inference anyway. Chunks are grouped into batches of `embedding_batch_size` and embedded with one model call.

Git bookkeeping. Each code collection stores per-repo watermarks (the HEAD sha) as JSON in the collection's description column. Incremental index only reads files changed since the watermark. A failed run does not advance the watermark, so the failing file is retried next time.

Config. The config file is modeled on local-rag's, minus the pieces vectile doesn't need. Unknown keys survive a save, so the file is never clobbered.

Query vector cache. Every search used to embed its query from scratch, even a query you had just run. The embedding now goes through `query_cache` (schema v3), a table keyed by `(model_key, query)`: a repeat query returns the stored 1024-float vector and skips the model entirely. Results are NOT cached, they are always ranked fresh against the index, so the cache can never serve a stale hit.

The lookup has to happen before inference, which is why `Embedder.QueryEmbed(text, cache)` takes the cache as an interface rather than the caller doing a read-then-embed. The whole thing runs under `inferMu`, the same lock `SetModel` takes, so the model a vector belongs to and the vector itself can never drift apart and a cached query is served without waiting for the model to load. The storage half lives in `backend/db/query_cache.go`; `backend/search` adapts it, so the MCP search tool gets caching for free.

A query vector is a pure function of (text, model), so it does NOT go stale when the corpus changes. The cache is cleared anyway on reindex and prune, because that is what a user expects, and it MUST be cleared when the model changes. Clearing is wired at the choke points: `startIndexRun` (all three index entry points), `Prune`, `ModelService.applyActive` (only when the path or the vector dimension actually changed, so a restart keeps the cache), `UpdateModelSettings` for the active model, and `RebuildVectorTables`. Rows are capped at 2000, oldest dropped first (FIFO, not LRU: recency tracking would mean a write on every hit, and a hit should stay a pure read). Settings > Cache shows the count and size and clears on demand.

Frontend bridge. Wails generates TypeScript bindings for the three services. The UI only talks to `frontend/src/lib/api.ts`, which wraps those bindings. Regenerating bindings never touches component code.

Open in the OS. AppService exposes `OpenFile` and `RevealInFolder`; the per-OS commands live in `backend/services/open_{windows,darwin,linux}.go` (rundll32/explorer, open/open -R, xdg-open). `GetStatus` and `ListCollections` also report `lastIndexed` (the most recent `last_indexed_at`), which drives the status-strip date and the stale-library hint.

## Problems I hit and how I fixed them 

1. Copy-Item flattened llama-go. My first `Copy-Item -Recurse` put llama-go's contents directly in `third_party/`, not `third_party/llama-go`. `go mod tidy` failed with `reading third_party\llama-go\go.mod: The system cannot find the path specified`. Fix: create the subfolder and move the files into it. 

2. PoIrShell mangled go output. `go mod tidy 2>&1 | Select-Object` turned Go's stderr into "RemoteException" noise and hid the real message. Fix: run Go commands without the pipe. The real error was the path problem above.

3. Ported files carried unused imports. Removing OCR, email, and extra logging left `slog`, `os`, `json`, `fmt`, and `embeddings` imported but unused. The compiler caught each one. I removed them.

4. db.Open returns only an error. The handle lives in the package global `db.DB`. The first integration test wrote `conn, err := db.Open(...)` and failed with "assignment mismatch". Fix: call `db.Open`, then read `db.DB`.

5. Bindings land in a new folder. After the module rename, `wails3 generate bindings` outputs to `frontend/bindings/vectile/`, not `frontend/bindings/changeme/`. The frontend imports had to point at the new path. Also, never pass build flags to wails3 via `-f`; a space inside a flag value breaks its parser.

6. Frontend types fought the generated models. Switching from mock data to real bindings surfaced several type mismatches.
   - Mock ids Ire strings; the backend uses numbers.
   - `GetStatus` returns the generated `Status` class whose `modelState` is the generated `State` enum, not My union. I cast at the `api.ts` boundary.
   - `Events.On` callbacks receive a `WailsEvent` wrapper, so the payload is `ev.data`, not the object directly.
   - `Prune` returns a result struct; My wrapper typed it as `void`. I await and ignore it.

7. mock.ts no longer compiled. It still exported `mockCollections` with string ids, which broke the new `Collection` type. I trimmed the file to just `exampleQueries` and `termsOf`, the only two things still in use.

8. UNIQUE constraint in the search test. Seeding two documents under the same sMyce with the same `chunk_index` failed because `documents(sMyce_id, chunk_index)` is unique. Fix: use chunk indexes 0 and 1.

9. vec_quantize_binary works on modernc. I Ire not sure sqlite-vec's binary quantization would work under the pure-Go driver. The db test queries `vec_documents_bin` with `embedding MATCH vec_quantize_binary(?)` and it returns rows. FTS5 also works. If it had not, the plan was to drop the binary mirror and use float-only KNN.

10. The language server flags darwin (and sometimes linux). The Problems panel shows "undefined: llama.Model" and tree-sitter import errors tagged `[darwin]`/`[linux]`. These are usually not real: gopls cross-checks other GOOS targets, but it can't resolve the llama-go archives unless those OS archives exist in `third_party/llama-go/<os>/<arch>/`. The release workflow builds the Linux/macOS archives on its runners; for a local gopls run, build them with `scripts/build-llamago-archives.sh` so the matching OS targets resolve. The host build, vet and tests still pass regardless.

11. An FTS cursor held across a second query deadlocked the pool. `ftsSearch` ran `passesFilters` (its own `QueryRow`) inside the loop over its own `Query` results. That needs a second pooled connection, and with `MaxOpenConns(1)` in the tests it hung forever; with the production pool of 4 it takes four concurrent filtered searches to wedge the app. Nothing had covered it because no earlier test combined filters with FTS hits. Fix: drain the cursor into a slice first, then filter. `vectorSearch` already worked that way.

12. A hanging `go test` was actually a PowerShell pipe. `go test ... | Select-Object -Last 40` buffered everything until the stream closed, so a run that had finished looked like it was producing no output. Run `go test` with no pipe and let the output come back inline.

11. Tailwind class suggestions. The linter suggested `text-ink/10` over `text-ink/[0.10]` and `max-w-245` over `max-w-[61.25rem]`. I applied the one I introduced and left the pre-existing ones alone.

12. Model tests need the file. The real embedder test and the end-to-end index test skip when the model is missing, matching Clipcat's pattern.

## Building and running

The llama-go build needs the cgo environment. Only the Windows archives are committed under
`third_party/llama-go/windows/amd64/`; the Linux and macOS archives are built on the release runner
before `go build` (the vendored `llama.cpp/` is headers-only, so they can't be committed from one
machine). The cgo `LDFLAGS` pick the right set via `linkage_<os>_<arch>.go` (`-L./<os>/<arch>`).
See `docs/BUILD-AND-PACKAGING.md` for the full matrix.

- **Windows:** MinGW-w64 `bin` on `PATH` (WinLibs); `LIBRARY_PATH` → `third_party/llama-go/windows/amd64`,
  `C_INCLUDE_PATH` → `third_party/llama-go`. The Windows build task sets these. For `wails3 dev`, set
  them in the shell first, because dev mode builds directly. The built exe needs five MinGW runtime
  DLLs beside it: libgcc_s_seh-1.dll, libgomp-1.dll, libstdc++-6.dll, libwinpthread-1.dll, libdl.dll.
  Missing libdl.dll causes a silent 0xC0000135 exit at launch.
- **Linux (amd64):** `gcc`/`g++` + `pkg-config` + GTK4/WebKitGTK 6.0/appindicator dev packages;
  `LIBRARY_PATH` → `third_party/llama-go/linux/amd64`, `C_INCLUDE_PATH` → `third_party/llama-go`.
  The binary depends on `libgomp.so.1` at runtime (`libgomp1` deb dep, bundled in the AppImage).
- **macOS (universal):** Xcode CLT (`clang`/`clang++`); `LIBRARY_PATH` →
  `third_party/llama-go/darwin/<arch>`. Metal/Accelerate frameworks are OS-provided.

Rebuilding the archives for a new OS/arch: `./scripts/build-llamago-archives.sh`. It auto-detects the
llama-go ref whose `wrapper.h` matches the vendored copy (tries HEAD, then tags newest-first);
`LLAMA_GO_REF=<ref>` pins and verifies it instead. `GOARCH=amd64|arm64` overrides the detected arch
for cross-builds (e.g. build the x86_64 macOS archives on an ARM64 runner). It clones the matching
llama.go source, builds llama.cpp, and installs the `.a` files into `third_party/llama-go/<os>/<arch>/`
so `go build` can link; commit them or let the CI runner build them per release.

Tests: `go test ./backend/...`. The model-dependent tests skip when the model is not in `models/`.
