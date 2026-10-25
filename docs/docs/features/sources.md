---
sidebar_position: 6
---

# Sources

Four kinds of source. Each becomes a collection, visible in Library, Browse, and Index.

![Settings, Sources section: documents and code sources, each as a named group with its paths and exclude list](/img/screenshots/settings-sources.png)

## Obsidian vaults

Point it at a vault folder and it reads your `.md` notes, including the metadata block at the top, the tags, and the `[[wikilinks]]`. A note is findable through the notes it links to.

Every vault lands in one collection called `obsidian`. Excluded folders skip a `.trash` directory or an attachments folder without moving anything on disk.

## Project folders

Any folder of documents. Each file is read according to its extension, then split by its own structure where there is one.

| Kind | Extensions |
|---|---|
| Documents | `.md`, `.txt`, `.pdf`, `.docx`, `.dotx`, `.epub`, `.html` |
| Data | `.csv`, `.json`, `.yaml`, `.yml`, `.xml`, `.sql` |
| Office | `.xlsx`, `.pptx` |
| Notebooks | `.ipynb` |
| Scripts | `.sh`, `.bash` |

Excel workbooks become one section per worksheet, PowerPoint decks one section per slide, and notebooks keep their text cells and code cells apart.

## Code repositories

Give it a git repository, or a folder containing several, and it scans them all.

Code is split at function and class boundaries, so a result is a whole function instead of half of one. Commit history is its own source, so you can search for the change rather than the current file. **Commit history window** sets how far back to walk, six months by default, and **Commit subject blacklist** skips noisy commits like `chore`.

## Calibre libraries

Point it at a Calibre library folder and it indexes the books: title, author, tags, series, publisher, description, and the text inside EPUB and PDF files.

Every library lands in one collection called `calibre`. Results carry the book title and author, so a hit reads like a citation rather than a file path.

## What vectile skips

- **Email and RSS.** The project vectile grew out of read email and RSS databases. vectile does not.
- **Scanned PDFs, until you install the plugin.** A PDF with real text works as it is. A PDF that is photos of pages needs the optional [OCR plugin](./ocr). A run says when it finds pages it could not read.
- **`.xls` and `.ppt`.** The old binary Office formats are unsupported. Save as `.xlsx` or `.pptx` first.
- **Cloud-only files.** A OneDrive or iCloud placeholder with no contents on disk is skipped instead of downloaded. [Settings](./settings) can change that.

## Keeping things out

Four skip lists, one per source type, all in [Settings](./settings) under Sources. An entry is a folder or file name, matched whole at any depth, so `node_modules` drops every copy in a tree and `CHANGELOG.md` drops every file with that name further down.

**Project folders** starts with `node_modules`. **Code repositories** adds to a built-in list of build output and lock files, and applies to the file tree only, not to commit history. **Obsidian vaults** is the place for an attachments folder or `.trash`. **Calibre libraries** matches the folder a book sits in, which is its author and title, so one entry drops a whole author.

Nothing here moves or deletes anything on disk.
