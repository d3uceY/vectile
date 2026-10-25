---
sidebar_position: 4
---

# Index

Indexing reads each file, splits it into passages, and stores the text with its numbers. The numbers are what let a later search find a passage that uses other words.

Indexing needs a model, so the buttons stay greyed out until one is loaded.

![The Index view: Index all and Re-index all in the header, then a row per collection with Index new, Re-index all, and Prune](/img/screenshots/index.png)

## Three ways to run it

| Control | What it does | When to use it |
|---|---|---|
| **Index new** (per collection) | Reads only files whose contents changed | Routinely, and after editing files |
| **Re-index all** (per collection) | Forgets the collection and reads everything again | After changing chunk size, overlap, or the model |
| **Index all** (header) | Runs every collection in one job | The first run, or after adding sources |

**Index new** is the one you want most days. A second pass over a large vault takes seconds, because unchanged files are skipped.

## While a run is going

Progress appears on the collection's row: a bar, the file being read, and a count. It survives switching views, closing the window, or reloading.

**Cancel** stops the run within seconds and keeps whatever it already indexed, so the next step is **Index new**. Only one run happens at a time. A desktop notification says when it finishes.

## Keeping it current

**Auto-reindex** in **Settings → Indexing** re-runs an incremental index on a timer. Set the interval in minutes.

**Prune** removes entries whose files are gone: deleted files, removed books, deleted code.

When auto-reindex is off and the last run was more than a day ago, Search shows a note under the filter bar.

![Settings, Indexing section: auto-reindex and its interval, start on login, cloud placeholder handling, the git history window, and the commit subject blacklist](/img/screenshots/settings-indexing.png)

## The tray

Closing the window hides vectile instead of quitting, so it keeps running in the system tray. The tray menu carries the current status, an **Index** submenu with **All Collections** and one entry per collection, **Cancel Indexing** while a run is active, **Show vectile**, and **Quit**.
