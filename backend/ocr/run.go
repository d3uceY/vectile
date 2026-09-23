package ocr

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// runTimeout bounds one page. A scanned page takes a second or two, so this is
// only a backstop against a binary that hangs and would otherwise wedge an
// index run.
const runTimeout = 120 * time.Second

// Run OCRs one page image with the bundled binary and returns its text. langs
// is filtered to the language data the bundle actually ships.
func Run(ctx context.Context, img image.Image, langs []string) (string, error) {
	bin := BinPath()
	if _, err := os.Stat(bin); err != nil {
		return "", fmt.Errorf("tesseract is not installed: %w", err)
	}

	tmp, err := os.CreateTemp("", "vectile-ocr-*.png")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if err := png.Encode(tmp, img); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}

	args := []string{
		tmp.Name(), "stdout",
		"-l", strings.Join(useLangs(langs), "+"),
		"--tessdata-dir", filepath.ToSlash(TessdataDir()),
	}
	return execTesseract(ctx, bin, args)
}

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

// validate runs a freshly extracted binary once, so a bundle that cannot
// execute (missing shared libraries, wrong architecture) is rejected here
// instead of silently producing no text later.
func validate(bin, tessdata string) error {
	out, err := execTesseract(context.Background(), bin,
		[]string{"--list-langs", "--tessdata-dir", filepath.ToSlash(tessdata)})
	if err != nil {
		return fmt.Errorf("the downloaded OCR binary did not run: %w", err)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "eng" {
			return nil
		}
	}
	return errors.New("the downloaded OCR bundle has no English language data")
}

// execTesseract runs the bundled binary and returns trimmed stdout. WaitDelay
// releases Wait if the child's stdio outlives it, so a killed process cannot
// block the index run on a pipe nobody will close.
func execTesseract(ctx context.Context, bin string, args []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	hideWindow(cmd)
	cmd.WaitDelay = 5 * time.Second

	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return "", fmt.Errorf("tesseract: %w (%s)", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("tesseract: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
