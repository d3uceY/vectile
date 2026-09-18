---
title: What you can index
description: Every kind of source vectile reads, and every file type it understands.
---

import Shot from '@site/src/components/Shot';

# What you can index

Four kinds of source. Each one becomes a collection, and every collection shows up in Library,
Browse, and Index.

<Shot
  src="screenshots/settings-sources.png"
  alt="Settings, Sources section: documents and code sources, each as a named group with its paths and exclude list"
  caption="Where sources are added. Documents and code are grouped, and each group becomes one collection."
/>

## Obsidian vaults

Point it at a vault folder and it reads your `.md` notes, including the metadata block at the top,
the tags, and the `[[wikilinks]]`. You can find a note through the notes it links to.

Every vault lands in one collection called `obsidian`. Excluded folders let you skip a `.trash`
directory or an attachments folder without moving anything on disk.

## Project folders

Any folder of documents. Each file is read according to its extension, then split by its own
structure where there is one.

| Kind | Extensions |
|---|---|
| Documents | `.md`, `.txt`, `.pdf`, `.docx`, `.dotx`, `.epub`, `.html` |
| Data | `.csv`, `.json`, `.yaml`, `.yml`, `.xml`, `.sql` |
| Office | `.xlsx`, `.pptx` |
| Notebooks | `.ipynb` |
| Scripts | `.sh`, `.bash` |

Excel workbooks become one section per worksheet, PowerPoint decks one section per slide, and
notebooks keep their text cells and code cells apart.

Each group you add gets its own collection, named after the group.

## Code repositories

Give it a git repository, or a folder containing several, and it scans them all.

Code is split at function and class boundaries, so a result is a whole function instead of half of
one. Commit history is a separate source, so you can search for the change rather than the current
file. **Commit history window** sets how far back to walk (six months by default) and **Commit
subject blacklist** skips noisy commits like `chore`.

<Shot
  src="screenshots/settings-indexing.png"
  alt="Settings, Indexing section: auto-reindex, start on login, cloud placeholder handling, and the git history window with a commit subject blacklist"
  caption="How far back to walk history, and which commits to skip."
/>

## Calibre libraries

Point it at a Calibre library folder and it indexes the books: title, author, tags, series,
publisher, description, and the text inside EPUB and PDF files.

Every library lands in one collection called `calibre`. Results carry the book title and author,
so a hit reads like a citation rather than a file path.

## What vectile skips

- **Email and RSS.** The project vectile grew out of read email and RSS databases. vectile does
  not.
- **Scanned PDFs.** A PDF with real text works. A PDF that is photos of pages produces nothing.
- **`.xls` and `.ppt`.** The old binary Office formats are unsupported. Save as `.xlsx` or
  `.pptx` first.
- **Cloud-only files.** A OneDrive or iCloud placeholder with no contents on disk is skipped
  instead of downloaded. [Settings](/docs/settings) can change that.

## Keeping things out

Two exclusion lists, both in Settings. **Excluded folders** applies to one Obsidian vault, by
folder name. **Project exclude folders** applies everywhere; it starts with `node_modules`, and
you can add build output, `.venv`, or anything else you never want searched.
