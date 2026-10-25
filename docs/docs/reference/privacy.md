---
sidebar_position: 4
---

# Privacy

Your files are read, split, turned into numbers, and stored on your own machine. The index, the model, and your settings live in one app data folder you can delete at any time. No account, no usage tracking.

Three things reach the network.

## The update check

On launch, vectile asks GitHub (`api.github.com`) for the newest release tag, to show an update badge. It sends no identifiers, and the answer is used for one version comparison. If the request fails, the app starts normally. No setting turns it off; block the host and it fails silently.

## Model downloads

A catalog download fetches the `.gguf` from `huggingface.co`; the file bytes come from its CDN (`*.cdn.hf.co`). Import a model you already have, or drop one into the models folder, and nothing is downloaded.

## The OCR plugin

Installing the Tesseract plugin fetches one archive from `github.com` release assets (served from `release-assets.githubusercontent.com`) and the release's `SHA256SUMS`. The archive is rejected unless its hash matches. It happens only on install, like model downloads.

## Everything else is local

Indexing, search, browse, and prune run inside the app against a database file on your disk. No server component, no remote embedding service. Opening a link hands the URL to your default browser; the app never loads the page itself.

## The MCP server

Turning it on in **Settings → Connect** opens a socket on `127.0.0.1`. Loopback traffic cannot leave the machine or be reached from another device.

That socket exposes your library to whichever client you point at it. Connect prints how many collections and chunks are readable; write tools stay off until you allow them. See [AI assistants](../features/mcp).

![Settings, Connect section: the loopback URL, a server running indicator, and a scope line reading how many collections and chunks are readable](/img/screenshots/settings-connect.png)

## What vectile never touches

- Deleting inside vectile removes entries from the index, never files from your disk.
- Cloud placeholder files are skipped rather than downloaded, so opening vectile does not pull your whole OneDrive onto the machine.
- No code path sends your documents anywhere.

## Checking for yourself

The source is public and Apache 2.0. The four places that touch a socket: `frontend/src/lib/update.ts` for the release check, `backend/services/model_download.go` for catalog downloads, `backend/ocr/install.go` for the OCR bundle, and `backend/mcp/service.go` for the loopback listener. Everything else in `backend/` and `frontend/` works against local files and a local database.
