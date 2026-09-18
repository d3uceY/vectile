---
title: Troubleshooting
description: What to check when vectile does not do what you expect.
---

# Troubleshooting

## Your OS blocked the installer or the app

vectile is not code-signed, so your operating system asks you to confirm before opening it. On
Windows, click **More info** then **Run anyway**. On macOS, right-click the app and choose **Open**
once. On Linux, the AppImage needs `chmod +x`.

## The index buttons are greyed out

Indexing needs an active model. Open **Settings → Model** and download one from the catalog, import
one, or drop a `.gguf` file into the models folder.

The sidebar shows the model as idle, loaded, or failed. A failed model means the file is there but
could not be read, usually because the download was cut short.

## Search only finds exact words

With no working model, the meaning search is skipped and only the word search answers. Exact
matches keep working, which can look healthy until you ask a question that shares no words with the
answer. Check the model state in the sidebar.

## Nothing comes back at all

1. **No collections configured.** Add a source in **Settings → Sources**.
2. **Configured but never indexed.** Open Index and run **Index new**.
3. **Indexed, but no match.** Try an example query, or loosen the filters.

## A collection shows a "needs reindex" badge

That collection has documents but no usable numbers stored for them, usually after switching to a
model with a different vector width. Run **Re-index all** for it, or **Index all**.

## Deleting is refused

Deletes are blocked while an index run is in progress, on every collection, not just the one being
indexed. Let the run finish, or cancel it from the Index view or the tray.

## A deleted file still shows up

An index run removes entries whose files no longer exist, and the Index view has a **Prune** button
when you do not want to wait. If an entry survives both, the file is still somewhere the indexer
can see it: check for a copy inside another configured folder.

## Indexing looks stuck

A run reports progress per file, so one very large document can sit on the same line for a while
while it is read and converted. **Cancel** responds within seconds. If the bar has not moved and
Cancel does nothing, look at the tray: its status line names the collection being indexed, or the
model state.

## An MCP client cannot connect

- The server has to be enabled in **Settings → Connect** and saved. The status plate says whether
  it is running, stopped, or about to start or stop when you save.
- The port in the client must match the port in the app. Copy the address printed in the Connect
  section.
- The client has to speak SSE. Some clients only speak stdio and cannot use a URL.
- The server listens on `127.0.0.1` only, so a client in a container, a virtual machine, or on
  another computer cannot reach it. That is deliberate.

## A Windows firewall prompt appeared

The MCP server listens on loopback, so denying the prompt does not break the app.

## The app will not start on Linux

The single-instance guard uses the session D-Bus. On a machine without one, such as a bare TTY or a
minimal window manager, vectile refuses to start.

## Starting vectile again does nothing

That is the single-instance guard working: the second launch raises the window of the first instead
of opening a duplicate.

## The first search after launch is slow

The model loads on first use rather than at startup, so the first search pays for loading it and
later ones are fast. Repeating a query is served from the
[query cache](/docs/search#the-query-cache).

## Go tests fail because no model is present

Tests that need real embeddings skip themselves when the model file is missing. To run them, put
`bge-m3-Q4_K_M.gguf` in the models folder or point `VECTILE_EMBED_MODEL` at one.
