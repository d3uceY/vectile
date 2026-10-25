---
sidebar_position: 7
---

# Contributing

Bug reports, feature ideas, documentation fixes, and pull requests are welcome.

## Reporting a bug

Check the [issues](https://github.com/d3uceY/vectile/issues) first. A useful report includes:

1. OS and version
2. vectile version, from the window title or the release you installed
3. Which model is loaded, from **Settings → Model**
4. Steps to reproduce
5. What you expected, and what happened

## Suggesting a feature

Open an issue prefixed `[Feature]`. Describe the problem you are trying to solve, and how the change would solve it. Examples from other tools help.

## Development

| Tool | Version |
|---|---|
| [Go](https://go.dev/dl/) | 1.26 or later |
| [Wails CLI](https://wails.io/docs/gettingstarted/installation) | v3 |
| [Task](https://taskfile.dev/installation/) | v3 |
| Node.js | 20 or later |
| MinGW-w64, Windows only | On `PATH` |

```bash
git clone https://github.com/d3uceY/vectile.git
cd vectile
task dev
```

| Command | Does |
|---|---|
| `task dev` | run in development mode, with hot reload |
| `task build` | compile the binary into `bin/` |
| `task package` | build an installer for your operating system |
| `go test ./backend/...` | run the Go suite |
| `task docs` | serve this site |

The first build takes a while: the Go side links llama.cpp and the interface has to be built once. On Linux, install `libgomp1`; the model needs it for OpenMP.

Tests that need real embeddings skip themselves when the model file is missing. To run those too, put `bge-m3-Q4_K_M.gguf` in the app's models folder, or point `VECTILE_EMBED_MODEL` at a model anywhere on disk.

[How it is built](./how-it-is-built) covers the packages. The full build and packaging matrix lives in
[`dev-docs/build-and-packaging.md`](https://github.com/d3uceY/vectile/blob/main/dev-docs/build-and-packaging.md).

## House rules

- **Tabs, not spaces** in Go files. `gofmt -w` changes nothing on a clean checkout.
- **No em dashes in user-facing copy.** Use a period, a colon, or a semicolon.
- Settings values are clamped in the interface and checked again in the backend, so a hand-edited settings file cannot break an index run.

## Pull requests

1. Branch from `main`.
2. Make the change and confirm `task build` passes.
3. Run `go test ./backend/...`, and add a Go test for any non-trivial logic.
4. Describe what changed and why.
5. Open the pull request against `main`, linking related issues.

One fix or feature per pull request.

## Code of conduct

Be respectful. Everyone here contributes voluntarily. Constructive criticism is welcome; personal attacks are not.
