# Contributing to vectile

Thanks for wanting to contribute! vectile is a personal, keyboard-first desktop search engine for your own library — the work runs in-process, and document content never leaves the machine. Outbound requests are limited to the launch update check and user-initiated model and OCR downloads; a new one needs a line in `SECURITY.md` and `README.md`.

This guide covers how the repo is organized, how to build and test, and what a good pull request looks like.

## Quick links

- [README.md](README.md) — what vectile is and how to use it
- [PRODUCT.md](PRODUCT.md) — product goals and constraints
- [DESIGN.md](DESIGN.md) — architecture notes
- [dev-docs/app-flow.md](dev-docs/app-flow.md) — how the app flows end to end
- [dev-docs/backend-notes.md](dev-docs/backend-notes.md) — backend conventions and platform gotchas
- [dev-docs/cpu-compatibility.md](dev-docs/cpu-compatibility.md) — CPU instruction sets, which ones the binaries need, and why that can crash an older PC

## Development environment

**Required tools:**

- **Go 1.26+** (see `go.mod`)
- **Node.js + npm** for the frontend (`frontend/package.json`)
- **Wails v3 CLI** (`wails3`) — the build tool
- **Go Task** for `Taskfile.yml` tasks
- **Windows only:** MinGW-w64 on `PATH`, plus the five MinGW runtime DLLs beside the exe. The llama-go (llama.cpp) CGO build depends on it. See `dev-docs/backend-notes.md` for exact setup.

**Optional but useful for tests:**

- A `bge-m3-Q4_K_M.gguf` model in `<UserConfigDir>/vectile/models/`. Model-dependent tests skip automatically when it's missing.

## Building and running

```bash
task dev        # run with hot reload (wails3 dev)
task build      # production binary in bin/
task package    # package an installer for the current OS
task run        # run the built binary
go test ./backend/...
```

vectile includes llama.cpp (a C++ library that runs the embedding model) through the bundled
`third_party/llama-go`, whose prebuilt archives are committed per OS and CPU under
`third_party/llama-go/{windows,linux,darwin}/<arch>`. You only need a C/C++ compiler on your `PATH`;
see [`dev-docs/build-and-packaging.md`](dev-docs/build-and-packaging.md) for the full cross-platform
build, packaging, and release notes.

- **Windows (amd64):** needs MinGW-w64 on `PATH`, with `LIBRARY_PATH`/`C_INCLUDE_PATH` pointing at
  `third_party/llama-go` (the Windows build task sets these). The built exe needs five MinGW runtime
  DLLs beside it (`libgcc_s_seh-1.dll`, `libgomp-1.dll`, `libstdc++-6.dll`, `libwinpthread-1.dll`,
  `libdl.dll`). Missing `libdl.dll` causes a silent `0xC0000135` exit at launch.
- **Linux (amd64):** needs `gcc`/`g++`, `pkg-config`, `libgtk-4-dev`, `libwebkitgtk-6.0-dev` and
  `libayatana-appindicator3-dev`. The binary depends on `libgomp.so.1` at runtime (bundled in the
  AppImage, `Depends: libgomp1` in the `.deb`). `task linux:package` yields AppImage + `.deb`
  (+ `.rpm`/AUR).
- **macOS (universal):** needs Xcode Command Line Tools. `task darwin:build:universal` builds
  arm64 + amd64 and `lipo`s them together; `task darwin:package:dmg` wraps the `.app` in a DMG. The
  Metal/Accelerate frameworks are OS-provided, so nothing extra ships.

## Where things live

```
main.go                     app startup, window, services, auto-reindex loop
backend/appdata             the data directory and the model path
backend/config              config.json load, save, defaults
backend/db                  SQLite schema and helpers (modernc + vec0 + FTS5)
backend/embeddings          the llama.go embedder (bge-m3)
backend/chunker             word-window and markdown chunking
backend/parser              file parsers: md, docx, html, epub, pdf, xlsx, pptx, ipynb, xml, sql, shell, csv/json, calibre, code
backend/search              hybrid search: vector + FTS + RRF
backend/indexer             obsidian, project, git, calibre indexers; prune
backend/services            Wails services the UI calls
backend/startup             launch-at-login per OS
third_party/llama-go        vendored llama.cpp bindings (do not edit casually)
frontend/src/App.tsx        app shell
frontend/src/components/    UI components
frontend/src/lib/api.ts     the only place the UI touches the bindings
```

Backend services are the contract with the frontend. When you change one, regenerate the bindings so the TypeScript side stays in sync (`wails3 generate bindings`, or see `dev-docs/backend-notes.md`).

## Conventions

**Go backend**

- Format with `gofmt`/`go vet` before committing.
- One package = one concern (chunker, parser, indexer, ...). Keep packages small and independently testable.
- Add defensive guards for hand-edited input — the settings UI clamps values, but `config.json` can be edited by hand (see the bounds checks in `chunker.SplitIntoWindows`).
- Every package should have tests. Table-driven tests preferred. Integration tests that need the model or real files should skip gracefully when unavailable (see `indexer` integration tests).
- No external dependencies unless the stdlib genuinely can't do it.

**Frontend**

- SolidJS patterns: signals over mutable state, components stay focused.
- TypeScript strict. Run `tsc` (via `npm run build:dev` or `npm run build`) to type-check.
- Keep the UI keyboard-first: search jump (⌘K / Ctrl K), arrow-key navigation. New interactive elements should be reachable and operable without a mouse where it makes sense.
- Follow existing Tailwind v4 usage in `src/` rather than introducing a new styling approach.

**Commit messages**

Write concise, imperative-mood messages that describe the change, not the churn:

```
indexer: prune chunks whose source files are gone

Adds a prune pass that drops rows whose source files no longer exist so
search results don't go stale after deletions.
```

Scope prefixes like `indexer:`, `search:`, `chunker:`, `frontend:` help when scanning history.

## Testing

```bash
go test ./backend/...          # run all backend tests
go test ./backend/chunker/...  # a single package
```

Run the full suite before opening a PR. Frontend changes should be smoke-tested in `task dev` and, if the change is user-visible, verified with a quick manual pass of the affected flow (search, browse, settings, index).

## Pull request process

1. Fork the repo and create a feature branch from `main`.
2. Make your change with focused, well-scoped commits.
3. Run the backend tests and the frontend type-check.
4. Open a PR with a clear description: what changed, why, and how to verify it. Link to related issues if any.
5. Keep the scope tight — a small, correct PR is easier to review than a large one. If a change spans backend and frontend, land the backend contract first.

## Reporting issues

Include:

- What you were doing
- What you expected to happen
- What actually happened
- Your OS and vectile version
- If it's a crash or hang, the backend log output if you can capture it

## License

By contributing, you agree that your contributions are licensed under the same [Apache License 2.0](LICENSE) that covers the project.
