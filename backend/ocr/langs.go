package ocr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// tessdataBase is where single-language data files come from: the upstream
// fast integer models, one file per language, at the repository root.
//
// Pinned to a commit rather than main. The upstream repository publishes the
// files but no checksums, so there is nothing to verify a download against;
// what keeps the bytes stable is the URL itself. Bumping the hash is a
// deliberate act, the same way the bundle's Version is. A var only so tests
// can point it at a local server.
var tessdataBase = "https://raw.githubusercontent.com/tesseract-ocr/tessdata_fast/87416418657359cb625c412a48b6e1d6d41c29bd/"

// langPattern is the only shape a language code may take. Codes come from the
// frontend and from config.json, and both end up in a file path, so anything
// outside this set is refused rather than joined.
var langPattern = regexp.MustCompile(`^[a-z0-9_]+$`)

// EnglishCode is the one language that cannot be removed: useLangs falls back
// to it whenever the configured list names nothing the bundle holds.
const EnglishCode = "eng"

// LanguagePath is where one language's data lives.
func LanguagePath(code string) string {
	return filepath.Join(TessdataDir(), code+".traineddata")
}

// InstallLanguage downloads one language's data into the bundle's tessdata
// directory. The download lands in a .part beside the target and only becomes
// <code>.traineddata once it is complete, so an interrupted run leaves the
// previous data (or nothing) instead of a truncated file that Tesseract would
// load and read badly.
//
// onProgress is called with (downloaded, total) at most every 100ms; total is
// 0 when the server sends no length. It may be nil.
func InstallLanguage(ctx context.Context, code string, onProgress func(downloaded, total int64)) error {
	if !langPattern.MatchString(code) {
		return fmt.Errorf("invalid language code %q", code)
	}
	if !Installed() {
		return fmt.Errorf("install the OCR plugin first")
	}
	if err := os.MkdirAll(TessdataDir(), 0o755); err != nil {
		return err
	}

	dest := LanguagePath(code)
	part := dest + ".part"
	defer os.Remove(part) // no-op once the rename has happened

	if err := fetch(ctx, tessdataBase+code+".traineddata", part, "", onProgress); err != nil {
		return fmt.Errorf("download %s: %w", code, err)
	}
	return os.Rename(part, dest)
}

// RemoveLanguage deletes one language's data file. A missing file is not an
// error: the point is that afterwards the language is gone.
func RemoveLanguage(code string) error {
	if !langPattern.MatchString(code) {
		return fmt.Errorf("invalid language code %q", code)
	}
	if code == EnglishCode {
		return fmt.Errorf("English cannot be removed")
	}
	if err := os.Remove(LanguagePath(code)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
