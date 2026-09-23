// Package ocr installs and runs a bundled Tesseract binary so vectile can
// read PDFs that are photos of pages rather than text.
//
// The binaries do not ship with vectile. They are published as one archive per
// OS/architecture by d3uceY/Tesseract-bundler-, downloaded on demand, checked
// against the release's own SHA256SUMS, and unpacked into vectile's plugin
// directory. Every run then uses that copy by absolute path with an explicit
// --tessdata-dir, so a Tesseract on the user's PATH is never involved.
//
// This package is a leaf: it knows nothing about PDFs, and only depends on
// appdata. The parser drives it.
package ocr

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"vectile/backend/appdata"
)

// Version is the Tesseract release vectile installs. Bump it together with the
// asset names below when the bundler publishes a new version.
const Version = "5.5.3"

const (
	repo = "d3uceY/Tesseract-bundler-"
	tag  = "tesseract-" + Version
)

// base is the download prefix for every asset of the release. A var only so
// tests can point it at a local server; production never changes it.
var base = "https://github.com/" + repo + "/releases/download/" + tag + "/"

// asset is one platform's archive in the release.
type asset struct {
	name   string
	approx int64 // for the pre-download size label only; the hash is the real check
	isZip  bool
}

// platforms maps a GOOS-GOARCH key to its archive. A platform missing from
// this map is one vectile cannot install OCR on, and both Platform and the
// service report that honestly instead of offering a download that would fail.
//
// Deleting entries here is the one-line retreat if a bundle turns out not to
// be self-contained: the Windows archive is built static, but the Unix ones
// link leptonica (and, on Linux, libicu and friends), so Install runs the
// binary once and refuses to keep it if it cannot execute.
var platforms = map[string]asset{
	"windows-amd64": {"tesseract-" + Version + "-windows-amd64.zip", 17882639, true},
	"linux-amd64":   {"tesseract-" + Version + "-linux-amd64.tar.gz", 20096630, false},
	"linux-arm64":   {"tesseract-" + Version + "-linux-arm64.tar.gz", 19892804, false},
	"macos-amd64":   {"tesseract-" + Version + "-macos-amd64.tar.gz", 18895822, false},
	"macos-arm64":   {"tesseract-" + Version + "-macos-arm64.tar.gz", 18786846, false},
}

// Platform returns the release key for this build, or "" when there is no
// bundle for it.
func Platform() string {
	goos := runtime.GOOS
	if goos == "darwin" {
		goos = "macos"
	}
	key := goos + "-" + runtime.GOARCH
	if _, ok := platforms[key]; ok {
		return key
	}
	return ""
}

// AssetSize returns the approximate download size, for the label shown before
// the download starts.
func AssetSize(platform string) int64 { return platforms[platform].approx }

// AssetURL returns the archive URL for platform.
func AssetURL(platform string) string { return base + platforms[platform].name }

// ChecksumsURL returns the release's SHA256SUMS, which lists every archive.
func ChecksumsURL() string { return base + "SHA256SUMS" }

// ReleaseURL is the human-facing release page.
func ReleaseURL() string { return "https://github.com/" + repo + "/releases/tag/" + tag }

// Dir is where one version's files live: plugins/tesseract/<version>/.
func Dir() string { return filepath.Join(appdata.PluginsDir(), "tesseract", Version) }

// BinPath is the bundled executable. It is always called by absolute path.
func BinPath() string {
	name := "tesseract"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(Dir(), "bin", name)
}

// TessdataDir is the bundled language data, passed to every run explicitly.
func TessdataDir() string { return filepath.Join(Dir(), "tessdata") }

// Installed reports whether the bundled binary is in place. It stats on every
// call rather than caching the answer, so a plugin installed while the app is
// running is picked up by the next page instead of needing a restart.
func Installed() bool {
	_, err := os.Stat(BinPath())
	return err == nil
}

// Languages lists the language data the bundle ships.
func Languages() []string {
	entries, err := os.ReadDir(TessdataDir())
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if name, ok := strings.CutSuffix(e.Name(), ".traineddata"); ok {
			out = append(out, name)
		}
	}
	return out
}
