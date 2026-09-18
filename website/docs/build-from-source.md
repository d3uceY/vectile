---
title: Build from source
description: Compile vectile yourself on Windows, macOS, or Linux.
---

# Build from source

## What you need first

- **Go 1.26** or newer
- **Node.js** and npm
- the **wails3** CLI
- **Windows:** MinGW-w64 on your `PATH`, with `LIBRARY_PATH` and `C_INCLUDE_PATH` pointing at
  `third_party/llama-go`. The build task sets both for you, but a plain `go build` will not link
  without them.
- **Linux:** `libgomp1`, which the model needs for OpenMP

## Commands

From the repository root:

```bash
task dev       # run in development mode, with hot reload
task build     # compile the binary into bin/
task package   # build an installer for your operating system
```

The first build takes a while: the Go side links llama.cpp and the interface has to be built once.

## The vendored model engine

llama.cpp arrives as Go bindings under `third_party/llama-go`, with static libraries for each
platform beside them:

```text
third_party/llama-go/windows/amd64/
third_party/llama-go/linux/amd64/
third_party/llama-go/darwin/arm64/
third_party/llama-go/darwin/amd64/
```

Those libraries have to be built on the platform they run on. To build one, or rebuild all of them
after a llama.cpp upgrade:

```bash
./scripts/build-llamago-archives.sh
```

The script clones llama-go, finds the version whose `wrapper.h` matches the one vendored here,
builds the libraries with CMake, and installs them in the right folder. Pin the version with
`LLAMA_GO_REF`. On an Apple Silicon Mac, `GOARCH=amd64` cross-builds the Intel libraries.

## Running the tests

```bash
go test ./backend/...
```

Tests that need real embeddings skip themselves when the model file is missing. To run those too,
put `bge-m3-Q4_K_M.gguf` in the app's models folder, or point `VECTILE_EMBED_MODEL` at a model
anywhere on disk.

:::warning
Do not pipe the test output through a filter such as `Select-Object`. The stream buffers and the run
looks hung when it is not.
:::

## House rules

- **Tabs, not spaces** in Go files. `gofmt -w` changes nothing on a clean checkout.
- **No em dashes in user-facing copy.** Use a period, a colon, or a semicolon.
- Settings values are clamped in the interface and checked again in the backend, so a hand-edited
  settings file cannot break an index run.

## Release builds

The release workflow runs on a version tag shaped like `v1.2.3`. It builds the Windows installer and
portable exe, the Linux AppImage and deb, and the macOS dmg and zip, then publishes them to GitHub
Releases. The tag is the only place the version is written.

One Windows detail matters if you touch CI: the MinGW version is pinned to match the GCC that
produced the committed llama-go libraries. A different GCC gives link errors that mention things like
`std::__get_once_call` rather than the compiler.
