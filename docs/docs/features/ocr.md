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

![Settings, OCR section: plugin status and version, what it is for, the download URL, an Install button, an Open release page link, and a switch for using OCR on pages with no text](/img/screenshots/settings-ocr.png)

## After installing

Nothing changes for PDFs you already indexed, because a run skips any file that has not changed since last time. To read them again: **Settings → OCR → Re-index everything**.

That re-reads every file in every collection, so it takes a while on a large one. Nothing is deleted, and each file's passages are replaced as it is re-read.

After that, text lifted from a scan behaves like any other passage, with the page number it came from.

## Turning it off, or removing it

**Use OCR for pages with no text** stops the reading pass without uninstalling anything. It only ever runs on a page that came back empty.

**Remove** deletes the plugin. Text already indexed stays where it is; only later runs stop reading scans.

## When it cannot help

Some pages defeat any OCR engine: handwriting, dense tables, low resolution photos. Those pages yield nothing, vectile moves on, and the rest of the document is indexed as usual.

If a run says pages had no readable text and the plugin is already installed, the PDF is the limit rather than the setup.

## Language data

The bundle includes English. For scans in another language, add its code to `ocr.languages` in [the settings file](../reference/configuration) and drop the matching `.traineddata` file into the plugin's `tessdata` folder. A code with no data behind it is ignored, so a typo falls back to English instead of failing every page.
