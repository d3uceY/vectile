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

### 2. CPU-specific native crash in the AVX-VNNI kernels

**Corrected 2026-09-26 (the original diagnosis was AVX-512 and it was wrong).**

The prebuilt `third_party/llama-go/windows/amd64/libggml-cpu.a` does **not**
contain AVX-512. The `avx512` strings in the archive are only the feature-probe
functions (`ggml_cpu_has_avx512*`), which every build contains; `objdump -d`
shows zero `%zmm` and zero `{k1}` masks.

What it does contain is **357 `vpdpbusd` instructions**, which are AVX-VNNI.
They sit in exactly the kernels a Q8_0 embedding model runs on every matmul:
`ggml_vec_dot_q8_0_q8_0`, `ggml_vec_dot_q4_0_q8_0`, `ggml_vec_dot_q5_0_q8_0`,
`ggml_gemv/gemm_q4_b32_8x8_q8_0_lut_avx`, and the `tinyBLAS_Q0_AVX::gemm*`
family. AVX-VNNI arrived with Intel Alder Lake (12th gen, 2021) and AMD Zen 5.

So the fault was not AVX-512 at all. The i7-1185G7 is Tiger Lake: AVX2 and
AVX-512, but **no AVX-VNNI**, so the first Q8_0 dot product raised
`STATUS_ILLEGAL_INSTRUCTION` (0xc000001d). The development machine is an
i7-13650HX (Raptor Lake): AVX2, FMA, F16C, BMI2 and AVX-VNNI, and no AVX-512,
which is precisely the instruction profile found in the archive because it was
built there with `-march=native`.

Note that a CPU fault like this is not a Go panic, so no `recover()` in
`backend/services/index.go` can catch it.

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

  **Only `-DGGML_NATIVE=OFF` does anything.** `GGML_AVX_VNNI`, `GGML_AVX512`,
  `GGML_AVX512_VBMI` and `GGML_AVX512_VNNI` are already `OFF` by default in
  upstream `ggml/CMakeLists.txt`, so passing them is a no-op. The trap is
  `GGML_NATIVE`, which defaults to `ON` and adds `-march=native`; that is what
  made the compiler emit AVX-VNNI on an Alder Lake or newer build machine.

  With `GGML_NATIVE=OFF`, llama.cpp instead enables SSE4.2/AVX/AVX2/FMA/F16C/
  BMI2 explicitly (a Haswell-2013 floor) and never AVX-VNNI or AVX-512.

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

3. **Portable archives remove the CPU trap.** `-DGGML_NATIVE=OFF` means the
   native embedding code runs the same AVX2 path on every machine (Haswell 2013
   and newer), eliminating instruction-set-specific crashes.

## Verification

- `go test ./backend/config ./backend/chunker` passes (including the updated
  `TestIsCollectionEnabled`).
- Editor language server reports no errors on all changed files.

To confirm which failure hit a given machine: check Windows Event Viewer for
the faulting module and exception code. `0xc000001d` (illegal instruction)
points at the AVX-512 path; a Go `fatal error: concurrent map read and map
write` in the logs points at the map race (now fixed in code).

## Follow-up (resolved 2026-09-26)

The step above was never actually taken: `third_party/llama-go/windows/amd64/`
was still the 2026-08-22 build when this was fixed, so every Windows release up
to that point still carried the AVX-VNNI instructions.

### What was actually going on

- **The archives were not built by this repo's tooling.** All seven files were
  byte-identical copies of `references/Clipcat/third_party/llama-go/*.a`, built
  from a llama-go checkout whose `llama.cpp` submodule matched the vendored
  headers. The skill's `build.ps1` has since gained `-DGGML_NATIVE=OFF`, but
  nothing ever rebuilt the committed copies.
- **The vendored headers pin the ABI.** cgo compiles the bindings against
  `third_party/llama-go/llama.cpp`, so the archives must be built from the same
  `llama.cpp` revision. Matching `wrapper.h` alone is not enough: llama-go
  `9cd52560` (2026-07-20, llama.cpp `178a6c4493` / b10069) is the ref whose
  `llama.h` and `ggml.h` are identical to the vendored copies, while HEAD has
  since bumped to b10675 and changed `llama_model_params`. Building the
  libraries from b10675 links a llama.cpp that disagrees with the bindings, and
  the process dies with `0xc0000005` reading address `0x104` inside
  `llama_wrapper_model_load` before it ever reaches a kernel.

### What shipped

