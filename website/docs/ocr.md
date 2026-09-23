---
title: Scanned PDFs
description: Install the OCR plugin so vectile can read PDFs that are photos of pages.
---

import Shot from '@site/src/components/Shot';

# Scanned PDFs

A PDF holds one of two things: text, or pictures of text. The first kind works in vectile with no
setup. The second kind, a scan or a photo of a document, is an image, so there is nothing in the
file to search until something reads the picture.

That something is Tesseract, an OCR engine. vectile does not ship it, because most libraries have
no scanned PDFs and nobody should download 20 MB they will not use.

<Shot
  src="screenshots/ocr-setup.png"
  alt="The one-time setup dialog titled Read scanned PDFs, marked optional, showing the plugin status, the download address, an Install button, and a Not now button"
  caption="The offer appears once, after the model step. Not now is a real answer and it will not ask again."
/>

## Installing it

**Settings → OCR → Install.**

The card states what it does, the version, the file size, and the exact address the download comes
from. vectile fetches the archive built for your system, checks it, unpacks it into its own folder,
and runs that copy. A Tesseract you installed yourself is never used.

<Shot
  src="screenshots/settings-ocr.png"
  alt="Settings, OCR section: plugin status and version, what it is for, the download URL, an Install button, an Open release page link, and a switch for using OCR on pages with no text"
  caption="Status, source, and the switch, in one card. Remove sits in the same place once it is installed."
/>

## After installing

Nothing changes for PDFs you already indexed, because a run skips any file that has not changed
since last time. To read them again:

**Settings → OCR → Re-index everything.** That re-reads every file in every library, so it takes a
while on a large one. Nothing is deleted, and each file's passages are replaced as it is re-read.

After that, text lifted from a scan behaves like any other passage. It is split into chunks,
embedded, and comes back in search results with the page number it came from.

## Turning it off, or removing it

**Use OCR for pages with no text** stops the reading pass without uninstalling anything. It only
ever runs on a page that came back with no text at all, so a page that already has a text layer is
never re-read.

**Remove** deletes the plugin. Text already indexed stays where it is. Only later runs stop reading
scans.

## When it cannot help

Some pages defeat any OCR engine: handwriting, dense tables, low resolution photos. Those pages
yield nothing, vectile moves on, and the rest of the document is indexed as usual.

If a run says pages had no readable text and the plugin is already installed, then the PDF is the
limit rather than the setup.

## Language data

The bundle includes English. If your scans are in another language, add its code to `ocr.languages`
in [the settings file](/docs/configuration) and drop the matching `.traineddata` file into the
plugin's `tessdata` folder. A code with no data behind it is ignored, so a typo falls back to
English instead of failing every page.
