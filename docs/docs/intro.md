---
sidebar_position: 1
---

# Introduction

vectile searches the files already on your machine: notes, documents, books, and code, in one window.

![The Search view](/img/screenshots/search.png)

## What it does

- Reads an Obsidian vault, a folder of documents, a Calibre library, or a code repository.
- Splits each file into passages and turns each passage into numbers.
- Answers every query twice: the exact words, and passages that mean the same thing.
- Keeps it all in one SQLite file inside a folder you can delete.
- Serves the same search to a local AI assistant over MCP.

Indexing, search, and the model run inside the app. No server, no account.

## Pages

| Page | What it does |
|---|---|
| [Search](./features/search) | One query box, filters, results as cards |
| [Library](./features/library) | Every collection, its files, and its chunk counts |
| [Browse](./features/browse) | The chunks of one collection, read in a side pane |
| [Index](./features/indexing) | Incremental and full runs, prune |
| [Models](./features/models) | The `.gguf` file that turns text into numbers |
| [Sources](./features/sources) | What you can point it at, and what it skips |
| [Scanned PDFs](./features/ocr) | The optional Tesseract plugin |
| [Settings](./features/settings) | Model, chunking, search, sources, indexing, Connect |
| [AI assistants](./features/mcp) | A local server for an assistant on the same machine |

## Next

- [Installation](./installation) — a binary for Windows, macOS, or Linux
- [First run](./first-run) — model, source, index, search
- [Contributing](./contributing) — build it yourself
