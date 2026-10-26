# OCR languages

How the language picker in **Settings → OCR** works, end to end: what a scan
actually needs, where the numbers come from, and how the Go side, the Wails
bindings and the Solid UI join up. Written for the next person who opens this
repo, in the same spirit as `backend-notes.md` and `model-library.md`.

The user-facing version of this is `docs/docs/features/ocr.md`. This file is the
one with the code in it.

## The short version

- Tesseract reads a page with the language files you name on the command line.
  One file per language: `<code>.traineddata` (`eng`, `deu`, `chi_sim`).
- The OCR plugin bundle ships English. Every other language is downloaded on
  demand, one small file at a time, and kept next to the bundle.
- Two lists matter and they are not the same thing:
  - **what the run asks for** — `ocr.languages` in `config.json`
  - **what is on disk** — `OCRState.languages`, read from the `tessdata` folder
- The Settings panel is the gap between those two lists. A code in the first
  list with no file behind it shows as **no data** with a Download button.
- A code with no file is dropped at run time and the run falls back to English,
  so a hand-edited typo cannot make every scan fail.

## Two lists, and why

| | Where it lives | Who writes it | What it means |
|---|---|---|---|
| Asked for | `config.OCR.Languages` (`config.json`) | Settings draft, or a text editor | "read pages with these" |
| On disk | `ocr.Languages()` → `OCRState.Languages` | the filesystem | "these are the files that exist right now" |

Keeping them apart is what lets the UI say *you asked for French, there is no
French data* rather than silently reading English. The run time filter lives in
`backend/ocr/run.go`:

```go
// useLangs narrows langs to what the bundle ships. A config naming a language
// with no data falls back to English rather than failing every page.
func useLangs(langs []string) []string {
	have := make(map[string]bool)
	for _, l := range Languages() {
		have[l] = true
	}
	var out []string
	for _, l := range langs {
		if have[l] {
			out = append(out, l)
		}
	}
	if len(out) == 0 {
		return []string{"eng"}
	}
	return out
}
```

## Where the code lives

```
backend/config/config.go            OCRConfig{Enabled, Languages}; defaults to
                                    ["eng"]; normalizeOCRLanguages on Load
backend/ocr/manifest.go             Dir(), BinPath(), TessdataDir(), Languages()
backend/ocr/install.go              the bundle install: fetch (streaming download
                                    + SHA256SUMS + progress), unpack, run
                                    --list-langs, atomic swap; Sweep
backend/ocr/langs.go                InstallLanguage / RemoveLanguage: one
                                    .traineddata file, .part then rename
backend/ocr/langs_catalog.go        the pickable list: code + display name
backend/ocr/run.go                  useLangs + the actual tesseract command
backend/parser/pdf.go               opts.OCR && ocr.Installed() gate, then
                                    ocr.Run(..., opts.Languages) per page
backend/indexer/indexer.go          pdfOptions: config → parser.PDFOptions
backend/services/ocr.go             OCRService: state, the one-download slot,
                                    progress events, install/remove methods
frontend/bindings/.../ocrservice.ts generated; do not edit
frontend/src/lib/types.ts           OCRState.languages, OCRLanguage, the event
                                    payloads
frontend/src/lib/api.ts             listOCRLanguages / installOCRLanguage /
                                    removeOCRLanguage wrappers
frontend/src/lib/store.tsx          ocrLanguages, ocrLangInstall, the actions and
                                    the ocr:language-* listeners
frontend/src/components/settings/OcrLanguageDialog.tsx   the searchable picker
frontend/src/components/settings/sections/OcrSection.tsx the Languages block
scripts/vectile-stub.mjs            demo catalog + method IDs for the dev server
scripts/dev-stub.mjs                the browser-side download ticker
scripts/vectile-interact.mjs        `ocrInstalled: true` for the docs screenshot
```

## How the pieces connect

### 1. Config decides what to read with

`config.OCR` is two fields:

```go
type OCRConfig struct {
	Enabled   bool     `json:"enabled"`
	Languages []string `json:"languages"` // tesseract codes, joined with "+"
}
```

`config.Load` runs `normalizeOCRLanguages`: trim, lowercase, drop empties,
de-duplicate, and an empty list becomes `["eng"]`. That is the only place bad
input is fixed up, and it happens on read, so a hand-edited file cannot produce
a list the engine will choke on. Codes that name no file are *not* removed here
— that is the run-time fallback's job, and removing them would hide the missing
file from the UI instead of showing it as **no data**.

### 2. Reading a scan

The indexer hands the config to the parser, which hands one page at a time to
the OCR package:

