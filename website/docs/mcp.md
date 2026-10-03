---
title: AI assistants (MCP)
description: Let a local AI assistant search your library, over a socket only your computer can reach.
---

import Shot from '@site/src/components/Shot';

# AI assistants (MCP)

MCP, the Model Context Protocol, is a common way for an AI assistant to ask another program for
data. vectile can be that other program, so an assistant on the same computer can search your
library.

The server listens on `127.0.0.1`, the loopback address, so it is reachable from your machine and
from nowhere else. Turn it on in **Settings → Connect**, which shows the live state of the server,
the address to paste into a client, and copyable setup snippets.

## The tools an assistant gets

| Tool | Writes? | What it does |
|---|---|---|
| `vectile_search` | No | The same search as the app, with the same filters. Returns a snippet and a chunk id. |
| `vectile_get_chunk` | No | One result in full, with optional neighbouring chunks. |
| `vectile_read_source` | No | A whole note, book section, or code file, reassembled in order. |
| `vectile_grep` | No | Exact or regular-expression match for identifiers, error strings, and TODOs. |
| `vectile_list_collections` | No | Every collection with source and chunk counts and last-indexed time. |
| `vectile_collection_info` | No | Detail on one collection: source types, counts, and sample titles. |
| `vectile_list_sources` | No | The files indexed in one collection. |
| `vectile_facets` | No | Metadata keys and common values, so filters are not guesswork. |
| `vectile_status` | No | Counts, database size, last indexed time, and the active model. |
| `vectile_find_related` | No | Chunks nearest to one the assistant already found. |
| `vectile_timeline` | No | Documents in date order, newest first. |
| `vectile_index` | Yes | Runs indexing for one collection and waits for it to finish. |
| `vectile_prune` | Yes | Removes entries whose files no longer exist. |

A search result carries a chunk id and a short snippet rather than the whole passage. The assistant reads more with `vectile_get_chunk` (that chunk, or its neighbours) or `vectile_read_source` (the whole note, book section, or file), which keeps its context small.

The read tools are always available. The two write tools stay off until you switch on **Allow
write tools**, so an assistant cannot start a long index of your library without you saying so in
the app first.

<Shot
  src="screenshots/settings-connect.png"
  alt="Settings, Connect section: the running server state, the loopback URL with a copy button, an enabled toggle, a transport choice, the port field, an allow write tools toggle, and the list of vectile tools with write tagged"
  caption="The status plate answers for the saved server and for the changes you have not saved yet."
/>

### Search parameters

`vectile_search` takes `query` plus any of the app's filters:

| Parameter | Example |
|---|---|
| `collection` | `obsidian` |
| `top_k` | `20` |
| `max_chars` | `1200` |
| `source_type` | `markdown`, `pdf`, `docx`, `epub`, `html`, `plaintext`, `code`, `commit`, `calibre-description` |
| `path` | `backend/services` |
| `date_from` / `date_to` | `2026-01-01` |
| `author` | An author name, for book results |
| `metadata_filter` | A JSON object of key and value pairs |

## Connecting a client

vectile speaks Streamable HTTP on `/mcp` by default. That is the transport current assistants
probe first, and most of them find it from the URL alone.

Claude Desktop, in `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "vectile": {
      "url": "http://127.0.0.1:31123/mcp"
    }
  }
}
```

Claude Code:

```bash
claude mcp add vectile --transport http http://127.0.0.1:31123/mcp
```

Anything else that speaks Streamable HTTP:

```text
http://127.0.0.1:31123/mcp
```

## Choosing a transport

**Settings → Connect** has a Transport choice:

- **Streamable HTTP** (the default) answers on `/mcp`.
- **SSE** is the older transport, for an assistant that cannot use Streamable HTTP. It answers on
  `/sse`.

The choice applies when you save, and the server restarts on the new address. The status plate
names the transport that is actually running, so it never disagrees with a change you have not
saved yet.

VS Code and Claude Code are the two clients that record the transport in their own setup. Pick
those tabs in Settings and the snippet already carries the right one, so there is nothing to edit.

If you connected an assistant before vectile moved to Streamable HTTP, paste the setup in again
once. The default address changed from `/sse` to `/mcp`.

## The port

The default is `31123`, and you can change it in **Settings → Connect**. The new port applies when
you save, and the status plate says whether the server is running, stopped, or waiting for that
save. Copy the address shown there into your client.

## What an assistant can see

Everything you have indexed, in every collection you have not disabled. If that is more than you
want, leave the server off, leave it on with write tools off, or disable collections in
**Settings → Sources**, which keeps them out of results in the app and over MCP alike.

The Connect section prints what is exposed (`N collections · M chunks readable`), so the trade-off
has a number attached to it. [Privacy](/docs/privacy) covers the rest.
