# CPU instruction sets and vectile's CPU compatibility

Why the Windows build can die on someone else's PC, which instruction sets
actually matter to this app, and how the repo keeps a bad build from shipping.
Written for the next person who opens this repo, in the same spirit as
`backend-notes.md`.

The specific bug that prompted this document is in `index-crash-fix.md`. This
one is the general background.

## The short version

vectile does not talk to llama.cpp over a socket or a DLL. The vendored
`third_party/llama-go` archives are linked straight into the binary, so which
instructions that code may use is decided when the archives are **compiled**, and
the CPU floor is baked into the shipped exe.

Run that exe on a CPU that predates the floor and the process dies on an illegal
instruction. There is no graceful path:

- It is a CPU fault, not a Go panic, so `recover()` in `backend/services/index.go`
  cannot catch it, and the `indexing:failed` event never fires.
- There is no log line from vectile. Windows may only leave an Event Viewer entry.
- It does not fail at startup. The model loads fine, because loading is mostly
  file I/O; the fault comes when the first matmul runs. From the user's side the
  app just vanishes the moment they press Index, or type a search.

That is why the CPU baseline is treated as a correctness property here, not a
performance knob.

## The instruction sets, in the order they arrived

SIMD sets come in two widths that matter: 128-bit (`xmm`) and 256-bit (`ymm`).
The trap that bit this project is that **AVX gives you 256-bit registers for
floats only**; 256-bit *integer* work has to wait for AVX2. Quantised model
weights are integer data, so that distinction turned out to be the whole ball
game.

| Set | First shipped | What it added | Relevance here |
|---|---|---|---|
| **SSE2** | 2001 Intel, 2003 AMD64 | 128-bit integer and float SIMD | The floor. Part of the x86-64 baseline, so **every 64-bit x86 CPU has it** |
| SSE3 | 2004 | horizontal add, `movddup` | minor for ggml |
| SSSE3 | 2006 (Core 2) | `pshufb` byte shuffle | quantised formats lean on byte shuffles |
| SSE4.1 | 2007 (Penryn) | `pmulld`, blends | minor for ggml |
| **SSE4.2** | 2008 Intel (Nehalem), 2011 AMD (Bulldozer) | string compare, `crc32`, `popcnt` | ggml's `GGML_SSE42` level. Note AMD only from Bulldozer; Phenom II has SSE4a, not 4.2 |
| **AVX** | 2011 (Sandy Bridge, Bulldozer) | 256-bit **float** ops, 3-operand VEX encoding | wide registers for floats, integer stays 128-bit |
| F16C | 2012 (Ivy Bridge, Piledriver) | half-float conversion | |
| FMA3 | 2012 AMD, 2013 Intel (Haswell) | fused multiply-add | matters for f32 kernels |
| BMI1/BMI2 | 2013 Intel (Haswell), 2015 AMD (Excavator) | scalar bit manipulation (`mulx`, `shlx`) | |
| **AVX2** | 2013 Intel (Haswell), 2015 AMD (Excavator) | 256-bit **integer** SIMD | **the one that matters most here** |
| AVX-512 | 2016-2017 (Xeon Phi, Skylake-X) | 512-bit registers, mask registers | rare on consumer parts; Alder Lake fused it off and newer parts dropped it. Forbidden here |
| **AVX-VNNI** | 2021 Intel (Alder Lake), 2024 AMD (Zen 5) | `vpdpbusd`: byte dot-product into int32 | what crashed the app, see below |

Two of these deserve spelling out.

**AVX2 is the cliff, not AVX.** The quantised dot products that do all the work
in an embedding model are integer and shuffle heavy, so they only get fast when
256-bit integer ops arrive. Between SSE2 and AVX2 nothing gives them a wide
path.

**AVX-VNNI is a very narrow requirement.** It arrived with Intel's 12th
generation in 2021 and, on the AMD side, only with Zen 5 in 2024. Machines from
2013 to 2020 are perfectly capable and still lack it, which is exactly how a
build made on a 13th-gen laptop shipped to a machine that could not run it.

## Which kernels needed what

From `objdump` on the committed archives, the AVX-VNNI instructions sat in:

```
ggml_vec_dot_q8_0_q8_0
ggml_vec_dot_q4_0_q8_0
ggml_vec_dot_q4_1_q8_1
ggml_vec_dot_q5_0_q8_0
ggml_vec_dot_q5_1_q8_1
ggml_gemv_q4_b32_8x8_q8_0_lut_avx
ggml_gemm_q4_b32_8x8_q8_0_lut_avx
tinyBLAS_Q0_AVX::gemm*
```

