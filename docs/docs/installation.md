---
sidebar_position: 2
toc_max_heading_level: 2
---

import DownloadList from '@site/src/components/DownloadList';

# Installation

vectile ships as a self-contained binary. Free, Apache 2.0 licensed, no account.

## Download

<DownloadList />

Older versions, release notes, and SHA256 sums are on the [releases page](https://github.com/d3uceY/vectile/releases).

## Install

The Windows installer runs on its own. The portable zip needs no install step: unzip it and run `vectile-windows-amd64.exe`.

On macOS, open the dmg and drag `vectile.app` to **Applications**. On first launch, right-click → **Open**: the build is unsigned, so Gatekeeper needs that one confirmation.

On Linux:

```bash
chmod +x vectile-linux-amd64.AppImage
./vectile-linux-amd64.AppImage
```

```bash
sudo apt install ./vectile-linux-amd64.deb
```

## The first launch will warn you

vectile is not code-signed, so your operating system warns you before opening it. Every build comes from [this repository](https://github.com/d3uceY/vectile); you can read the source.

**Windows.** SmartScreen shows a blue box that says Windows protected your PC. Click **More info**, then **Run anyway**.

**macOS.** Gatekeeper says Apple cannot check the app. Right-click it, choose **Open**, then **Open** again in the dialog. Once is enough.

## Requirements

| | |
|---|---|
| **OS** | Windows 10, macOS 12, or a recent Linux distribution |
| **CPU** | AVX2, meaning a processor from 2013 or later |
| **Internet** | Model download, the OCR plugin, and the launch version check |

## Next

[First run](./first-run) adds a model, points vectile at a folder, and runs the first index.
