---
title: Search
description: How a query is answered, what the score means, and how to narrow the results.
---

import Shot from '@site/src/components/Shot';

# Search

Search is the home screen: one box, a row of filters, results as cards. Press
<kbd>Ctrl</kbd>+<kbd>K</kbd> (<kbd>⌘</kbd>+<kbd>K</kbd> on macOS) from anywhere and the cursor is
already in the box.

Type the question you would ask out loud, not the file name you wish you had used.

<Shot
  src="screenshots/search.png"
  alt="The Search view: a query box with a Ctrl K hint, a filter row, and result cards with highlighted snippets, collection chips, and monospaced file paths"
  caption="Each card carries the title, the matching passage, the collection, and the path."
/>

## Every query runs twice

**Word search** matches the exact words you typed. It is why pasting an error message turns up the
file that quotes it.

**Meaning search** uses the model to compare your query against every passage in the library by
subject rather than by wording. A passage about blue-green deploys comes back for "how do we ship
changes safely" even though it never says "ship".

The two searches return two lists, and a passage near the top of both wins. The defaults favour
meaning over exact words, and both weights are editable in **Settings → Search**.

<Shot
  src="screenshots/settings-search.png"
  alt="Settings, Search section: top k, the RRF constant, and separate vector and full-text weight fields"
  caption="Both weights live in Settings, along with how many results to return."
/>

## Reading a result

A card carries the title, the matching passage with your query words picked out, the collection,
and the path. Two controls sit on it: the **rank toggle**, which swaps `#1` for the score as a
percentage, and **Read full passage**, which opens the whole passage and reveals **Open file** and
**Reveal in folder**.

<Shot
  src="screenshots/search-passage.png"
  alt="A result card expanded to show the full passage, its tags, and the Open file and Reveal in folder actions"
  caption="An expanded card. The rank, the collection, and the path stay visible while you read."
/>

## Filters

| Filter | Matches |
|---|---|
| Collection | One collection you configured |
| Source type | Markdown, PDF, DOCX, EPUB, HTML, plain text, code, commits, book metadata |
| Path | Part of the file path, ignoring case. `backend/services` limits results to that folder. |
| Author | The author of a book, for Calibre results |
| Date range | Documents dated between the two days you set |
| Top k | How many results to return. Defaults to the value in Settings. |

The text filters wait until you stop typing before they search.

## Why you got nothing

- **No collections yet.** Nothing is configured, and the link goes to Settings.
- **Collections but no chunks.** Sources are configured but never indexed, and the link goes to
  Index.
- **Indexed, and still nothing.** Nothing matches. Example queries are offered as a start.

## The query cache

Turning a query into numbers is the slowest part of a search, so vectile remembers the ones it has
already done. Searching the same words again returns almost instantly, with a "cached" label next
to the result count.

The cache empties itself after an index run, a prune, or a model change.
**Settings → Cache** shows what it holds and can empty it.

<Shot
  src="screenshots/settings-cache.png"
  alt="Settings, Cache section: the number of cached query vectors, the space they take, and a Clear cache button"
  caption="Clearing the cache costs nothing except turning repeated queries into numbers again."
/>

## When the model is missing

The meaning search is skipped and the word search still answers. Exact matches keep working, which
can look like a healthy search until you ask a question that shares no words with the answer.
