---
title: Settings file reference
description: Every key in config.json, with its type, default, and what it does.
---

# Settings file reference

Everything you change in the Settings view is stored in one plain text file:

```text
<app data>/vectile/config.json
```

vectile writes it every time you press Save. You never have to edit it by hand; this page is here
for when you want to back it up, copy it to another machine, or see what a setting really is.

If you do edit it, keep the quotes and commas as they are and close the app first. A value outside
its allowed range is pulled back to the nearest one.

## Top level

| Key | Type | Default | What it does |
|---|---|---|---|
| `embedding_model` | string | `bge-m3` | Name of the embedding model, for display. |
| `active_model` | string | the default model path | Path to the active `.gguf` file. Managed by the Model section. |
| `embedding_batch_size` | int | `32` | Chunks per call to the model. Mirrored from the active model. |
| `chunk_size_tokens` | int | `500` | Chunk size in words. 50 to 1500. |
| `chunk_overlap_tokens` | int | `50` | Words shared between neighbouring chunks. 0 to chunk size minus 1. |
| `obsidian_vaults` | string[] | `[]` | Vault folders to index. |
| `obsidian_exclude_folders` | string[] | `[]` | Folder names to skip inside vaults. |
| `project_exclude_folders` | string[] | `["node_modules"]` | Folder names skipped anywhere inside project folders. |
| `calibre_libraries` | string[] | `[]` | Calibre library folders. |
| `repositories` | map | `{}` | Collection name to a repo path, or to a folder to scan for repos. |
| `projects` | map | `{}` | Collection name to a list of document paths. |
| `disabled_collections` | string[] | `[]` | Collections skipped during indexing and search. |
| `skip_cloud_placeholders` | bool | `true` | Skip cloud-only placeholder files instead of downloading them. |
| `git_history_in_months` | int | `6` | How far back to walk commit history. 1 to 240. |
| `git_commit_subject_blacklist` | string[] | `[]` | Skip commits whose subject starts with any of these. |

## `search_defaults`

| Key | Type | Default | What it does |
|---|---|---|---|
| `top_k` | int | `10` | Results returned. 1 to 200. |
| `rrf_k` | int | `60` | The constant in the ranking formula. 1 to 200. |
| `vector_weight` | float | `0.7` | Weight of the meaning search. 0 to 1. |
| `fts_weight` | float | `0.3` | Weight of the word search. 0 to 1. |

## `gui`

| Key | Type | Default | What it does |
|---|---|---|---|
| `auto_reindex` | bool | `false` | Re-run an incremental index on a timer. |
| `auto_reindex_interval_minutes` | int | `60` | Minutes between runs. 1 to 10080, which is a week. |
| `start_on_login` | bool | `false` | Launch with your session. |
| `mascot.show_searching` | bool | `true` | Show Vexter while a query runs. |
| `mascot.show_indexing` | bool | `true` | Show Vexter while a library indexes. |
| `mascot.show_nothing` | bool | `true` | Show Vexter when a search comes back empty. |

## `ocr`

| Key | Type | Default | What it does |
|---|---|---|---|
| `enabled` | bool | `true` | Run OCR on pages that come back with no text. |
| `languages` | string[] | `["eng"]` | Language codes to read with. Codes with no matching `.traineddata` file are ignored. |

See [Scanned PDFs](/docs/ocr).

## `mcp`

| Key | Type | Default | What it does |
|---|---|---|---|
| `enabled` | bool | `false` | Start the MCP server with the app. |
| `port` | int | `31123` | Port for the local server. Bound to `127.0.0.1` only. |
| `allow_write` | bool | `false` | Let assistants call `vectile_index` and `vectile_prune`. |

## A full example

```json
{
  "embedding_model": "bge-m3",
  "active_model": "C:\\Users\\you\\AppData\\Roaming\\vectile\\models\\bge-m3-Q4_K_M.gguf",
  "embedding_batch_size": 32,
  "chunk_size_tokens": 500,
  "chunk_overlap_tokens": 50,
  "obsidian_vaults": ["D:\\notes"],
  "obsidian_exclude_folders": [".trash", "attachments"],
  "project_exclude_folders": ["node_modules", "dist", ".venv"],
  "calibre_libraries": ["D:\\Calibre Library"],
  "repositories": {
    "vectile": ["D:\\code\\vectile"],
    "work": ["D:\\code\\work"]
  },
  "projects": {
    "papers": ["D:\\reading"]
  },
  "disabled_collections": [],
  "skip_cloud_placeholders": true,
  "git_history_in_months": 6,
  "git_commit_subject_blacklist": ["chore", "wip"],
  "search_defaults": {
    "top_k": 10,
    "rrf_k": 60,
    "vector_weight": 0.7,
    "fts_weight": 0.3
  },
  "gui": {
    "auto_reindex": false,
    "auto_reindex_interval_minutes": 60,
    "start_on_login": false,
    "mascot": {
      "show_searching": true,
      "show_indexing": true,
      "show_nothing": true
    }
  },
  "mcp": {
    "enabled": false,
    "port": 31123,
    "allow_write": false
  },
  "ocr": {
    "enabled": true,
    "languages": ["eng"]
  }
}
```

## Environment

One environment variable changes behaviour. `VECTILE_EMBED_MODEL` overrides the path vectile looks
in for its model, which is useful when the model lives outside the app data folder.
