---
title: First run
description: From a fresh install to your first useful search result, in about five minutes.
---

import Shot from '@site/src/components/Shot';

# First run

With an empty library, vectile opens a short tour that walks you through adding a folder,
indexing it, and searching it. Once something is indexed, the tour never comes back. This page is
the same path written down.

## 1. Give it a model

A model turns text into numbers, and it is the one piece vectile cannot supply.

Open **Settings → Model**, then either click **Get a model** and download from the catalog, click
**Import model…** and pick a `.gguf` file you already have, or drop a `.gguf` file into the app's
`models` folder.

The plate at the top of the section says what is loaded. [Models](/docs/models) covers the
catalog.

<Shot
  src="screenshots/settings.png"
  alt="Settings, Model section: the loaded model state, the active model dropdown, an Import model button, the embedding batch size, and the Get a model catalog"
  caption="Three ways in, all of them local: the catalog, the import dialog, or a file dropped into the models folder."
/>

## 2. Add a source

Open **Settings → Sources**. It is grouped by the kind of thing you have. Pick the kind, then add
a path by typing it or clicking **Browse**.

| Source type | Add | Becomes |
|---|---|---|
| Project folders | A folder of documents | One collection per group name |
| Code repositories | A repo, or a folder full of repos | One collection per group name |
| Obsidian vaults | A vault folder | The `obsidian` collection |
| Calibre libraries | A library folder | The `calibre` collection |

Nothing is saved until you click **Save** in the header. Until then the header shows an "unsaved"
pill, and leaving Settings asks what to do.

<Shot
  src="screenshots/settings-sources.png"
  alt="Settings, Sources section: project folders and code repositories as named groups, with a Browse button and an exclude list"
  caption="A group becomes a collection, so the name you give the group is the name you see in Library."
/>

## 3. Index

Open the **Index** view. Each collection has **Index new** and **Re-index all**, and the header
has **Index all**.

**Index new** reads only the files whose contents changed. **Re-index all** reads everything again,
which is what you want after changing the chunk size or the model.

Progress shows per collection and the run can be cancelled at any point. See
[Indexing](/docs/indexing).

<Shot
  src="screenshots/index.png"
  alt="The Index view: one row per collection with Index new, Re-index all, and Prune buttons"
  caption="The first run does the work. Every run after it skips files that have not changed."
/>

## 4. Search

Press <kbd>Ctrl</kbd>+<kbd>K</kbd> (<kbd>⌘</kbd>+<kbd>K</kbd> on macOS) from any view, or pick
**Search** in the sidebar. Type the question you would ask out loud, not the file name you wish
you had used.

Results arrive as cards: title, the matching passage with your words picked out, the collection,
and the path. Expand a card to read the whole passage, open the file in its own application, or
reveal it in your file manager.

<Shot
  src="screenshots/search-passage.png"
  alt="A search result expanded to show the full passage, its tags, and Open file and Reveal in folder buttons"
  caption="Expanding a card shows the whole passage and the two buttons that leave the app."
/>

## Where vectile keeps things

| What | Where |
|---|---|
| Your settings | `config.json` |
| The index | `db/vectile.db` |
| Models | `models/*.gguf` |

All of it sits in one `vectile` folder in your app data directory:
`%AppData%\vectile` on Windows, `~/Library/Application Support/vectile` on macOS,
`~/.config/vectile` on Linux. Delete the folder and you are back to a clean install. Delete only
`db/vectile.db` and you lose the index but keep your settings and sources.

## Two defaults worth turning on

In **Settings → Indexing**:

**Auto-reindex** re-runs an incremental index on a timer, every 60 minutes by default, so the
library does not drift. **Start on login** launches vectile with your session; closing the window
then keeps it in the system tray rather than quitting, so search stays one keystroke away.
