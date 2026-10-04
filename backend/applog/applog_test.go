package applog

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A slog call must reach the file. This is the whole point of the package: the
// shipped Windows build is a GUI binary with no console, so stderr is gone.
func TestInitWritesLogToFile(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer Close()

	slog.Warn("parse produced nothing", "path", "C:/docs/scan.pdf", "sourceType", "pdf")

	data, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	got := string(data)
	for _, want := range []string{"parse produced nothing", "scan.pdf", "sourceType=pdf"} {
		if !strings.Contains(got, want) {
			t.Fatalf("log missing %q\n--- log ---\n%s", want, got)
		}
	}
}

// Debug lines stay out unless VECTILE_DEBUG is set, so a normal run is not
// buried in per-page extraction noise.
func TestDebugIsOffByDefault(t *testing.T) {
	t.Setenv(verboseEnv, "")
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	defer Close()

	slog.Debug("very chatty per-page detail")
	slog.Info("kept")
	Close()

	data, _ := os.ReadFile(Path(dir))
	if strings.Contains(string(data), "very chatty") {
		t.Fatal("debug line written without VECTILE_DEBUG")
	}
	if !strings.Contains(string(data), "kept") {
		t.Fatal("info line was dropped")
	}
}

func TestDebugOnWhenEnvSet(t *testing.T) {
	t.Setenv(verboseEnv, "1")
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	defer Close()

	slog.Debug("verbose detail here")
	Close()

	data, _ := os.ReadFile(Path(dir))
	if !strings.Contains(string(data), "verbose detail here") {
		t.Fatal("debug line dropped with VECTILE_DEBUG set")
	}
}

// A log past the cap is rotated aside, keeping the newest content in the live
// file rather than growing without bound.
func TestRotationKeepsRecentLog(t *testing.T) {
	dir := t.TempDir()
	path := Path(dir)

	// Pre-fill past the cap so Init rotates.
	if err := os.WriteFile(path, []byte(strings.Repeat("x", maxBytes+1)), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	slog.Error("after rotation")
	Close()

	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("expected rotated file: %v", err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "after rotation") {
		t.Fatal("live log should hold only the new content")
	}
	if len(data) >= maxBytes {
		t.Fatalf("live log not truncated: %d bytes", len(data))
	}
}

// Init on a path that exists as a *file* must return an error, not panic: a
// broken data dir should not stop the app from starting.
func TestInitBadDirReturnsError(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// MkdirAll fails because the path is a file.
	if err := Init(blocker); err == nil {
		t.Fatal("expected an error for a file used as the log dir")
	}
}
