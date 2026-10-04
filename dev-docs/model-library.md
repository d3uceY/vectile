# Model library

How vectile's model system works: importing a .gguf, per-model settings, and
the active-model switch. Written for the next person who opens this repo, in
the same spirit as `backend-notes.md`.

## What it does

vectile used to expect one model at `models/bge-m3-Q4_K_M.gguf` and that was
it. Now you can:

- Import a `.gguf` embedding model from Settings. The app opens a native file
  dialog, copies the file into the `models/` folder, and registers it.
- Pick the active model from a dropdown. Any `.gguf` sitting in `models/`
  shows up there even if you never imported it through the UI.
- Give each model its own settings, stored on its row: embedding dimension
  and native context window (both auto-discovered from the GGUF header), batch
  size, and CPU threads. Those apply when the model is active.
- Switch models freely. If the new model's embedding dimension differs from
  the one the vector tables were built at, the switch waits for you to confirm,
  because changing dimensions wipes every stored embedding and forces a full
  re-index. Collections that lost their embeddings show "needs reindex" until
  you index them again.
- Delete a model. You can't delete the active one. Removing a model also
  deletes its file inside `models/`, so the next folder scan can't resurrect
  it.

## Where the code lives

```
backend/db/models.go               models table helpers, vector-dim tracking,
                                   RebuildVectorTables
backend/db/schema.go               the models table (schema v2), vecTablesDDL
backend/embeddings/gguf.go         ReadMetadata: dims + context from the header
backend/embeddings/embedder.go     SetModel(path, ctx, threads), per-model ctx
                                   and threads
backend/services/model_service.go  ModelService: import, list, switch, delete,
                                   settings, startup apply
backend/services/app.go            GetStatus model name, ListCollections
                                   needsReindex
backend/config/config.go           active_model field
main.go                            registers the service, ApplyActiveModel
                                   after db.Open
frontend/src/lib/types.ts          ModelInfo, SetActiveResult
frontend/src/lib/api.ts            wrappers around the ModelService bindings
frontend/src/lib/store.tsx         models signal, setActiveModel flow,
                                   model:changed listener
frontend/src/components/settings/SettingsView.tsx   the Model section
frontend/src/components/library/LibraryView.tsx     needs-reindex badge
frontend/src/components/index/IndexView.tsx         needs-reindex chip
frontend/bindings/.../modelservice.ts               generated bindings
scripts/vectile-stub.mjs           dev-stub demo models + method IDs
```

## How the pieces connect

### Startup

`main.go` picks the initial model path from `cfg.ActiveModel`, or the default
`appdata.ModelPath()` if that's empty, and creates the embedder. The path has
to come from config rather than the DB, because the embedder is created before
the DB is open.

After `db.Open`, it runs `ModelService.ApplyActiveModel()`. That does three
things in order:

1. Reconciles the `models/` folder into the table (scan `*.gguf`, add missing
   rows, delete rows whose file is gone).
2. Finds the active model: the row with `is_active = 1`, falling back to
   `cfg.ActiveModel`. If the configured model vanished, it clears the config
   field and points the embedder at the default.
3. Loads that model into the embedder with its settings. If the model's
   dimension doesn't match `meta.vector_dim`, it rebuilds the vector tables
   first.

### Listing and the folder scan

`ListModels` starts by reconciling, because the `models/` folder is the source
of truth. Every call scans the folder, upserts any `.gguf` that isn't
registered, and deletes any row whose file no longer exists. If the deleted
row was the active model, it clears `is_active` and `cfg.ActiveModel` so the
app falls back to the default instead of pointing at a ghost.

That reconcile is why imports and hand-dropped files behave the same: both end
up as rows with a path inside `models/`. The scan stores the native context
window and the default batch size (32) on the row, and `UpsertModel`'s
`ON CONFLICT` also backfills any row still holding `0` for either value, so
rows created before the batch-size default was fixed repair themselves on the
next scan, without clobbering settings the user tuned.

### Importing

The Settings button calls `pickModelFile` (native dialog, filtered to
`.gguf`), then `ImportModel`. The backend checks the extension, reads the GGUF
header for the dimension and native context window, copies the file into
`models/` with a collision-safe name, and upserts a row. It does not
auto-activate. You pick it from the dropdown.

The copy, rather than referencing the file in place, is deliberate: the app
owns a copy, so the model survives the original being moved or deleted.

### Switching models

The dropdown calls `store.setActiveModel(path)` which hits
`SetActiveModel(path, force=false)`. The backend looks up the target row and
compares its dimension against `GetVectorDim`, which reads `meta.vector_dim`,
the dimension the vector tables were built at.

- Same dimension: it applies immediately. The embedder reloads with the new
  model's settings, and existing embeddings stay valid.
- Different dimension, not forced: it returns `needsRebuild = true` and
  applies nothing. The frontend shows a confirm dialog. On confirm it calls
  `SetActiveModel(path, true)`, which rebuilds the tables and then applies.