```
indexer.pdfOptions(cfg)            {OCR: cfg.OCR.Enabled, Languages: cfg.OCR.Languages}
  → parser.ParsePDF(path, opts)
      → per page: opts.OCR && ocr.Installed()      // no plugin, no OCR
          → ocr.Run(ctx, image, opts.Languages)
              → tesseract <page.png> stdout -l eng+deu --tessdata-dir <dir>
```

The language list reaches the engine as a single `-l` argument joined with `+`,
and the data folder is passed explicitly with `--tessdata-dir`, so a Tesseract
on the user's PATH is never involved:

```go
args := []string{
	tmp.Name(), "stdout",
	"-l", strings.Join(useLangs(langs), "+"),
	"--tessdata-dir", filepath.ToSlash(TessdataDir()),
}
```

`TessdataDir()` is `<app data>/vectile/plugins/tesseract/5.5.3/tessdata`, next to
the bundled `bin/tesseract`. Nothing sets `TESSDATA_PREFIX`; the flag is the
whole story.

### 3. Getting the data

Two different downloads, deliberately separate:

**The bundle** (`ocr.Install`) is a whole Tesseract build for the platform: it
fetches the release's `SHA256SUMS`, verifies, unpacks beside the target, runs
`--list-langs` to prove the binary works and has English, then renames the
directory into place. All of that is in `backend/ocr/install.go` and predates
this feature.

**One language** (`ocr.InstallLanguage`) is a single file:

```go
dest := LanguagePath(code)          // tessdata/<code>.traineddata
part := dest + ".part"
fetch(ctx, tessdataBase+code+".traineddata", part, "", onProgress)
os.Rename(part, dest)
```

`fetch` is the same streaming downloader the bundle uses, with one change: an
empty expected hash skips the checksum comparison. `tessdataBase` points at
`tesseract-ocr/tessdata_fast`, **pinned to a commit**:

```go
var tessdataBase = "https://raw.githubusercontent.com/tesseract-ocr/tessdata_fast/8741641.../"
```

Upstream publishes the files but no hashes, so there is nothing to verify
against. What keeps the bytes stable is the URL: bumping that commit is a
deliberate act, the same way bumping `ocr.Version` is. The `.part` file means an
interrupted download leaves the old data (or nothing) rather than a truncated
file Tesseract would load and read badly. `ocr.Sweep()` deletes leftover `.part`
files under `tessdata/` at startup.

`RemoveLanguage` deletes the file and refuses `eng`, because English is what the
fallback reaches for. Missing-file removal is not an error.

### 4. The service: one download at a time

`OCRService` (`backend/services/ocr.go`) owns a single slot, `current job`. A job
is either the bundle install (`code == ""`) or one language download:

```go
type job struct {
	active    bool
	code      string // "" for the bundle
	cancel    context.CancelFunc
	lastAt    time.Time
	lastBytes int64

	// progress, replayed by GetOCRState while the bundle downloads
	downloaded int64
	total      int64
	percent    float64
	speed      float64
	err        string
}
```

They share the slot because both write into the same directory and the bundle
install *replaces the whole version folder* — a language download running
underneath it would vanish mid-write. So `beginJob` refuses a second job, and
`InstallOCR` and `InstallOCRLanguage` both route through it:

```go
ctx, err := beginJob(code, total)   // nil, error when a job is already running
go s.runLangInstall(ctx, code)
```

Progress is bookkeeping, not logic, so one helper serves both: `trackProgress`
records the byte count and returns percent and speed, and each caller builds its
own event payload. `end` clears the slot, emits the terminal event, and keeps
`err` only for the bundle, so a failed language download cannot make the plugin
card look broken.

The events, all emitted on the Wails event bus:

| Event | Payload | When |
|---|---|---|
| `ocr:language-progress` | `{code, downloaded, total, percent, speed}` | throttled to ~100 ms by the downloader |
| `ocr:language-complete` | `{code}` | file renamed into place |
| `ocr:language-failed` | `{code, message}` | download, 404, or rename failed |
| `ocr:language-cancelled` | `{code}` | `CancelOCRInstall` while a language job ran |

`CancelOCRInstall` is deliberately shared: it cancels whichever job is in flight
rather than adding a second cancel method for the UI to remember.

The read side is `GetOCRState`, and it is derived from disk on every call:
`stamp()` fills `Installed`, `Dir`, `Languages` (sorted), platform and asset size,
then `Enabled` from config. That is why a plugin installed while the app is
running is picked up by the next page without a restart.

### 5. The UI: rows, switches, the picker

The store holds the two lists and the download in flight:

```ts
const [ocrLanguages, setOCRLanguages] = createSignal<OCRLanguage[]>([]);   // catalog
const [ocrLangInstall, setOCRLangInstall] = createSignal<OCRLanguageProgress | null>(null);
```

