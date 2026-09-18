---
title: Library and Browse
description: Look at what vectile indexed, read one passage at a time, and remove what you no longer want.
---

import Shot from '@site/src/components/Shot';

# Library and Browse

Search answers a question. Library and Browse show you what is actually in the index.

## Library

Library lists every collection with how many files it has, how many chunks it holds, and when it
was last read. Each row says what kind of source it is, so a vault and a folder of repos are not
indistinguishable.

Expand a row to see the files inside, one line each, with the file's type and chunk count. The
delete buttons sit on those rows.

A **needs reindex** badge appears on a collection that has documents but no usable numbers stored
for them, which happens after a model change. **Re-index all** in the Index view is the fix.

<Shot
  src="screenshots/library.png"
  alt="The Library view: collections listed with their type, file count, chunk count, and last indexed date, with one collection expanded to show its files"
  caption="Expand a collection to see the files inside it."
/>

## Browse

Browse takes one library at a time and shows its chunks in order, grouped under the file they came
from. Select a chunk and the pane on the right fetches and shows its full text.

Chunks load as you scroll, so Browse stays usable on a library with 200,000 of them, and the file
headers stay pinned so you always know which file you are reading.

Select chunks with the checkboxes, then use the bar above the list to delete them.

<Shot
  src="screenshots/browse.png"
  alt="The Browse view: a chunk stream grouped under sticky file headers, with checkboxes and a reading pane showing the selected chunk's full text"
  caption="Chunks load as you scroll, grouped under their file, with the selected one read in the pane beside them."
/>

## Deleting

Three sizes of delete, and all three change only the index. The files on your disk are never
touched.

| Action | Where | What it removes |
|---|---|---|
| Delete a chunk | Browse: select, then **Delete** | Those chunks and the numbers stored for them |
| Delete a source | Library: the button on a file row | That file's chunks |
| Delete a collection | Library or Index | Every chunk in the collection |

Deleting a source leaves the path in Settings, so the next index run adds the file back. Deleting
a collection removes its configuration too, so it does not come back on the next index or the next
automatic run.

Deleting is refused while an index run is in progress. Let the run finish, or cancel it first.

:::note
Deleting does not empty the [query cache](/docs/search#the-query-cache). The cached queries are
still correct for everything that remains.
:::
