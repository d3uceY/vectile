---
sidebar_position: 1
---

# Search

Press <kbd>Ctrl</kbd>+<kbd>K</kbd> (<kbd>⌘</kbd>+<kbd>K</kbd> on macOS) from anywhere and the cursor is already in the box.

![The Search view: a query box with a Ctrl K hint, a filter row, and result cards with highlighted snippets, collection chips, and monospaced file paths](/img/screenshots/search.png)

## Every query runs twice

**Word search** matches the exact words you typed. Pasting an error message turns up the file that quotes it.

**Meaning search** compares your query against every passage by subject rather than wording. A passage about blue-green deploys comes back for "how do we ship changes safely" even though it never says "ship".

The two lists are blended, and a passage near the top of both wins. The defaults favour meaning, and both weights are editable in **Settings → Search**.

## Reading a result

A card carries the title, the matching passage with your query words picked out, the collection, and the path.

The **rank toggle** swaps `#1` for the score as a percentage. **Read full passage** opens the whole passage and reveals **Open file** and **Reveal in folder**.

![A result card expanded to show the full passage, its tags, and the Open file and Reveal in folder actions](/img/screenshots/search-passage.png)

## Filters

| Filter | Matches |
|---|---|
| Collection | One collection you configured |
| Source type | Markdown, PDF, DOCX, EPUB, HTML, plain text, code, commits, book metadata |
| Path | Part of the file path, ignoring case. `backend/services` limits results to that folder. |
| Author | The author of a book, for Calibre results |
| Date range | Documents dated between the two days you set |
| Top k | How many results to return. Defaults to the value in Settings. |

## Why you got nothing

- **No collections yet.** Nothing is configured, and the link goes to Settings.
- **Collections but no chunks.** Sources are configured but never indexed, and the link goes to Index.
- **Indexed, and still nothing.** Nothing matches. Example queries are offered as a start.

## The query cache

Turning a query into numbers is the slowest part of a search, so vectile remembers the ones it has already done. The same words again return almost instantly, with a **cached** label next to the result count.

The cache empties itself after an index run, a prune, or a model change. **Settings → Cache** shows what it holds and can empty it.

![Settings, Cache section: the number of cached query vectors, the space they take, and a Clear cache button](/img/screenshots/settings-cache.png)

## When the model is missing

Search is ranked by meaning, which needs an embedding model, so vectile does not pretend otherwise: with no model active the box stays closed and the view offers **Add a model**, straight into **Settings → Model**. Choosing one there turns search back on, no restart.
