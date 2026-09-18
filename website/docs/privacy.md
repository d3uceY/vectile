---
title: Privacy
description: What vectile does with your data, which is nothing, and the two exceptions.
---

import Shot from '@site/src/components/Shot';

# Privacy

Your files are read, split, turned into numbers, and stored on your own machine. The index, the
model, and your settings live in one app data folder you can delete at any time. There is no
account, no usage tracking, and no data sent anywhere.

Two features can reach the network. Both are worth naming exactly.

## The update check

When it starts, vectile asks GitHub for the number of the newest release, so it can tell you when a
newer version exists. The request identifies nothing about you or your library, and the answer is
used for one comparison. If it fails, the app starts normally and says nothing.

## Model downloads

Downloading a model from the catalog fetches it from Hugging Face. That happens only when you press
download: import a model you already have, or drop one into the models folder, and nothing is
downloaded.

## Everything else is local

Searching, indexing, browsing, and pruning run inside the app, against a database file on your disk.
There is no server component and no remote embedding service.

## The MCP server

Turning it on in **Settings → Connect** opens a socket on `127.0.0.1`, the loopback address.
Loopback traffic never leaves the machine and cannot be reached from another device on your network.

What that socket exposes is your library, to whichever client you point at it. The Connect section
prints how many collections and chunks that is, and the write tools stay off until you allow them.
See [AI assistants](/docs/mcp).

<Shot
  src="screenshots/settings-connect.png"
  alt="Settings, Connect section: the loopback URL, a server running indicator, and a scope line reading how many collections and chunks are readable"
  caption="The scope line exists so that turning this on has a size attached to it."
/>

## What vectile never touches

- Deleting inside vectile removes entries from the index, never files from your disk.
- Cloud placeholder files are skipped rather than downloaded, so opening vectile does not pull your
  whole OneDrive onto the machine.
- Nothing is uploaded. No code path sends your documents anywhere.

## Checking for yourself

The source is public and MIT licensed. Rather than take the claims above on trust, read the two
places that use the network: `frontend/src/lib/update.ts` for the release check, and
`backend/services/model_download.go` for catalog downloads. Everything else in `backend/` and
`frontend/` works against local files and a local database.
