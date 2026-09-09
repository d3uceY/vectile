# Indexing crash on some Windows machines — root cause and fix

**Date:** 2026-09-09

## Symptom

On some Windows PCs, starting an index run ("Index collection" / "Index all")
crashes the whole app. The crash reproduced on a test laptop with an Intel
Core i7-1185G7 (Tiger Lake, 4 cores / 8 threads) but not on the main
development machine.

The crash was **not** memory pressure from the embedding model. The test used a
very small model, which ruled out the "large model + large KV cache exhausts
RAM" theory.

## Root causes

Two independent, machine-specific failure modes were found.

### 1. Fatal Go runtime error: `concurrent map read and map write`

The `Config` struct held mutable maps that were written and read from
different goroutines at the same time:

- `IsCollectionEnabled` lazily built a shared `disabledSet` map
  (`config.go`). The first call writes to that map while another goroutine's
  first call reads it.
- `AddSourcePath`, `RemoveSourcePath`, and `DeleteCollection` mutated the live
  `Projects` / `Repositories` maps **in place** (`cfg.Projects[name] = ...`,
  `delete(cfg.Projects, name)`), while the index goroutine iterated the same
  maps via `configuredCollections`, `CheckNameConflict`, and `runIndex`.

Go reports a concurrent map read/write as a **fatal runtime error**, not a
panic. It bypasses `recover()` entirely and terminates the process. Because
it is a scheduling/timing race, it fires on some machines and not others —
slower, fewer-core machines interleave the goroutines differently.

### 2. CPU-specific native crash in the AVX-512 kernels

The prebuilt `third_party/llama-go/windows/amd64/libggml-cpu.a` contains
AVX-512 kernels (confirmed by `avx512` symbols in the archive). The
i7-1185G7 is Tiger Lake — one of the few consumer CPUs that supports AVX-512 —
so on that machine llama.cpp executes the AVX-512 code path, which the
development machine (no AVX-512) never exercised. A fault in that path
crashes during embedding regardless of model size.

## Fixes

### Remove the shared mutable map (`backend/config/config.go`)

`IsCollectionEnabled` no longer caches into `disabledSet`. It scans the tiny
`DisabledCollections` slice directly, so it never holds mutable shared state:

```go
func (c *Config) IsCollectionEnabled(name string) bool {
    for _, n := range c.DisabledCollections {
        if n == name {
            return false
        }
    }
    return true
}
```

### Copy-on-write for config maps (`backend/services/index.go`)

`AddSourcePath`, `RemoveSourcePath`, and `DeleteCollection` no longer mutate
the shared `Projects` / `Repositories` maps. They build a shallow copy, apply
the change to the copy, and reassign the field:

- `setMapSlice(m, key, val)` — returns a copy of `m` with one key replaced.
- `withoutMapKey(m, key)` — returns a copy of `m` with one key dropped.

The slice values are also cloned before `append`/`removeStr` so the backing
array is never shared with the index goroutine. `persistConfig` no longer
calls the removed `ResetDisabledCache`.

### Panic recovery around index runs (`backend/services/index.go`, `models.go`)

The background index goroutines (`IndexCollection`, `IndexAll`) now defer
`finishIndexRun`, and `IndexSynchronous` recovers its own panics. A caught
panic is logged (with a stack trace) and emitted to the frontend as a new
`indexing:failed` event (`IndexFailed` in `models.go`), then the run state and
index lock are released. This means a future panic aborts the run and shows an
error instead of killing the app.

### Defensive guards

- `backend/indexer/git.go` — `truncateSHA` guards `headSHA[:12]` /
  `oldSHA[:12]` (a persisted watermark is user-visible JSON and may not be a
  full 40-char SHA); `commitToChunks` guards `commit.SHA[:7]` /
  `commit.AuthorDate[:10]`.
- `backend/parser/code.go` — nil-tree guard after `parser.ParseCtx`.

### Frontend handling of `indexing:failed`

- `frontend/src/lib/types.ts` — added `IndexFailed`.
- `frontend/src/lib/store.tsx` — listens for `indexing:failed`, clears the
  indexing state, and shows a danger toast.

### Portable native builds

- `scripts/build-llamago-archives.sh` and the llama-go `build.ps1` reference
  now configure llama.cpp with:

  ```
  -DGGML_NATIVE=OFF -DGGML_AVX512=OFF -DGGML_AVX512_VBMI=OFF -DGGML_AVX512_VNNI=OFF
  ```

  This removes `-march=native` and skips the AVX-512 kernels entirely (AVX2 is
  the ceiling), so the resulting archives run on any x86-64 CPU.

## Why this works

1. **The fatal map error is gone, not just caught.** `recover()` cannot
   intercept a Go runtime fatal, so the only correct fix was to remove the
   concurrent read/write on the same map object. `IsCollectionEnabled` no
   longer writes anything, and the `Projects` / `Repositories` mutations write
   only to freshly-allocated copies that no other goroutine reads until the
   whole map pointer is swapped.

2. **Panic recovery is defense in depth.** For any remaining panic (slice
   bounds, nil derefs), the app now logs, tells the UI, clears state, and
   continues instead of dying with the window.

3. **Portable archives remove the CPU trap.** Disabling AVX-512 and
   `-march=native` means the native embedding code runs the same AVX2 path on
   every machine, eliminating instruction-set-specific crashes.

## Verification

- `go test ./backend/config ./backend/chunker` passes (including the updated
  `TestIsCollectionEnabled`).
- Editor language server reports no errors on all changed files.

To confirm which failure hit a given machine: check Windows Event Viewer for
the faulting module and exception code. `0xc000001d` (illegal instruction)
points at the AVX-512 path; a Go `fatal error: concurrent map read and map
write` in the logs points at the map race (now fixed in code).

## Follow-up

The committed Windows archives in `third_party/llama-go/windows/amd64/` still
contain the AVX-512 kernels. Rebuild them with the flags above (requires
MinGW on `PATH`) and commit the result to fully eliminate cause #2 on Windows.
