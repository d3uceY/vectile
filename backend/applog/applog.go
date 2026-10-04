// Package applog configures the process-wide slog logger so every warning and
// error is written to a file in the app-data directory.
//
// vectile ships as a Windows GUI binary (-H windowsgui), which has no console:
// stderr goes nowhere. Without this, a release build that fails to parse a PDF
// reports only "0 indexed" and the reason is unrecoverable. Everything logged
// through slog (indexing, parsing, OCR, model loading) lands in
// <app-data>/vectile.log instead.
package applog

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// FileName is the log file inside the app-data directory.
const FileName = "vectile.log"

// maxBytes caps the log before it is rotated to FileName+".1". Two files at
// this size bound the on-disk cost at ~4 MB, which is plenty for diagnostics
// and small enough that nobody notices it.
const maxBytes = 2 << 20 // 2 MiB

// verboseEnv turns on debug lines when set to a non-empty value. Debug is off
// by default because OCR and per-page extraction log at that level and a
// normal index run would otherwise bury the warnings.
const verboseEnv = "VECTILE_DEBUG"

var (
	mu     sync.Mutex
	file   *os.File
	closer io.Closer
)

// Init opens (and rotates if needed) the log file under dir and installs a
// slog handler that writes there. It is safe to call before appdata.Init
// succeeds if dir is supplied explicitly.
//
// A failure to open the file is not fatal: the app still runs, and the
// returned error lets main report it. Returning an error rather than panicking
// keeps a read-only data directory from stopping launch.
func Init(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	path := filepath.Join(dir, FileName)
	if err := rotate(path); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if closer != nil {
		_ = closer.Close()
	}
	file = f
	closer = f

	level := slog.LevelInfo
	if os.Getenv(verboseEnv) != "" {
		level = slog.LevelDebug
	}

	// Text format, not JSON: this file is read by a human pasting a snippet
	// into a bug report, and the source position is what makes a warning
	// actionable. AddSource costs little at this volume.
	handler := slog.NewTextHandler(f, &slog.HandlerOptions{
		Level:     level,
		AddSource: true,
	})
	slog.SetDefault(slog.New(handler))
	return nil
}

// Close flushes and releases the log file. Call on shutdown.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if closer != nil {
		_ = closer.Close()
		closer = nil
		file = nil
	}
}

// Path returns the log file path inside dir.
func Path(dir string) string { return filepath.Join(dir, FileName) }

// rotate moves an oversized log aside to <path>.1, replacing any previous
// rotation. A single rename keeps this atomic and needs no rewriting.
func rotate(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size() < maxBytes {
		return nil
	}
	if err := os.Remove(path + ".1"); err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(path, path+".1")
}
