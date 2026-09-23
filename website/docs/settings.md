---
title: Settings
description: What each section of the Settings view controls.
---

import Shot from '@site/src/components/Shot';

# Settings

Settings is a rail of eight sections in three groups: Engine, Library, and Companions.

Your edits are a draft until you click **Save** in the header, which is why the header shows an
"unsaved" pill. Leaving Settings with unsaved changes asks whether to save, leave, or keep
editing.

## Engine

### Model

The state of the loaded model, the active model dropdown, **Import model…**, that model's
settings, the catalog under **Get a model**, and the installed models. See
[Models](/docs/models).

<Shot
  src="screenshots/settings.png"
  alt="Settings, Model section: the loaded model state, the active model dropdown, Import model, batch size, the Get a model catalog, and the installed model list"
  caption="State, active model, and catalog on one screen."
/>

### OCR

An optional plugin that reads PDF pages which came back with no text, meaning scans and photos of
pages. The card shows the version, the file size, and the address the download comes from. See
[Scanned PDFs](/docs/ocr).

<Shot
  src="screenshots/settings-ocr.png"
  alt="Settings, OCR section: plugin status and version, what it is for, the download URL, an Install button, an Open release page link, and a switch for using OCR on pages with no text"
  caption="Most libraries never need this. The card says what it costs in size before you click Install."
/>

### Chunking

| Setting | Default | Range | What it does |
|---|---|---|---|
| Chunk size | 500 words | 50 to 1500 | How much text goes into one chunk. |
| Chunk overlap | 50 words | 0 to chunk size minus 1 | How much neighbouring chunks share, so a sentence across a boundary is still findable. |

Chunk size is counted in words, and the ceiling is what the model can read at once.

Changes only apply to new chunks, so run **Re-index all** to re-split the whole library. An overlap
at or above the chunk size is not allowed.

<Shot
  src="screenshots/settings-chunking.png"
  alt="Settings, Chunking section: chunk size and chunk overlap, each with its range stated next to the field"
  caption="Overlap is capped below chunk size, so an out of range value cannot make the splitter loop."
/>

### Search

| Setting | Default | Range | What it does |
|---|---|---|---|
| Top k | 10 | 1 to 200 | How many results come back. |
| RRF k | 60 | 1 to 200 | How hard the top results are favoured. Lower values favour them more. |
| Vector weight | 0.7 | 0 to 1 | How much the meaning search counts. |
| Full-text weight | 0.3 | 0 to 1 | How much the word search counts. |

Setting one weight to `0` removes that search entirely, which shows what the other one found on its
own.

<Shot
  src="screenshots/settings-search.png"
  alt="Settings, Search section: top k, the RRF constant, and separate vector weight and full-text weight fields"
  caption="Top k and the ranking constant above the two weights that decide which list wins."
/>

### Cache

The stored query results, with a button to empty them.
[Search](/docs/search#the-query-cache) explains when the cache clears itself.

<Shot
  src="screenshots/settings-cache.png"
  alt="Settings, Cache section: the number of cached query vectors, the space they occupy, and a Clear cache button"
  caption="Clearing the cache is instant, and costs only the queries you repeat."
/>

## Library

### Sources

Every path vectile reads, grouped by kind. **Documents** holds Obsidian vaults, each with its own
excluded folders, and Calibre libraries. **Code** holds project folders and code repositories,
each added as a named group.

Type a path or click **Browse** for the folder picker. Excluded folder names live here too.
[What you can index](/docs/sources) explains what each kind becomes.

<Shot
  src="screenshots/settings-sources.png"
  alt="Settings, Sources section: project folders and code repositories as named groups with Browse buttons and exclude lists"
  caption="A group becomes a collection, so the name you give the group is the name in Library."
/>

### Indexing

| Setting | Default | What it does |
|---|---|---|
| Auto-reindex | off | Re-run an incremental index on a timer. |
| Auto-reindex interval | 60 minutes | How often, from 1 minute to a week. |
| Start on login | off | Launch vectile with your session. |
| Skip cloud placeholders | on | Skip OneDrive, iCloud, Google Drive, and Synology placeholder files instead of downloading them. |
| Commit history window | 6 months | How far back to index git history. |
| Commit subject blacklist | empty | Skip commits whose subject starts with any of these. |

<Shot
  src="screenshots/settings-indexing.png"
  alt="Settings, Indexing section: auto-reindex with its interval, start on login, cloud placeholder handling, the git history window, and the commit subject blacklist"
  caption="Everything on this card is about keeping the index honest without you thinking about it."
/>

## Companions

### Vexter

The small pixel dinosaur in the sidebar footer. It pops up while a query runs, while a library
indexes, and when a search comes back empty, then lowers itself again. Each moment has its own
switch, and one switch turns all three off. Vexter is decoration: turning it off changes nothing
about how search or indexing behave.

<Shot
  src="screenshots/settings-mascot.png"
  alt="Settings, Vexter section: a master disable switch and one switch for each moment Vexter appears, searching, indexing, and no results"
  caption="Three moments, three switches, and one that turns all of them off."
/>

### Connect

The MCP server that lets a local AI assistant search your library. See
[AI assistants](/docs/mcp).

<Shot
  src="screenshots/settings-connect.png"
  alt="Settings, Connect section: the running server state, its loopback URL, an enable toggle, a port field, an allow write tools toggle, and the five vectile tools"
  caption="The one section that decides whether anything outside vectile can read your library."
/>
