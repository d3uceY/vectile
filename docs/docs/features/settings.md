---
sidebar_position: 8
---

# Settings

Three groups: Engine, Library, and Companions. Your edits are a draft until you click **Save**.

## Engine

### Model

The loaded model, its per-model settings, and the catalog. See [Models](./models).

![Settings, Model section: the loaded model state, the active model dropdown, Import model, batch size, the Get a model catalog, and the installed model list](/img/screenshots/settings.png)

### OCR

An optional plugin that reads PDF pages which came back with no text. See [Scanned PDFs](./ocr).

![Settings, OCR section: plugin status and version, what it is for, the download URL, an Install button, an Open release page link, and a switch for using OCR on pages with no text](/img/screenshots/settings-ocr.png)

### Chunking

| Setting | Default | Range | What it does |
|---|---|---|---|
| Chunk size | 500 words | 50 to 1500 | How much text goes into one chunk. |
| Chunk overlap | 50 words | 0 to chunk size minus 1 | How much neighbouring chunks share, so a sentence across a boundary is still findable. |

Chunk size is counted in words, and the ceiling is what the model can read at once.

Changes only apply to new chunks, so run **Re-index all** to re-split the whole collection. An overlap at or above the chunk size is not allowed.

![Settings, Chunking section: chunk size and chunk overlap, each with its range stated next to the field](/img/screenshots/settings-chunking.png)

### Search

| Setting | Default | Range | What it does |
|---|---|---|---|
| Top k | 10 | 1 to 200 | How many results come back. |
| RRF k | 60 | 1 to 200 | How hard the top results are favoured. Lower values favour them more. |
| Vector weight | 0.7 | 0 to 1 | How much the meaning search counts. |
| Full-text weight | 0.3 | 0 to 1 | How much the word search counts. |

Setting one weight to `0` removes that search entirely, which shows what the other one found on its own.

![Settings, Search section: top k, the RRF constant, and separate vector weight and full-text weight fields](/img/screenshots/settings-search.png)

### Cache

The stored query results, with a button to empty them. [Search](./search#the-query-cache) explains when the cache clears itself.

![Settings, Cache section: the number of cached query vectors, the space they occupy, and a Clear cache button](/img/screenshots/settings-cache.png)

## Library

### Sources

Every path vectile reads, grouped by kind. **Documents** holds Obsidian vaults and Calibre libraries; **Code** holds project folders and repositories, each added as a named group. Excluded folder names live here too. [Sources](./sources) explains what each kind becomes.

![Settings, Sources section: project folders and code repositories as named groups with Browse buttons and exclude lists](/img/screenshots/settings-sources.png)

### Indexing

| Setting | Default | What it does |
|---|---|---|
| Auto-reindex | off | Re-run an incremental index on a timer. |
| Auto-reindex interval | 60 minutes | How often, from 1 minute to a week. |
| Start on login | off | Launch vectile with your session. |
| Skip cloud placeholders | on | Skip OneDrive, iCloud, Google Drive, and Synology placeholder files instead of downloading them. |
| Commit history window | 6 months | How far back to index git history. |
| Commit subject blacklist | empty | Skip commits whose subject starts with any of these. |

![Settings, Indexing section: auto-reindex with its interval, start on login, cloud placeholder handling, the git history window, and the commit subject blacklist](/img/screenshots/settings-indexing.png)

## Companions

### Vexter

The pixel chameleon in the sidebar footer, with a switch per moment: idle, searching, indexing. One switch turns all three off.

Vexter is decoration. Turning it off changes nothing about search or indexing.

![Settings, Vexter section: a master disable switch and one switch for each moment Vexter appears, idle, searching and indexing](/img/screenshots/settings-mascot.png)

### Connect

The MCP server that lets a local AI assistant search your library. See [AI assistants](./mcp).

![Settings, Connect section: the running server state, its loopback URL, an enable toggle, a transport choice, a port field, an allow write tools toggle, and the list of vectile tools](/img/screenshots/settings-connect.png)
