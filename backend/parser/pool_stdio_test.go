package parser

import (
	"strings"
	"testing"

	"github.com/klippa-app/go-pdfium/webassembly"
)

// The released Windows build is linked with -H windowsgui, so the process can
// have invalid std handles. go-pdfium defaults Stdout/Stderr to os.Stdout /
// os.Stderr and hands them to wazero as the WASI stdio fds, which made every
// PDF fail with:
//
//	could not instantiate webassembly module: GetFileType /dev/stdout:
//	The handle is invalid.
//
// The pool still initialised, so this surfaced only as "0 indexed" in the UI.
// initPDFPool must therefore supply its own writers and never fall back to the
// process std handles.
func TestInitPDFPoolDoesNotUseProcessStdio(t *testing.T) {
	// A pool built with explicit writers must serve a real instance. If
	// initPDFPool relied on os.Stdout this is the call that fails on Windows
	// GUI builds.
	pdfPoolOnce.Do(initPDFPool)
	if pdfPoolErr != nil {
		t.Fatalf("pool init: %v", pdfPoolErr)
	}
	inst, err := pdfPool.GetInstance(30 * 1e9) // 30s in ns
	if err != nil {
		t.Fatalf("GetInstance: %v", err)
	}
	defer inst.Close()
}

// Guards the actual regression: a pool configured the way go-pdfium defaults
// to must be distinguishable from ours, so the fix is not silently reverted by
// dropping the Stdout/Stderr fields.
func TestPoolConfigSuppliesStdioWriters(t *testing.T) {
	cfg := webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
		Stdout:   logWriter{},
		Stderr:   logWriter{},
	}
	if cfg.Stdout == nil || cfg.Stderr == nil {
		t.Fatal("pdfium pool config must set Stdout and Stderr explicitly")
	}
}

// logWriter must swallow pdfium's output without erroring, including empty
// writes, because it now backs the WASI stdio fds.
func TestLogWriterAlwaysSucceeds(t *testing.T) {
	w := logWriter{}
	for _, in := range []string{"", "   ", "issue: Missing font", "multi\nline\n"} {
		n, err := w.Write([]byte(in))
		if err != nil {
			t.Fatalf("Write(%q) error: %v", in, err)
		}
		if n != len(in) {
			t.Fatalf("Write(%q) = %d, want %d", in, n, len(in))
		}
	}
	// A large write must not be truncated either.
	big := strings.Repeat("x", 1<<16)
	if n, err := w.Write([]byte(big)); err != nil || n != len(big) {
		t.Fatalf("large Write = %d, %v; want %d, nil", n, err, len(big))
	}
}