Applying a model (`applyActive`) sets `is_active`, calls
`Embedder.SetModel(path, ctx, threads)`, mirrors the model's batch size into
`cfg.EmbeddingBatchSize`, writes `cfg.ActiveModel`, and emits `model:changed`.
The frontend listens for that event and refreshes status, collections, and the
models list, so the sidebar plate, status pill, dropdown, and badges update
without a restart.

### The dimension change path

`RebuildVectorTables(dim)` drops `vec_documents_bin` and `vec_documents`,
recreates both at the new dimension, deletes the `binary_backfill_done` meta
flag, and records `meta.vector_dim`. Dropping the binary mirror first matters:
its rowids align with the float table.

That delete is destructive. Every embedding is gone, so any collection that
has documents but no rows in `vec_documents` now reports `needsReindex`.
`ListCollections` computes that per collection with an EXISTS / NOT EXISTS
pair, no stored column. Indexing a collection re-embeds it at the new
dimension, and once its documents have vectors again the flag clears on its
own.

### Deleting a model

`DeleteModel` refuses to remove the active model (the backend returns an
error and the UI disables the button). Otherwise it deletes the row and, when
the file lives inside `models/`, the file too. The file removal matters:
without it, the next folder scan would re-add the model you just deleted.

### Per-model settings

The indexer reads `cfg.EmbeddingBatchSize`, not the models table, so on
activation the model's batch size is mirrored into config. Context window and
threads go straight into the embedder via `SetModel`.

Context window `0` is a sentinel meaning "the model's native maximum". The
folder scan and import both read `<arch>.context_length` from the GGUF header
and store it on the row, and a model that still has `0` is passed through to
llama.go, which resolves it to `llama_model_n_ctx_train` after load. There is no
hardcoded 2048 cap anymore.
Threads `0` means "all cores".

## Key decisions

- **Import copies into `models/`.** The app owns the file. Referencing in
  place would have made the folder scan's path checks meaningless.
- **Any dimension is supported, and dim changes are gated.** The alternative
  was restricting to 1024-dim models to avoid the vector-table rebuild, but
  real multi-model support means accepting models like nomic-embed-text
  (768d). Since a dim change drops every embedding, it sits behind an explicit
  confirm.
- **Dims and native context come from the GGUF header, not from loading the
  model.** Listing models never loads a ~1GB file just to read two numbers.
  The scan/import paths keep `context_length` and store it on the row, so each
  model runs at its real window (8192 for bge-m3) instead of a fixed 2048.
- **`needsReindex` is computed, not stored.** A dim change empties the vector
  tables for every collection at once, so a per-collection flag would be
  redundant. "Has documents but no embeddings" is exactly the state after a
  rebuild, and re-indexing fixes it automatically.
- **Active model lives in config.** The embedder path is needed before the DB
  is open. The table keeps `is_active` mirrored so the frontend has one
  obvious source for the dropdown.
- **`meta.vector_dim` is the single record of what the tables were built at.**
  Startup, the switch path, and the rebuild all compare against it, which
  covers the fresh-DB case where config points at a non-1024 model before any
  tables exist.

## Gotchas

- **`SetConfig` must not clobber the model fields.** The Settings form saves a
  whole config draft, and that draft carries stale copies of `active_model`,
  `embedding_model`, and `embedding_batch_size`, all of which the model flow
  owns. `SetConfig` in `index.go` preserves all three from the live config,
  or a routine settings save would revert the active model, its batch size,
  and the display-name fallback.
- **`NewEmbedder` changed signature.** It's now `NewEmbedder(path, ctx,
  threads)`. The test callers in `embed_test.go` and
  `indexer_integration_test.go` had to be updated.
- **GGUF reader needs `<arch>.embedding_length`.** It reads
  `general.architecture` first, then looks up `<arch>.embedding_length` and
  `<arch>.context_length`. A model that omits these gets dimension `0`, shown
  as "auto", and the real dimension is only confirmed when the model loads.
  Indexing with a wrong dimension fails loudly (sqlite-vec rejects a vector of
  the wrong length) instead of corrupting anything. Omitting `context_length`
  is safe: the row keeps `context_window = 0`, which the embedder treats as
  "use the native maximum".
- **Folder-scan rows used to write `batch_size = 0`.** `syncModelsFromFolder`
  built a zero-value `Model` and `UpsertModel` inserted `batch_size` verbatim,
  shadowing the schema's `DEFAULT 32`. The scan now sets the default
  explicitly, and `UpsertModel`'s `ON CONFLICT` backfills any surviving `0`
  rows (and `0` context windows) on the next scan without touching values the
  user tuned.
- **Stub method IDs must match the generated bindings.** `wails3 generate
  bindings` assigns stable IDs, but they still have to be mirrored in
  `scripts/vectile-stub.mjs`: ListModels 4184755701, ImportModel 3637578651,
  SetActiveModel 859586252, DeleteModel 3374659369, UpdateModelSettings
  2688418022.
- **Import is a synchronous copy.** A multi-hundred-MB model blocks the call
  with no progress UI yet. Fine for now; add a busy state when it becomes
  annoying.
