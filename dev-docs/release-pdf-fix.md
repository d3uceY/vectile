# Why the released app indexed nothing

**Date:** 2026-10-04
**Affects:** every Windows release up to and including `v0.7.0`
**Status:** fixed, not yet released

## What users saw

Install the app from a release, point it at a folder of PDFs, press Index. The
run finishes and reports **0 new**. No error, no crash, no explanation. The
PDFs are simply not there.

The same code built and run from a terminal worked perfectly.

## The short version

The app ships as a **Windows windowed program** (built with `-H windowsgui`),
which means it has no console. Some of the libraries inside it assume a console
exists, and one of them died quietly when it could not find one.

## How PDFs get read

vectile does not read PDFs itself. It uses PDFium, a small program bundled
inside the exe and written in a different language. To run it, vectile starts a
tiny virtual machine, loads PDFium into it, and asks it to turn each page into
text.

Before PDFium reads anything, it sets up two output pipes:

- `stdout`, where normal output goes
- `stderr`, where errors go

Normally both point at a console window. That is where the trouble starts.

## The bug

`go-pdfium`, the Go library that wraps PDFium, does this by default:

```go
if config.Stdout == nil { config.Stdout = os.Stdout }
if config.Stderr == nil { config.Stderr = os.Stderr }
```

`os.Stdout` means "the console I was started from". It then hands those pipes
to the virtual machine, which tries to open them by the names `/dev/stdout` and
`/dev/stderr`.

A `-H windowsgui` program has no console. Those pipes are not empty. They are
**invalid**, like a phone number that was never assigned. So the virtual machine
fails with:

```
could not instantiate webassembly module:
GetFileType /dev/stdout: The handle is invalid.
```

Every PDF failed on that line.

## The detail that hid it

The failure happened at the wrong moment.

Setting up the pool of virtual machines worked fine. The error came later, when
vectile actually asked for a machine to read a PDF. So the app looked healthy
right up until it silently produced nothing.

That combination (healthy setup, failure at use time, no error shown) is what
made this hard to find.

## Why a terminal run worked

Launching an app from a terminal **gives it a console**, and Windows hands it
working pipes as part of the deal.

| How it was started | Console? | Pipes valid? | Result |
|---|---|---|---|
| `go run .` or `.\bin\vectile.exe` from a terminal | yes | yes | works |
| Double-clicking the installed app | no | no | fails silently |

So "dev works, release does not" was never about the build. Both builds had the
same code. The difference was **how the program was launched**.

## The fix

Give PDFium our own pipes instead of letting it reach for the console's.

```go
pdfPool, pdfPoolErr = webassembly.Init(webassembly.Config{
    MinIdle:  1,
    MaxIdle:  1,
    MaxTotal: 1,
    Stdout:   logWriter{},
    Stderr:   logWriter{},
})
```

`logWriter` accepts anything PDFium writes and forwards it to the app log at
debug level. It always reports success, because it never touches a console. Set
`VECTILE_DEBUG=1` to see what PDFium says.

The regression tests are in `backend/parser/pool_stdio_test.go`.

## Why this took three separate fixes to find

Each one was needed before the next clue appeared. Worth reading in order,
because the same shape will recur elsewhere.

### 1. Nothing was logged anywhere

The app is a windowed binary, so its error output went nowhere. It also never
told Go's logging system where to write, so all 106 log calls across the
backend were discarded.

Fixed by `backend/applog`, which writes everything to
`<app-data>/vectile.log` and rotates it at 2 MB. See
[`logging.md`](logging.md).

**Without this, there was nothing to read.**

### 2. The error was invisible even once logging worked

Once the log existed, the real message appeared immediately:

```
ERROR parser/pdf.go:99  failed to get PDF pool instance
  err="could not instantiate webassembly module:
       GetFileType /dev/stdout: The handle is invalid."
```

Fixed by the explicit pipes above.

### 3. The app used one word for two different things

When a file produced no text, vectile counted it as `skipped`.

But "skipped" should mean "already up to date, nothing to do", which is healthy.
What actually happened was "I tried to read this and failed", which is not.

One word, opposite meanings. So a total failure looked exactly like a normal
no-op run.

Fixed by splitting the counter in two:

| Counter | Meaning |
|---|---|
| `Skipped` | already indexed and unchanged. Nothing to do. Healthy. |
| `Failed` | read the file, extracted nothing. Something is wrong. |

`Failed` now reaches the UI, in the toast and in the Index page summary. It is
covered by `backend/indexer/failcount_test.go`.

## What to check if this happens again

If indexing reports nothing and you are not sure why:

1. **Read the log.** `<app-data>/vectile.log`, which on Windows is
   `%APPDATA%\vectile\vectile.log`. Start here.
2. **Look at the split.** A run that reports `N skipped, 0 failed` is healthy.
   A run that reports `N failed` found files it could not read, and the log
   names them.
3. **Check how the app was launched.** Anything started from a terminal has a
   console and can mask bugs that depend on one. Test by double-clicking the
   installed exe, the way a user would.

## Rules for working on this

1. **Never let a bundled library use the process std handles.** If a library
   defaults `Stdout` or `Stderr` to `os.Stdout` / `os.Stderr`, override it.
   Windowed builds have no console and those handles are invalid, not empty.
2. **Check both build halves when a bug appears only in a release.** This one
   needed the code to be right *and* the launch method to be right.
3. **Do not use one counter for two meanings.** "Nothing to do" and "could not
   do it" must be separate, or a real failure will read as success.
4. **A clean startup is not evidence the app works.** The pool here initialised
   fine and still failed on every PDF.
