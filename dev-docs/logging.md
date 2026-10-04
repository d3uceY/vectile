# Logging

Where vectile writes its logs, why it needs to, and how to read them.

## The problem logging solves

vectile ships as a Windows **windowed** program, built with `-H windowsgui`.
That flag is what gives it a normal app window and no black console.

The catch: a windowed program has no console, so anything written to `stdout`
or `stderr` goes nowhere. Before this was fixed, the backend had 106 log calls
and none of them produced output a developer could see. A bug that logged a
clear error still looked like silence.

That is not a small problem. It meant a broken release could only be diagnosed
by rebuilding it and adding print statements, which is exactly what happened in
[`release-pdf-fix.md`](release-pdf-fix.md).

## What it does now

`backend/applog` opens a log file at startup and points Go's logging system at
it. Everything the backend logs lands there.

| | |
|---|---|
| Location | `<app-data>/vectile.log` |
| Windows path | `%APPDATA%\vectile\vectile.log` |
| Rotation | renamed to `vectile.log.1` once it passes 2 MB |
| Level | `INFO` by default, `DEBUG` when `VECTILE_DEBUG` is set |

Two files at 2 MB each is the most it will ever use on disk.

## How it is wired

In `main.go`, in order:

```go
dataDir, err := appdata.Init()
// ...
if err := applog.Init(dataDir); err != nil {
    log.Printf("log file unavailable, continuing without it: %v", err)
}
defer applog.Close()
```

`applog.Init` also redirects the standard library `log` package to the same
file. That matters because the startup failures worth reading most, config load
and database open, used `log.Fatalf` and were equally silent.

Failing to open the log is deliberately not fatal. A read-only data directory
should not stop the app from starting.

## Turning on debug logging

Set `VECTILE_DEBUG` to any non-empty value before launching:

```powershell
$env:VECTILE_DEBUG = "1"
.\bin\vectile.exe
```

Debug is off by default because some of it is very chatty. Per-page PDF text
extraction and PDFium's own output both log at debug, and a normal index run
would bury the warnings that matter.

## How to read a log

Each line carries a timestamp, a level, and the source location that produced
it:

```
time=2026-10-04T18:03:38Z level=ERROR source=vectile/backend/parser/pdf.go:99
  msg="failed to get PDF pool instance"
  err="could not instantiate webassembly module: GetFileType /dev/stdout: The handle is invalid."
```

The `source=` field is the useful part. It names the file and line, so a warning
leads straight to the code that produced it.

To find only problems in a long log:

```powershell
Select-String -Path "$env:APPDATA\vectile\vectile.log" -Pattern "level=ERROR|level=WARN"
```

## A healthy run looks like this

```
level=INFO  msg="vectile starting" version=v0.7.0
level=INFO  msg="active model" dims=384 rebuild=false
level=INFO  msg="project indexer: found files" count=5 collection=operator-pdf
```

Indexing both starts and finishes with no warning in between. A run that names
files with a `WARN` reason is a run that could not read them.

## Related counters

The log and the run summary work together. `IndexResult` has separate counters
for the two ways a file can end up out of the index:

- `Skipped` means already indexed and unchanged. Expected output of re-indexing
  a collection you have not touched.
- `Failed` means the file was read and produced no usable text. The reason is in
  `ErrorMessages`, and the same reason is in the log with its source line.

Keeping these apart is what stops a broken run from looking like a healthy one.
They used to share a single counter, which is how a total failure once reported
"0 new" with no hint that anything was wrong.

## Rules for working on this

1. **Log at the right level.** Anything a user might act on is `Warn` or
   `Error`. Anything only useful while debugging is `Debug`.
2. **Include the path.** A warning about a file that does not name the file is
   not much of a warning.
3. **Do not log secrets or file contents.** Paths, sizes, counts, and error
   strings are fine. Chunk text is not.
4. **Never call `fmt.Println` or write to `os.Stdout` directly in backend code.**
   It goes nowhere in a release. Use `slog`.
