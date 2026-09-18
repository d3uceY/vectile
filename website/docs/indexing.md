---
title: Indexing
description: Incremental and full runs, cancelling, pruning, and keeping the index current.
---

import Shot from '@site/src/components/Shot';

# Indexing

Indexing is the step that makes your files searchable. vectile reads each file, splits it into
passages, turns each passage into numbers, and writes both the numbers and the text into its
database. The numbers are what let a later search find a passage that uses different words.

Indexing needs a model, so the index buttons stay greyed out until one is loaded.

## Three ways to run it

| Control | What it does | When to use it |
|---|---|---|
| **Index new** (per collection) | Reads only files whose contents changed | Routinely, and after editing files |
| **Re-index all** (per collection) | Forgets the collection and reads everything again | After changing chunk size, overlap, or the model |
| **Index all** (header) | Runs every collection in one job | The first run, or after adding sources |

**Index new** is the one you want most days. A second pass over a large vault takes seconds, because
unchanged files are skipped. You never need to delete a collection and add it back; **Re-index all**
covers every case where the stored data is out of date.

<Shot
  src="screenshots/index.png"
  alt="The Index view: Index all and Re-index all in the header, then a row per collection with Index new, Re-index all, and Prune"
  caption="One row per collection, with an incremental run and a full run for each."
/>

## While a run is going

Progress appears on the collection's row: a bar, the file being read, and a count. It survives
switching views, closing the window, or reloading.

**Cancel** stops the run within seconds and keeps whatever it already indexed, so the next step is
**Index new**. Only one run happens at a time. A desktop notification tells you when it finishes.

## Keeping it current

Press the button when you want to. That always works.

**Auto-reindex** in **Settings → Indexing** re-runs an incremental index on a timer. Set the
interval in minutes.

**Prune** deals with files you deleted. Their passages are still in the index, and Prune removes
entries whose files are gone: deleted files, removed books, deleted code. Run it from the Index
view.

When auto-reindex is off and the last run was more than a day ago, Search shows a note under the
filter bar.

<Shot
  src="screenshots/settings-indexing.png"
  alt="Settings, Indexing section: auto-reindex and its interval, start on login, cloud placeholder handling, the git history window, and the commit subject blacklist"
  caption="Auto-reindex, start on login, and the code-indexing settings, all on one card."
/>

## The tray

Closing the window hides vectile instead of quitting, so it keeps running in the system tray. The
tray menu carries the current status, an **Index** submenu with **All Collections** and one entry
per collection, **Cancel Indexing** while a run is active, **Show vectile**, and **Quit**.

Quit lives in the tray, not on the window's close button.
