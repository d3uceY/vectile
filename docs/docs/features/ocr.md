---
sidebar_position: 7
---

# Scanned PDFs

A PDF holds one of two things: text, or pictures of text. The first kind works with no setup. The second kind is an image, so there is nothing to search until something reads the picture.

That something is Tesseract, an OCR engine. vectile does not ship it.

![The one-time setup dialog titled Read scanned PDFs, marked optional, showing the plugin status, the download address, an Install button, and a Not now button](/img/screenshots/ocr-setup.png)

## Installing it

**Settings → OCR → Install.**

vectile fetches the archive built for your system, checks its hash, unpacks it into its own folder, and runs that copy. A Tesseract you installed yourself is never used.

![Settings, OCR section: plugin status and version, what it is for, the download URL, an Install button, an Open release page link, a switch for using OCR on pages with no text, and a Languages list with English switched on](/img/screenshots/settings-ocr.png)

## After installing

Nothing changes for PDFs you already indexed, because a run skips any file that has not changed since last time. To read them again: **Settings → OCR → Re-index everything**.

That re-reads every file in every collection, so it takes a while on a large one.

After that, text lifted from a scan behaves like any other passage, with the page number it came from.

## Turning it off, or removing it

**Use OCR for pages with no text** stops the reading pass without uninstalling anything. It only ever runs on a page that came back empty.

**Remove** deletes the plugin. Text already indexed stays where it is; only later runs stop reading scans.

## When it cannot help

Some pages defeat any OCR engine: handwriting, dense tables, low resolution photos. Those pages yield nothing, vectile moves on, and the rest of the document is indexed as usual.

## Language data

English ships with the plugin. For scans in another language, open **Settings → OCR → Languages** and use **Add language**: pick one from the list and its data downloads once, a few megabytes, then stays on this machine. Switch a language on to read with it; switch it off to stop. Removing a language deletes its data and leaves English alone.

A scan written in two languages reads best with both switched on. **Save settings** applies the list.

The list is stored in `ocr.languages` in [the settings file](../reference/configuration) either way, so a hand-edited file keeps working. A code with no data behind it cannot be switched on from the panel: it shows as **no data** until its data is downloaded. Hand-edit a code with no data and it is ignored at run time, rather than failing every page.
