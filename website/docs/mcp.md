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
| `vectile_search` | No | The same search as the app, with the same filters. |
| `vectile_list_collections` | No | Every collection with source and chunk counts and last-indexed time. |
| `vectile_collection_info` | No | Detail on one collection: source types, counts, and sample titles. |
| `vectile_index` | Yes | Runs indexing for one collection and waits for it to finish. |
| `vectile_prune` | Yes | Removes entries whose files no longer exist. |

The read tools are always available. The two write tools stay off until you switch on **Allow
write tools**, so an assistant cannot start a long index of your library without you saying so in
the app first.

<Shot
  src="screenshots/settings-connect.png"
  alt="Settings, Connect section: the running server state, the loopback URL with a copy button, an enabled toggle, the port field, an allow write tools toggle, and the list of vectile tools with write tagged"
  caption="The status plate answers for the saved server and for the changes you have not saved yet."
/>

### Search parameters

`vectile_search` takes `query` plus any of the app's filters:

| Parameter | Example |
|---|---|
| `collection` | `obsidian` |
| `top_k` | `20` |
| `source_type` | `markdown`, `pdf`, `docx`, `epub`, `html`, `plaintext`, `code`, `commit`, `calibre-description` |
| `path` | `backend/services` |
| `date_from` / `date_to` | `2026-01-01` |
| `author` | An author name, for book results |
| `metadata_filter` | A JSON object of key and value pairs |

## Connecting a client

Claude Desktop, in `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "vectile": {
      "url": "http://127.0.0.1:31123/sse"
    }
  }
}
```

Claude Code:

```bash
claude mcp add vectile --transport sse http://127.0.0.1:31123/sse
```

Anything else that speaks SSE:

```text
http://127.0.0.1:31123/sse
```

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
