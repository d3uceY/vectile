---
sidebar_position: 3
---

# First run

From a fresh install to a usable search result.

## 1. Give it a model

Three ways in, all in **Settings → Model**: **Get a model** for the catalog, **Import model…** for a `.gguf` you already have, or drop a file into the app's `models` folder. [Models](./features/models) covers the catalog.

![Settings, Model section: the loaded model state, the active model dropdown, Import model, the batch size, the catalog, and the installed models](/img/screenshots/settings.png)

## 2. Add a source

One path per group in **Settings → Sources**.

| Source type | Add | Becomes |
|---|---|---|
| Project folders | A folder of documents | One collection per group name |
| Code repositories | A repo, or a folder full of repos | One collection per group name |
| Obsidian vaults | A vault folder | The `obsidian` collection |
| Calibre libraries | A library folder | The `calibre` collection |


![Settings, Sources section: project folders and code repositories as named groups, with Browse buttons and an exclude list](/img/screenshots/settings-sources.png)

## 3. Index

**Index new** reads only the files that changed. **Re-index all** reads everything again, which is what a chunk size or model change needs. See [Indexing](./features/indexing).

![The Index view: Index all and Re-index all in the header, then a row per collection with Index new, Re-index all, and Prune](/img/screenshots/index.png)

## 4. Search

Press <kbd>Ctrl</kbd>+<kbd>K</kbd> (<kbd>⌘</kbd>+<kbd>K</kbd> on macOS).

![A search result expanded to show the full passage, its tags, and the Open file and Reveal in folder buttons](/img/screenshots/search-passage.png)

## Where vectile keeps things

| What | Where |
|---|---|
| Your settings | `config.json` |
| The index | `db/vectile.db` |
| Models | `models/*.gguf` |
| The OCR plugin, if installed | `plugins/tesseract` |

All of it sits in one `vectile` folder in your app data directory: `%AppData%\vectile` on Windows, `~/Library/Application Support/vectile` on macOS, `~/.config/vectile` on Linux.

## Two defaults worth turning on

In **Settings → Indexing**:

**Auto-reindex** re-runs an incremental index on a timer, every 60 minutes by default, so the library does not drift. **Start on login** launches vectile with your session; closing the window then hides it in the tray rather than quitting.