Those are the Q8_0 and Q4_0-family paths, which is precisely what the catalog's
recommended model (BGE Small EN v1.5 **Q8_0**) runs on every matmul. So the very
first embedding hit an AVX-VNNI instruction, and the app died before it could
report anything.

The lesson generalises: check what the *recommended* model exercises, not just
whatever happens to be on your own disk.

## Why the penalty is not uniform: measured

Same machine (i7-13650HX), batch of 8, n=128, with the linked archives proved by
disassembling the produced binary:

| model | `avx2` | `sse2` | penalty |
|---|---|---|---|
| BGE Small EN v1.5 **Q8_0** | 87 passages/sec | 46 | **1.9x** |
| BGE-M3 **Q4_K_M** | 12.2 passages/sec | 0.7 | **~17x** |

Q8_0 has a workable 128-bit fallback, so dropping to SSE2 costs about half the
throughput. The **K-quant** formats (Q4_K, Q6_K) do not: their dequantisation is
shuffle heavy and only has a fast path at AVX2, so an SSE-only build falls back
to narrow or generic code.

That matters because three of the four catalog models are K-quant, and 0.7
passages/sec means hours to index a real library. It is why `sse2` is a
deliberate legacy choice rather than the default, and why anyone on a legacy
build should be pointed at the recommended Q8_0 model.

## The levels the repo actually builds

`scripts/build-llamago-archives-windows.ps1 -Baseline` maps to explicit CMake
options, and `scripts/check-llamago-archives-portable.ps1` enforces the result:

| `-Baseline` | Runs on | CMake options set ON | Notes |
|---|---|---|---|
| `sse2` | every x86-64 CPU (2003+) | none (GCC targets the x86-64 baseline) | portable, ~17x slower on K-quants |
| `sse42` | 2008 Intel / 2011 AMD | `GGML_SSE42` | buys essentially nothing over `sse2`; SSE4.2 adds CRC and string ops, not wider integer SIMD |
| `avx2` | 2013 Intel / 2015 AMD | `GGML_SSE42`, `GGML_AVX`, `GGML_AVX2`, `GGML_FMA`, `GGML_F16C`, `GGML_BMI2` | **default**, and the floor for the Linux and macOS archives |

An `avx` level (Sandy Bridge 2011+) was attempted and removed: AVX already
produces 256-bit `%ymm` for float ops, so `objdump` cannot tell an AVX build from
an AVX2 build, and an unverifiable level is worse than no level.

Whatever the baseline, three options are always forced OFF, because they are the
ones that break machines: `GGML_NATIVE`, `GGML_AVX_VNNI` and the `GGML_AVX512*`
family. `GGML_AVX_VNNI` and `GGML_AVX512*` already default to `OFF` upstream, so
they are passed anyway as a guard against a future default flip.

### `GGML_NATIVE` is the trap

`GGML_NATIVE` defaults to `ON` and adds `-march=native`, which compiles for the
build machine's exact CPU. On a 12th-gen or newer Intel laptop that means
AVX-VNNI instructions, in a binary you then hand to other people. Setting
`-DGGML_AVX512=OFF` and friends does *not* undo it, which is how the original
misdiagnosis in `index-crash-fix.md` came about: those flags were no-ops.

The configure log is the quick confirmation. A portable build prints an empty
variant list, a native or AVX2 build prints the flags it chose:

```
-- Adding CPU backend variant ggml-cpu:                      # sse2
-- Adding CPU backend variant ggml-cpu: -msse4.2;-mf16c;-mfma;-mbmi2;-mavx;-mavx2   # avx2
```

## Other platforms

- **macOS arm64:** Apple Silicon has NEON unconditionally, so there is no
  baseline question. Not affected.
