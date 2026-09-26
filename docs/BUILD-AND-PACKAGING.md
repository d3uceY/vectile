# Building and packaging vectile on Windows, Linux and macOS

vectile runs the bge-m3 embedding model **in-process** through [`llama-go`](https://github.com/tcpipuk/llama-go),
a CGO binding to llama.cpp. Because that pulls in native C/C++, every platform needs its own set
of llama.cpp static archives at link time. Only the **Windows** archives are **committed** under
`third_party/llama-go/windows/amd64/`; the Linux and macOS archives are **built on the CI runner**
before each release (see below). A build only needs a suitable C/C++ compiler on `PATH` (plus
`LIBRARY_PATH`/`C_INCLUDE_PATH` pointing at the llama-go root) - it never rebuilds llama.cpp.

This document explains the layout, the one-off archive build, the per-platform prerequisites, and
how each target is built, packaged and shipped.

## What actually needs to be native

- `third_party/llama-go` ships:
  - `linkage_static.go` - the **shared** cgo include flags (`-I` paths, OS-agnostic).
  - `linkage_<os>_<arch>.go` - the **per-OS/per-arch** cgo `LDFLAGS` (`-L` + lib list).
  - `wrapper.cpp` / `wrapper.h` - the Go<->C++ bridge, compiled into `libbinding.a`.
  - `llama.cpp/` - **headers only** (the public llama.cpp API that `wrapper.cpp` and the Go code
    include). No `.c`/`.cpp` sources live here.
- The llama.cpp **implementation** is compiled into the static archives in
  `third_party/llama-go/<os>/<arch>/`.

Because the archives are platform-specific (COFF on Windows, ELF on Linux, Mach-O on macOS), there
is **no single "cross-compile for all three from one machine"** path for the C++ side. You build on
(or for) each target.

## The archive layout

```
third_party/llama-go/
  windows/amd64/      libbinding.a, libllama.a, libllama-common*.a, libggml*.a   (MinGW/COFF)
  linux/amd64/        same names, ELF                                            (gcc, OpenMP)
  darwin/arm64/       same + libggml-metal.a + libggml-blas.a, arm64 Mach-O      (clang)
  darwin/amd64/       same + libggml-metal.a + libggml-blas.a, x86_64 Mach-O
```

Each directory holds the archives that the matching cgo `linkage_*` file links (`-L./<os>/<arch>`).

| Build tag file | Active on | Links |
|---|---|---|
| `linkage_windows_amd64.go` | `windows && amd64` | `... -lstdc++ -lm -lgomp` |
| `linkage_linux_amd64.go`   | `linux && amd64`   | `... -lstdc++ -lm -lgomp` |
| `linkage_darwin_arm64.go`  | `darwin && arm64`  | `... -lggml-metal -lggml-blas ... -framework Metal` |
| `linkage_darwin_amd64.go`  | `darwin && amd64`  | `... -lggml-metal -lggml-blas ... -framework Metal` |

### Runtime library dependencies

| Platform | What the archive links to | What you ship |
|---|---|---|
| Windows | MinGW runtime (`libgcc_s`, `libstdc++`, `libwinpthread`, `libgomp`, `libdl`) | The 5 MinGW DLLs next to the exe (`build/windows/runtime`) |
| Linux | `libgomp.so.1` (OpenMP) + `libstdc++` | Bundle `libgomp.so.1` in the AppImage; `Depends: libgomp1` + `libstdc++6` in the .deb |
| macOS | Metal / Accelerate / BLAS (system frameworks) | Nothing - frameworks are OS-provided |

## Rebuilding the llama.cpp archives (one-off, per OS/arch)

The vendored `llama.cpp/` is headers-only, so creating archives for a new OS/arch requires a full
llama.cpp source build. `scripts/build-llamago-archives.sh` does this and writes the finished `.a`
files into `third_party/llama-go/<os>/<arch>/`. It is the bash analogue of the
`llama-go-windows` skill's `reference/build.ps1`.

```
# Linux amd64
./scripts/build-llamago-archives.sh

# macOS arm64 (run on an Apple Silicon Mac)
./scripts/build-llamago-archives.sh

# macOS amd64 (run on an Intel Mac, or cross-build from ARM64 with GOARCH=amd64)
GOARCH=amd64 ./scripts/build-llamago-archives.sh

# Or pin a specific llama-go ref (auto-detect otherwise)
LLAMA_GO_REF=<commit-or-tag> ./scripts/build-llamago-archives.sh
```

> **`LLAMA_GO_REF` is optional.** The script must use the llama-go commit/tag whose `wrapper.h` is
> vendored in `third_party/llama-go` (the Go<->C++ ABI depends on it), but when the variable is
> unset the script auto-detects it: it clones upstream, tries `HEAD`, then every tag (newest first),
> and takes the first whose `wrapper.h` matches the vendored copy. When set, it verifies the ref
> matches (ABI guard) and aborts if not. The produced `.a` files can be committed, or left for the
> CI runner to build each release (only `bin/` is gitignored, `*.a` is not). `GOARCH` overrides the
> detected arch so you can cross-build (e.g. an ARM64 runner producing the x86_64 macOS archives).

Requirements: `gcc`/`g++` + `cmake` + `make` (or `ninja`) + `git` + `ar` on Linux; Xcode Command
Line Tools on macOS. Linux builds configure with `GGML_OPENMP=ON` (that's why `libgomp` is needed
at runtime); macOS builds with the Metal + BLAS backends on.

## Building and packaging per platform

The Wails task runner dispatches to the platform Taskfile via `GOOS` (see the root `Taskfile.yml`):

```
task build       # build the binary for the current OS  (GOOS=<host>)
task package     # build + package an installer for the current OS
task dev         # run in development mode
```

### Windows (amd64)

- **Toolchain:** MinGW-w64 (WinLibs) on `PATH`; `CGO_ENABLED=1`, `GOOS=windows`, `GOARCH=amd64`.
  The Windows Taskfile sets the WinLibs path and `LIBRARY_PATH`/`C_INCLUDE_PATH` automatically.
- **Build:** `task windows:build` (or `wails3 build`).
- **Package:** `task windows:package` builds an NSIS installer. The 5 MinGW DLLs in
  `build/windows/runtime/` are installed next to the exe and included in the installer by
  `project.nsi`.
- **Runtime:** ship the exe with the 5 MinGW DLLs beside it (the release publishes them as
  `vectile-windows-amd64-portable.zip`), plus the `.gguf` model (the user
  places the model in `<UserConfigDir>/vectile/models/`). Missing `libdl.dll` causes a silent
  `0xC0000135` exit at launch.
- **CPU floor:** `scripts/build-llamago-archives-windows.ps1` takes `-Baseline sse2|sse42|avx2`
  (default `avx2`, Haswell 2013+) and compiles out everything above it;
  `scripts/check-llamago-archives-portable.ps1` enforces it in CI. Measured on an i7-13650HX,
  batch of 8: `avx2` 87 passages/sec vs `sse2` 46 on bge-small Q8_0, but 12 vs **0.7** on bge-m3
  Q4_K_M. K-quant models have a fast path only for AVX2, so a portable `sse2` build costs ~17x on
  three of the four catalog models; that is why `avx2` is the default and `sse2` is a deliberate
  legacy choice (pair it with the recommended Q8_0 model, which stays usable).

### Linux (amd64)

- **Toolchain:** `gcc`/`g++`, `pkg-config`, `libgtk-4-dev`, `libwebkitgtk-6.0-dev`,
  `libayatana-appindicator3-dev`; `CGO_ENABLED=1`, `GOOS=linux`, `GOARCH=amd64`.
- **Build:** `task linux:build` (uses `third_party/llama-go/linux/amd64`).
- **Package:** `task linux:package` produces an AppImage, `.deb`, `.rpm` and an AUR package:
  - `.deb` (`build/linux/nfpm/nfpm.yaml`) declares `Depends: libgtk-4-1, libwebkitgtk-6.0-4,
    libgomp1` and installs the binary to `/usr/local/bin/vectile`.
  - AppImage (`build/linux/appimage/build.sh`) runs `linuxdeploy` to bundle the app and also copies
    `libgomp.so.1` into the AppDir so it runs on systems without gcc.
- **Runtime:** the binary only depends on `libgomp.so.1` + `libstdc++` (no llama/ggml shared libs).
  The `.deb` pulls `libgomp1`; the AppImage bundles it. The model still goes in
  `<UserConfigDir>/vectile/models/`.

### macOS (universal arm64 + amd64)

- **Toolchain:** Xcode Command Line Tools (`clang`/`clang++`); `CGO_ENABLED=1`, `GOOS=darwin`,
  `-mmacosx-version-min=12.0`. `task darwin:build` builds a single arch; `task darwin:build:universal`
  builds both and `lipo`s them together. The goal is a universal `.app`.
- **Package:** `task darwin:package` (or `package:universal`) assembles the `.app` bundle;
  `package:dmg` wraps it in a DMG. `Info.plist` is at `build/darwin/Info.plist` (bundle id
  `com.d3ucey.vectile`, icon file `iconfile.icns`).
- **Runtime:** nothing external to ship (Metal/Accelerate/BLAS are OS-provided). Distribute the
  `.app`/`.dmg`; the user places the model in `~/Library/Application Support/vectile/models/`.
- **Signing:** local builds use ad-hoc `codesign`; releases are unsigned (right-click -> Open on
  first launch). Notarization is out of scope.

## The release workflow (GitHub Actions)

A stable `vX.Y.Z` tag triggers `.github/workflows/release.yml`, which builds and publishes all three
platforms:

1. **`validate-tag`** requires `^v\d+\.\d+\.\d+$` (no prereleases).
2. **`build-windows-amd64`** (windows-latest): MinGW + CGO, injects the version, builds the exe,
   copies the 5 DLLs, runs `makensis`, uploads the installer + portable exe. **Notable:** the choco
   `mingw` 16.1.0 package is pinned and `CC`/`CXX` are set so the ABI matches the committed archives;
   the `.rsrc merge failure: multiple non-default manifests` linker warning is a known, non-fatal
   `wails3 generate syso` quirk (blank file-version metadata only).
3. **`build-linux-amd64`** (ubuntu-24.04): apt GTK/WebKit/appindicator deps + `cmake`, then
   `scripts/build-llamago-archives.sh` builds the Linux ELF archives into
   `third_party/llama-go/linux/amd64/` before the CGO build, then `.deb` via
   `wails3 tool package -format deb` and AppImage via `wails3 generate appimage`
   (`APPIMAGE_EXTRACT_AND_RUN=1` avoids FUSE issues on CI).
4. **`build-macos`** (macos-latest): `scripts/build-llamago-archives.sh` builds **both** the arm64
   and amd64 archives (the runner is ARM64, so amd64 is cross-built via `GOARCH=amd64`), then
   builds arm64 + amd64 binaries, `lipo`s them into a universal binary, assembles the `.app`, then
   produces a `.dmg` and a `.zip`.
5. **`release`** (ubuntu): gathers all artifacts, writes `SHA256SUMS.txt`, and publishes the GitHub
   Release with a download table + first-run notes. `cleanup-tag-on-failure` deletes the tag if any
   job failed.

The version injected into `main.Version`, the Windows metadata, the nfpm version and the Info.plist
all come from the git tag (minus the leading `v`).

## Build cache and build times

Go's build cache is on by default (`go env GOCACHE` -> `%LOCALAPPDATA%\go-build`) and nothing in the
Wails v3 build system clears it: `wails3 dev`, `wails3 build` and `wails3 task <platform>:build` all
end up running a plain `go build`, and `wails3 build` has no clean/force flag. Measured on this repo
with a warm cache:

| Step | Time |
| --- | --- |
| `go mod tidy` (no-op, but runs on every build) | 0.8 s |
| `wails3 generate bindings` | 2.5 s |
| frontend `vite build` | 2.7 s |
| `go build`, dev flags (warm) | 0.3 s |
| `go build`, production flags (warm) | 0.3 s |
| `go build`, production flags, first build in that flag set | 17.6 s |

So a warm `wails3 task build DEV=true` is about 7 s end to end, and "over a minute" means the cache
did not apply to the compile. Go keys each package on the toolchain version plus the exact compiler
flags, so a cache only pays off inside one flag set, and this repo has two that never share:

- **dev:** `-buildvcs=false -gcflags=vectile/...=-l`
- **production:** `-tags production -trimpath -buildvcs=false -ldflags="-w -s ..."`

Adding, removing or reordering a flag starts a third, empty cache set. To keep builds cached:

- **Never run `go clean -cache`** (or delete `%LOCALAPPDATA%\go-build`) to "fix" a build. It is the
  one thing guaranteed to make the next build take minutes: the standard library and every
  dependency has to be recompiled again, once per flag set in use.
- Expect exactly one slow build after a Go toolchain upgrade, since a new toolchain invalidates
  every entry and the cache refills gradually as you work.
- Keep antivirus out of the cache, or real-time scanning of a ~600 MB cache is paid on every build.
  Run once in an elevated PowerShell:

  ```powershell
  Add-MpPreference -ExclusionPath "$env:LOCALAPPDATA\go-build", "$env:USERPROFILE\go\pkg\mod", "$env:ProgramFiles\Go"
  ```

- Do not fiddle with the flag strings in `build/<os>/Taskfile.yml`. Any change means the next build
  of that mode is a full rebuild, so change them deliberately and not while iterating.
- Note the deliberate deviation from the Wails template in the DEV flag set: `-gcflags` is scoped to
  `vectile/...` rather than the template's `all="-l"`. With `all=` the standard library and every
  dependency are compiled into a dev-only namespace, so a dev build shares no compiled artifacts
  with `go build`, `go test`, `go vet` or gopls, and the first dev build after a cache reset has to
  rebuild the whole graph. Scoped, only this module's packages get `-l` (which is the code a
  debugger steps through anyway) and the dependencies reuse the normal cache entries.

## Troubleshooting

- **`go build` can't find `-lggml` / link fails on an OS** - the archives for that OS/arch aren't in
  `third_party/llama-go/<os>/<arch>/`. Build them with `scripts/build-llamago-archives.sh` (set
  `GOARCH` to cross-build), or commit the produced archives. On the release runners the workflow
  builds them automatically before `go build`.
- **`LLAMA_GO_REF` mismatch** - the script aborts because the fetched `wrapper.h` differs from the
  vendored one. Find the llama-go ref that matches (usually the release the vendored copy came from).
- **Windows exe exits `0xC0000135`** - a MinGW DLL is missing next to the exe (usually `libdl.dll`).
- **Linux binary fails to start in a container** - `libgomp.so.1` is not installed. Install
  `libgomp1` (or remove it from a rebuilt AppImage's need).
- **AppImage build fails on CI with a FUSE error** - set `APPIMAGE_EXTRACT_AND_RUN=1` so linuxdeploy
  doesn't need FUSE.