- `scripts/build-llamago-archives-windows.ps1` (new): the Windows twin of
  `build-llamago-archives.sh`. It auto-detects the llama-go ref whose
  `wrapper.h`, `llama.h` and `ggml.h` all match the vendored copies (it skips
  HEAD and lands on `9cd52560`), builds with `-DGGML_NATIVE=OFF`, compiles the
  vendored `wrapper.cpp`, and refuses to install anything non-portable.
- `scripts/check-llamago-archives-portable.ps1` (new): `objdump` scan that fails
  on `vpdpbusd` / `%zmm` / `{k1}` and asserts `libggml-cpu.a` still has AVX2 code
  (so a vacuous pass cannot happen). The builder runs it on the staging
  directory, and the Windows release job runs it before building, so the
  committed archives cannot regress.
- The seven committed archives were rebuilt from `9cd52560` with
  `-DGGML_NATIVE=OFF` and verified: `go test ./backend/embeddings/` loads
  bge-m3 Q4_K_M and the recommended `bge-small-en-v1.5-q8_0` model embeds
  through the AVX-VNNI-free kernels, and the full `go test ./backend/...` suite
  passes.
- The portable Windows release is now `vectile-windows-amd64-portable.zip`
  rather than a bare exe. It needs the five MinGW runtime DLLs beside it
  (`libgomp-1.dll` in particular), so the exe alone could not start on a machine
  that had never seen MinGW; the installer was unaffected because `project.nsi`
  installs the DLLs.

### CPU floor: a build parameter, default `avx2`

There is no free lunch here, and the measurement is what settled it. On an
i7-13650HX, batch of 8, n=128, with the linked archives verified by
disassembling the produced binary (see the build-cache note below):

| model | `avx2` | `sse2` | penalty |
|---|---|---|---|
| bge-small-en-v1.5 Q8_0 | 87 passages/sec | 46 | ~1.9x |
| bge-m3 Q4_K_M | 12 passages/sec | 0.7 | **~17x** |

Q8_0 barely notices, but K-quant models fall off a cliff. ggml's K-quant dot
products (Q4_K/Q6_K) have a fast path only for AVX2, so an SSE-only build drops
to generic code. Three of the four catalog models are K-quant, and at 0.7
passages/sec bge-m3 would need hours to index a real library. That makes `sse2`
a poor default in exchange for pre-2013 CPUs, so the default is **`avx2`**
(Haswell 2013+) and `sse2` exists for a deliberate legacy build. On such a
machine, steer the user at the recommended Q8_0 model, which stays usable at
~46 passages/sec.

An `avx` (Sandy Bridge 2011+) level was attempted and then removed: AVX already
has 256-bit `%ymm` for float ops, so objdump cannot separate AVX from AVX2, and
an unverifiable level is worse than none.

Detection is structural rather than symbol-based: no VEX (`c4`/`c5`) or EVEX
(`62`) encoded instruction may appear for the SSE levels, which is exact in
64-bit machine code and covers AVX, AVX2, FMA, F16C, VEX-encoded BMI,
AVX-VNNI and AVX-512 in one test. The mnemonic must be present on the line,
otherwise objdump's continuation lines for long immediates
(`movabs $0x736f622d...,%rax` continues with `62 6f 73`) read as data and produce
false hits.

### Measurement trap: the Go build cache does not re-link on archive changes

Go's build cache keys the cgo action on the C sources and the CGO_* flag
strings, not on the contents of libraries pulled in via `-L`. So after swapping
the archives, `go run`/`go test` can silently execute the OLD ones: a "SSE2"
benchmark reported the AVX2 figure (85 passages/sec) until a source edit changed
the main package and forced a fresh link. Touching a file does not help, because
the cache is content-keyed rather than mtime-keyed. Either force the link
(`-ldflags "-X main.x=y"`, a new package, or a real source change) or verify
the artifact: `go build -o x.exe`, then `objdump -d x.exe` and count VEX/EVEX.
An AVX2-linked binary has ~45,000; an SSE2-linked one has ~1,400, all of which
come from Go's own CPUID-dispatched stdlib code (`sha256.blockAVX2`,
`expandAVX512_*`, `cmpbody`).

### Checking an affected machine

Windows Event Viewer, Application log: the faulting exception code is
**`0xc000001d`** (illegal instruction). `0xc0000005` at a low address points at
an ABI mismatch instead (see above), and a Go `fatal error: concurrent map read
and map write` was cause #1.
