# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

Wails v3 desktop app (`github.com/wailsapp/wails/v3`), SolidJS + TypeScript + Vite. UI styling: Tailwind CSS v4 via `@tailwindcss/vite` (user decision). Fonts self-hosted via @fontsource (desktop app, no runtime CDN). Model engine is the in-process llama.go bindings vendored at `references/Clipcat/third_party/llama-go`; no Ollama, no HTTP embedding calls.

## Users

A single user working locally on their own machine, searching their own accumulated knowledge: notes, documents, books, email, RSS, code, and project folders. Primary job: find the exact thing they half-remember, fast, without leaving the machine or sending data anywhere.

## Product Purpose

It indexes personal knowledge into one local database (vector + full-text hybrid search) and lets the user search, browse, and manage that library through a fast, friendly interface instead of a CLI. Success means the user can search their whole knowledge base in one place, gets useful results in milliseconds, and trusts that nothing left the machine.

## Positioning

Everything runs in one binary: the model loader (llama.go), the indexer, and the search live in-process. No cloud, no API keys, no Ollama dependency. The UI makes a CLI-grade retrieval engine feel like a calm, pleasant, snappy desktop tool.

## Operating Context

- Desktop app for Windows/macOS/Linux via Wails webview.
- Keyboard-first: global focus to search (Cmd/Ctrl+K), keyboard navigation of results and the file tree.
- The user may be mid-task in another app and needs answers fast, so responsiveness (sub-100ms perceived search) and scannability matter more than decoration.
- Long-running operations (indexing a collection) run in the background; the UI shows progress and keeps working.

## Capabilities and Constraints

- Search: hybrid vector + FTS5 with Reciprocal Rank Fusion; filters for collection, source type, path, sender/author, and date range; top-k control.
- Collections: system (obsidian, email, calibre, rss), code repos, and project folders; each with sources and chunk counts.
- Index: add/trigger indexing per source with incremental updates and a force option.
- Prune: drop entries whose original files are gone.
- Browse: file tree of collections → sources → documents.
- Status/health: database stats (collections, sources, chunks, size, last indexed) and model-engine state.
- Settings: model, chunking, paths, search defaults, disabled collections, OCR, GUI options.
- Constraint (confirmed): the model loader lives in the vectile binary via vendored llama-go. The UI health indicator reflects an in-process model engine (loaded / idle / failed), never a remote service.
- Decided: the index and search are a native Go implementation inside vectile. The backend is wired end to end; the UI talks to it through generated Wails bindings.

## Brand Commitments

- Product name: "vectile".
- User-pinned visual constraints (binding): minimalist; bright background that text reads on (white-family paper); tasteful quirk; one very light, friendly green accent; dot-pattern and grid-pattern backgrounds; a file-tree component in the app; component borders in the minimalist hairline style; fast, snappy feel; accessible, visible, legible text with clear font hierarchy; no AI-slop writing patterns anywhere in copy.

## Evidence on Hand

- Feature/type reference: `references/local-rag-main` (SearchResult, Filters, collection/source/document model, index/prune/status/config surfaces).
- Model engine: `references/Clipcat/third_party/llama-go` (vendored llama.go with prebuilt static libs).
- No real user content is present in this repo; the UI ships with labeled synthetic/mock data until real indexing is wired.

## Product Principles

1. Local-first and private: the interface must never imply or reach for a cloud service.
2. Fast is a feature: results and navigation should feel instant; motion and loading are tuned for snappiness, not spectacle.
3. Scanability before expression: a search result's title, snippet, and source path are readable at a glance; quirk never obscures a task or a state.
4. Honest copy: controls name their action, errors name the problem and the recovery, and no marketing-slop phrasing.
5. Single accent, disciplined palette: one friendly green accent on a bright paper ground, used consistently and sparingly.

## Accessibility & Inclusion

- Desktop UI; target WCAG AA contrast for body text on paper.
- Full keyboard navigation; visible focus; honor `prefers-reduced-motion`.
- Legible font hierarchy with real size/weight steps, not styling tricks.
