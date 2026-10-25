---
sidebar_position: 2
---

# Library

Search answers a question. Library shows what is actually in the index.

![The Library view: collections listed with their type, file count, chunk count, and last indexed date, with one collection expanded to show its files](/img/screenshots/library.png)

## Reading a row

Every collection, with how many files it has, how many chunks it holds, and when it was last read. Each row names the source type.

Expand a row for its files, one line each, with type and chunk count.

A **needs reindex** badge appears on a collection that has documents but no usable numbers stored for them, which happens after a model change. **Re-index all** in the Index view is the fix.

## Deleting

Two sizes of delete here, and both change only the index. The files on your disk are never touched.

| Action | Where | What it removes |
|---|---|---|
| Delete a source | Library: the button on a file row | That file's chunks |
| Delete a collection | Library or Browse | Every chunk in the collection |

Both leave your sources in Settings, so the next index run adds the files back. To keep a collection out for good, remove its source in Settings.

Deleting is refused while an index run is in progress. Let the run finish, or cancel it first.

:::note
Deleting does not empty the [query cache](./search#the-query-cache). The cached queries are still correct for everything that remains.
:::