- **macOS amd64:** `scripts/build-llamago-archives.sh` builds at AVX2. Every Mac
  that can run macOS 12 (the app's floor) has AVX2, so this is effectively
  universal in practice.
- **Linux amd64:** the same script builds at AVX2, so it carries the same
  exposure to pre-2013 hardware that the Windows build used to. Not yet lowered;
  if you lower it, mirror the `-Baseline` plumbing and the CI check.

## Telling what a CPU supports

- **Look the model up.** `Get-CimInstance Win32_Processor | Select-Object Name`
  on Windows, then check the generation. This is usually enough, since the
  interesting boundaries are Haswell (2013) and Alder Lake (2021).
- **`coreinfo`** (Sysinternals) prints a feature list including `AVX`, `AVX2`
  and the VNNI bits.
- **From Go**, `golang.org/x/sys/cpu` exposes `cpu.X86.HasAVX`, `HasAVX2`,
  `HasSSE42`, `HasFMA` and the `HasAVX512*` set. AVX-VNNI is not in that struct
  on all versions, so for that one read the CPUID leaf the Intel SDM lists for
  `AVX-VNNI` (leaf 7, sub-leaf 1) rather than trusting a hardcoded bit number.
- **From the crash**: an illegal-instruction fault is exception code
  **`0xc000001d`** in Event Viewer, with the faulting module being the vectile
  exe. Contrast `0xc0000005` at a low address, which is the ABI mismatch
  described in `index-crash-fix.md`, not a CPU floor problem.

## Rules for working on this

1. **Never let `GGML_NATIVE` be `ON`** in a build whose archives get committed.
   Pass `-Baseline` and let the script set everything.
2. **Check the archive, not the symbol table.** An `avx512` symbol in an archive
   is only the `ggml_cpu_has_avx512` probe and says nothing about the code. Use
   `objdump -d` and look for actual instructions:

   ```
   # any VEX (c4/c5) or EVEX (62) encoded instruction: needs AVX or newer
   ^\s*[0-9a-f]+:\s+(c4|c5|62)\s(?:\s*[0-9a-f]{2})*\s+[a-zA-Z(]
   ```

   The trailing mnemonic matters: `objdump` wraps a long instruction's bytes onto
   extra lines that begin with an address and bytes but no mnemonic, so matching
   the first byte alone reports data as code (`movabs $0x736f622d...,%rax`
   continues with `62 6f 73`). On the AVX2 archive the naive pattern gave 9 hits
   in `libllama.a`, all of them data.
3. **Do not trust a build you have not proved.** Go's build cache keys the cgo
   link on the `CGO_*` flags and package hashes, *not* on the contents of the
   libraries it resolves through `-L`, so after swapping archives a `go build`
   can silently reuse the old link. Touching a file does not help (the cache is
   content-keyed, not mtime-keyed). Force it with a differing `-ldflags`, or
   disassemble the output and count VEX instructions.
4. **A Go binary is never VEX-free.** Ours contains around 1,400 VEX
   instructions from Go's own CPUID-dispatched stdlib code
   (`crypto/internal/fips140/sha256.blockAVX2`, `expandAVX512_*`, `cmpbody`).
   Those are guarded at runtime and fine. An AVX2-linked build adds
   roughly 45,000, so the two are easy to tell apart; check the enclosing
   symbols before blaming a residual hit.
5. **Keep the archives on the llama.cpp revision the vendored headers came from.**
   That is a separate constraint from the CPU floor and it bites differently: get
   it wrong and model load dies with `0xc0000005`. The builder's ABI guard
   handles it; see `index-crash-fix.md`.

## What a release ships

| Asset | Baseline | Audience |
|---|---|---|
| `vectile-windows-amd64-installer.exe` | `avx2` | the default, 2013+ CPUs |
| `vectile-windows-amd64-portable.zip` | `avx2` | same, no install step |
| `vectile-windows-amd64-legacy-installer.exe` | `sse2` | pre-2013 CPUs |
| `vectile-windows-amd64-legacy.zip` | `sse2` | same, no install step |

The legacy installer is the same `project.nsi` with `-DOUT_FILE`, and keeps the
same product identity and install path on purpose: installing it over the
default build is a clean in-place swap rather than a second app to keep in sync.

## If a user reports "it crashes when I index"

1. **Ask for the Event Viewer entry** and the exception code.
   - `0xc000001d` (illegal instruction): CPU floor. They need the legacy build.
   - `0xc0000005` at a low address: ABI mismatch, see `index-crash-fix.md`.
   - `fatal error: concurrent map read and map write`: the old config race, also
     in `index-crash-fix.md`.
   - Nothing at all, and it works on your machine: suspect a missing MinGW DLL
     instead; that fails at launch with `0xC0000135`, not during indexing.
2. **Check which model they use.** On a legacy build, K-quant models are ~17x
   slower and can look like a hang. Point them at BGE Small EN v1.5 Q8_0.
3. **Confirm with a build you can vouch for**, using rule 3 above.