`loadOCRLanguages()` fetches the catalog once when the OCR section mounts.
`installOCRLanguage(code)` sets an optimistic `{code, 0, 0, 0, 0}` so the bar
appears before the first progress event, then calls the binding. The four
listeners write progress into `ocrLangInstall`, clear it on a terminal event, and
reload `ocrState` on success — that reload is what flips the row from **no data**
to a switch.

`OcrSection` renders one row per language that is **asked for or on disk**:

```ts
const rows = () =>
  [...new Set([...chosen(), ...installed()])].sort(...)   // English first
```

Each row is decided by `has(code) = installed().includes(code)`:

| On disk | In `ocr.languages` | Row shows |
|---|---|---|
| yes | yes | name, code, `×` (non-English), switch **on** |
| yes | no | name, code, `×`, switch **off** |
| no | yes | name, code, amber **no data**, **Download** |
| no | no | cannot happen — not in `rows()` |

Actions:

- **switch** → `setLanguage(code, on)` writes the draft; the switch flips the
  code in or out of `draft.ocr.languages`. Turning the last one off keeps
  English, so the saved list is never empty.
- **×** → `removeOCRLanguage(code)` deletes the file, then drops the code from
  the draft. The row disappears, because nothing asks for it and nothing is on
  disk.
- **Download** → `installOCRLanguage(code)` for a code that is in config but has
  no file.

`OcrLanguageDialog` is the picker: a text input filtering `store.ocrLanguages()`
by display name or code, one row per language, `installed` chip when the file is
present, and the same `DownloadProgressBar` the bundle install uses. Picking a
language closes the dialog, starts the download if needed, and switches it on.
It reads the catalog from the store rather than a prop because it is a leaf that
needs the same two calls the section does.

### 6. Saving

Language choice goes through the ordinary Settings draft, like every other
control in the section:

```
switch tick  → setOCR({languages}) → settingsDraft → dirty
Save        → IndexService.SetConfig(draft) → Core.UpdateConfig → config.Save
            → the sticky "unsaved" bar clears
```

`config.Save` overlays `existing["ocr"] = cfg.OCR` onto whatever the file
already held, so unknown keys survive. The next index run picks the new list up
because `pdfOptions` reads the published config snapshot per file — no restart,
no re-index required to *select* a language (only to re-read PDFs that were
already processed).

## Why some choices look odd

- **Pinned commit, no checksum.** `tessdata_fast` ships data files but no
  manifests. Verifying against a hash we made up would be theatre; pinning the
  URL is the honest version of the same guarantee. If checksums ever matter, the
  fix is hosting the files beside the bundle release and adding them to its
  `SHA256SUMS`.
- **English cannot be removed, but can be switched off.** Removing the file
  would break the fallback that makes a bad list harmless. Switching it off is
  fine: a run with only `deu` reads German only, and English is used only when
  the list names nothing usable.
- **The catalog is a Go table.** 120-ish codes with display names, no network,
  no locale data to load at startup, and the same list available to any future
  caller (MCP, a CLI). It changes only when `tessdataBase` does.
- **`osd`, `equ`, `frk`, `script/*` are excluded.** Orientation detection and
  equation data are not languages; `frk` is a deprecated alias; the `script/`
  models cover a whole writing system and are not something a person picks per
  language.
- **One download slot, not a queue.** Two jobs at once would race on the same
  directory for no benefit — these files are megabytes, not gigabytes.

## Keeping it current

- **Add a language**: add a line to `languages` in
  `backend/ocr/langs_catalog.go` (code from the pinned commit, English display
  name), then regenerate bindings only if the struct changed. `Catalog()`
  sorts, so placement does not matter.
- **Move to newer data**: bump the commit hash in `tessdataBase`
  (`backend/ocr/langs.go`), then re-check the codes against the new tree.
- **Change where files come from**: `tessdataBase` is the only place a URL is
  built.

## Testing it

```bash
go test ./backend/ocr/... ./backend/config/... ./backend/services/...
cd frontend && npx tsc --noEmit
```

The Go tests use `httptest` for the download (swap `tessdataBase`), a fake
binary for `Installed()`, and a temp app-data dir, so they never touch a real
plugin. The config test covers the normalisation cases.

To see it in a browser without a model or a real Tesseract:

```bash
node scripts/dev-stub.mjs        # http://127.0.0.1:9255
```

The stub answers the three new method IDs, keeps `languages` on the OCR state,
persists `SetConfig`, and `dev-stub.mjs` drives a fake download ticker for both
the bundle and languages so the bars move.

The docs screenshot is `settings-ocr.png`, produced by the capture pipeline in
`scripts/capture.config.json`. Its job carries `"ocrInstalled": true`, which
makes `scripts/vectile-interact.mjs` click Install, wait for the stub's timer,
reload and reopen the section — the Languages block only exists once the plugin
is installed.
