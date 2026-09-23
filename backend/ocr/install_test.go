package ocr

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vectile/backend/appdata"
)

// ocrTestEnv points the app-data dir at a temp folder, so nothing touches the
// real plugin directory.
func ocrTestEnv(t *testing.T) {
	t.Helper()
	appdata.Dir = t.TempDir()
}

// archiveBytes builds a bundle with the same layout the release ships: one
// "tesseract/" root containing bin/ and tessdata/.
func archiveBytes(t *testing.T, isZip bool) []byte {
	t.Helper()
	entries := map[string]string{
		"tesseract/bin/" + filepath.Base(BinPath()): "not really a binary",
		"tesseract/tessdata/eng.traineddata":        "eng data",
	}

	var buf bytes.Buffer
	if isZip {
		zw := zip.NewWriter(&buf)
		for name, body := range entries {
			w, err := zw.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write([]byte(body)); err != nil {
				t.Fatal(err)
			}
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}

	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range entries {
		if err := tw.WriteHeader(&tar.Header{
			Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// stubServer serves an archive and a SHA256SUMS naming it. sum overrides the
// advertised hash, so a mismatch can be tested.
func stubServer(t *testing.T, platform string, archive []byte, sum string) *httptest.Server {
	t.Helper()
	if sum == "" {
		h := sha256.Sum256(archive)
		sum = hex.EncodeToString(h[:])
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "SHA256SUMS") {
			fmt.Fprintf(w, "%s  %s\n", sum, platforms[platform].name)
			return
		}
		_, _ = w.Write(archive)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// pointAt redirects downloads at srv for one test.
func pointAt(t *testing.T, srv *httptest.Server) {
	t.Helper()
	old := base
	base = srv.URL + "/"
	t.Cleanup(func() { base = old })
}

// stubValidate replaces the "does the binary actually run" check, which no
// synthetic archive can satisfy.
func stubValidate(t *testing.T) {
	t.Helper()
	old := validateBin
	validateBin = func(bin, tessdata string) error { return nil }
	t.Cleanup(func() { validateBin = old })
}

func TestInstallExtractsAndSwaps(t *testing.T) {
	for _, tc := range []struct {
		platform string
		archive  []byte
	}{
		{"windows-amd64", nil},
		{"linux-amd64", nil},
	} {
		t.Run(tc.platform, func(t *testing.T) {
			ocrTestEnv(t)
			stubValidate(t)
			isZip := platforms[tc.platform].isZip
			archive := archiveBytes(t, isZip)
			pointAt(t, stubServer(t, tc.platform, archive, ""))

			if err := Install(context.Background(), tc.platform, nil); err != nil {
				t.Fatalf("Install: %v", err)
			}
			if !Installed() {
				t.Fatal("expected the binary to be installed")
			}
			if _, err := os.Stat(filepath.Join(TessdataDir(), "eng.traineddata")); err != nil {
				t.Fatalf("tessdata missing after install: %v", err)
			}
			// The scratch dir and the .part download must not survive.
			if _, err := os.Stat(Dir() + ".tmp"); !os.IsNotExist(err) {
				t.Fatalf("staging dir left behind (err=%v)", err)
			}
			entries, err := os.ReadDir(filepath.Join(appdata.PluginsDir(), "tesseract"))
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".part") || strings.HasSuffix(e.Name(), ".tmp") {
					t.Fatalf("leftover %s", e.Name())
				}
			}
		})
	}
}

func TestInstallRejectsBadChecksum(t *testing.T) {
	ocrTestEnv(t)
	stubValidate(t)
	archive := archiveBytes(t, true)
	pointAt(t, stubServer(t, "windows-amd64", archive, strings.Repeat("0", 64)))

	err := Install(context.Background(), "windows-amd64", nil)
	if err == nil {
		t.Fatal("expected a checksum failure")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected a checksum error, got %v", err)
	}
	if Installed() {
		t.Fatal("a mismatched archive must not be installed")
	}
}

func TestInstallRejectsZipSlip(t *testing.T) {
	ocrTestEnv(t)
	stubValidate(t)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("tesseract/../../evil.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("pwned")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	pointAt(t, stubServer(t, "windows-amd64", buf.Bytes(), ""))

	if err := Install(context.Background(), "windows-amd64", nil); err == nil {
		t.Fatal("expected an unsafe-path error")
	}
	escaped := filepath.Join(appdata.PluginsDir(), "..", "evil.txt")
	if _, err := os.Stat(escaped); err == nil {
		t.Fatal("archive escaped the plugin directory")
	}
}

func TestSafeJoin(t *testing.T) {
	root := filepath.Join("plugins", "tesseract", "5.5.3")
	cases := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{"tesseract/", "", false},
		{"tesseract", "", false},
		{"tesseract/bin/tesseract", filepath.Join(root, "bin", "tesseract"), false},
		{"tesseract/tessdata/eng.traineddata", filepath.Join(root, "tessdata", "eng.traineddata"), false},
		// A bundle without the wrapper dir still lands correctly.
		{"bin/tesseract", filepath.Join(root, "bin", "tesseract"), false},
		// Escapes.
		{"../evil", "", true},
		{"tesseract/../../evil", "", true},
		{"tesseract/bin/../../../evil", "", true},
	}
	for _, tc := range cases {
		got, err := safeJoin(root, tc.name)
		if tc.wantErr {
			if err == nil {
				t.Errorf("safeJoin(%q) = %q, want an error", tc.name, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("safeJoin(%q): %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("safeJoin(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestSweepClearsLeftovers(t *testing.T) {
	ocrTestEnv(t)
	root := filepath.Join(appdata.PluginsDir(), "tesseract")
	if err := os.MkdirAll(filepath.Join(root, Version, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"bundle.zip.part", "5.5.3.tmp"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	Sweep()

	for _, name := range []string{"bundle.zip.part", "5.5.3.tmp"} {
		if _, err := os.Stat(filepath.Join(root, name)); !os.IsNotExist(err) {
			t.Fatalf("%s survived Sweep", name)
		}
	}
	if _, err := os.Stat(filepath.Join(root, Version)); err != nil {
		t.Fatalf("Sweep removed an installed version: %v", err)
	}
}

func TestUninstallRemovesEverything(t *testing.T) {
	ocrTestEnv(t)
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Uninstall(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(appdata.PluginsDir(), "tesseract")); !os.IsNotExist(err) {
		t.Fatal("Uninstall left the plugin dir behind")
	}
}

func TestManifestMatchesPlatformLookup(t *testing.T) {
	for platform, a := range platforms {
		if a.approx <= 0 {
			t.Errorf("%s has no size", platform)
		}
		url := AssetURL(platform)
		if !strings.HasPrefix(url, base) || !strings.HasSuffix(url, a.name) {
			t.Errorf("%s asset URL looks wrong: %s", platform, url)
		}
		if !strings.Contains(a.name, platform) {
			t.Errorf("%s asset name does not name its platform: %s", platform, a.name)
		}
		if strings.HasSuffix(a.name, ".zip") != a.isZip {
			t.Errorf("%s archive kind disagrees with its extension: %s", platform, a.name)
		}
	}
	if !strings.Contains(ReleaseURL(), tag) {
		t.Errorf("release URL does not point at %s: %s", tag, ReleaseURL())
	}
}

func TestPlatformIsOneOfTheKeys(t *testing.T) {
	p := Platform()
	if p == "" {
		t.Skip("no bundle for this OS/arch")
	}
	if _, ok := platforms[p]; !ok {
		t.Fatalf("Platform() returned %q, which is not in the manifest", p)
	}
}
