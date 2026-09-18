---
title: What is vectile?
description: A private search engine for the files already on your computer.
---

import Shot from '@site/src/components/Shot';

# What is vectile?

vectile searches the files you already own. It reads your notes, documents, books, and code,
writes a searchable copy of them into a database on your computer, and searches that copy in a
window you can leave open.

Every query runs twice: once for the exact words you typed, once for passages that mean the same
thing in other words. The two result lists are blended into one. A search for "how do we ship
changes safely" can land on a note about blue-green deploys that never says "ship".

Nothing leaves your machine. No server, no account, no API key, no tracking. The model that reads
your text runs inside the app, from a file on your disk.

<Shot
  src="screenshots/search.png"
  alt="The Search view: a query for kubernetes rollout, with ranked result cards showing highlighted snippets, collection chips, and file paths"
  caption="The highlit words in this snippet are the query, not the document's own wording."
/>

## What you can point it at

- an Obsidian vault
- a folder of documents: Markdown, PDF, DOCX, XLSX, PPTX, notebooks, and more
- a Calibre library of ebooks
- a code repository, including its commit history

Each source becomes a collection, which is what vectile calls one group of files. The first index
reads everything; every run after that skips files that have not changed.

## Five views

| View | What it is for |
|---|---|
| **Search** | The home screen: one query box, filters, results as cards. |
| **Library** | Every collection with its file and chunk counts, expandable to the files inside. |
| **Browse** | The chunks of one library, grouped by file, read in a side pane. |
| **Index** | Run an index, watch progress, delete what you no longer want. |
| **Settings** | Sources, model, chunking, search defaults, scheduling, MCP. |

A chunk is one passage, a few hundred words long, so a result is the paragraph that answers you
rather than a whole 40 page PDF.

<div className="doc-shots">
  <Shot
    src="screenshots/library.png"
    alt="The Library view: collections expanded to show their source files, with chunk counts and last indexed dates"
    caption="Library. Every collection, what is in it, and when it was last read."
  />
  <Shot
    src="screenshots/index.png"
    alt="The Index view: a row per configured collection with Index new, Re-index all, and Prune controls"
    caption="Index. Incremental and full runs, per collection or all at once."
  />
</div>

It is built for the moment you remember the note but not what you named it, and it takes as a
given that finding a file should not cost you your privacy.

## Where to start

[Download vectile](/docs/download), then follow [First run](/docs/first-run).
