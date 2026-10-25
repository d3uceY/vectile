package ocr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// langServer serves traineddata files from a temp directory and points the
// package at itself. The returned dir is what "the internet" holds.
func langServer(t *testing.T, files map[string]string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	prev := tessdataBase
	tessdataBase = srv.URL + "/"
	t.Cleanup(func() { tessdataBase = prev })
	return srv.URL
}

// withBundle puts a fake binary in place so Installed() is true: language data
// only makes sense once the plugin is there.
func withBundle(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(BinPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(BinPath(), []byte("not really a binary"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// onDisk reports whether a language's data made it into tessdata.
func onDisk(code string) bool {
	_, err := os.Stat(LanguagePath(code))
	return err == nil
}

func TestInstallLanguage(t *testing.T) {
	ocrTestEnv(t)
	withBundle(t)
	langServer(t, map[string]string{"deu.traineddata": "german data"})

	var progress int
	err := InstallLanguage(context.Background(), "deu", func(downloaded, _ int64) {
		progress = int(downloaded)
	})
	if err != nil {
		t.Fatalf("InstallLanguage: %v", err)
	}
	if progress != len("german data") {
		t.Errorf("progress reported %d bytes, want %d", progress, len("german data"))
	}
	got, err := os.ReadFile(LanguagePath("deu"))
	if err != nil {
		t.Fatalf("read installed language: %v", err)
	}
	if string(got) != "german data" {
		t.Errorf("installed data = %q, want %q", got, "german data")
	}
	if _, err := os.Stat(LanguagePath("deu") + ".part"); !os.IsNotExist(err) {
		t.Errorf("left a .part file behind: %v", err)
	}
	if !onDisk("deu") {
		t.Error("no data on disk after a successful install")
	}
}

func TestInstallLanguageRejectsBadCodes(t *testing.T) {
	ocrTestEnv(t)
	withBundle(t)
	langServer(t, nil)

	for _, code := range []string{"", "ENG", "../evil", "de/../x", "deu traineddata", "deu.traineddata"} {
		if err := InstallLanguage(context.Background(), code, nil); err == nil {
			t.Errorf("InstallLanguage(%q) succeeded, want an error", code)
		}
	}
	// Nothing may have been written outside tessdata.
	entries, err := os.ReadDir(TessdataDir())
	if err != nil {
		return // no dir at all is fine
	}
	if len(entries) != 0 {
		t.Errorf("bad codes left files behind: %v", entries)
	}
}

func TestInstallLanguageFailures(t *testing.T) {
	// No bundle: there is nothing to add data to.
	ocrTestEnv(t)
	langServer(t, map[string]string{"deu.traineddata": "german data"})
	err := InstallLanguage(context.Background(), "deu", nil)
	if err == nil || !strings.Contains(err.Error(), "plugin") {
		t.Fatalf("got %v, want an error naming the plugin", err)
	}

	// A code the server does not have fails and leaves nothing behind.
	ocrTestEnv(t)
	withBundle(t)
	langServer(t, map[string]string{"eng.traineddata": "english data"})
	if err := InstallLanguage(context.Background(), "xxx", nil); err == nil {
		t.Fatal("installing a language the server does not have should fail")
	}
	if onDisk("xxx") {
		t.Error("a failed download left data behind")
	}
}

func TestRemoveLanguage(t *testing.T) {
	ocrTestEnv(t)
	withBundle(t)
	langServer(t, map[string]string{"deu.traineddata": "german data"})

	if err := InstallLanguage(context.Background(), "deu", nil); err != nil {
		t.Fatalf("InstallLanguage: %v", err)
	}
	if err := RemoveLanguage("deu"); err != nil {
		t.Fatalf("RemoveLanguage: %v", err)
	}
	if onDisk("deu") {
		t.Error("data is still there after RemoveLanguage")
	}
	// Removing what is not there is the outcome the caller wanted.
	if err := RemoveLanguage("deu"); err != nil {
		t.Errorf("second RemoveLanguage: %v", err)
	}
	if err := RemoveLanguage(EnglishCode); err == nil {
		t.Error("English should not be removable")
	}
}

func TestCatalog(t *testing.T) {
	all := Catalog()
	if len(all) < 100 {
		t.Fatalf("catalog holds %d languages, want the full upstream list", len(all))
	}
	if all[0].Code != EnglishCode {
		t.Errorf("first entry is %q, want %q", all[0].Code, EnglishCode)
	}
	seen := map[string]bool{}
	for _, l := range all {
		if !langPattern.MatchString(l.Code) {
			t.Errorf("catalog code %q is not a valid language code", l.Code)
		}
		if seen[l.Code] {
			t.Errorf("catalog lists %q twice", l.Code)
		}
		seen[l.Code] = true
		if strings.TrimSpace(l.Name) == "" {
			t.Errorf("catalog entry %q has no display name", l.Code)
		}
	}
	for i := 2; i < len(all); i++ {
		if all[i-1].Name > all[i].Name {
			t.Errorf("catalog is not sorted by name at %q / %q", all[i-1].Name, all[i].Name)
		}
	}
}

func TestLanguagesSorted(t *testing.T) {
	ocrTestEnv(t)
	withBundle(t)
	if err := os.MkdirAll(TessdataDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"deu", "eng", "afr"} {
		if err := os.WriteFile(LanguagePath(name), []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := strings.Join(Languages(), ",")
	if got != "afr,deu,eng" {
		t.Errorf("Languages() = %q, want sorted \"afr,deu,eng\"", got)
	}
}

func TestSweepClearsPartialLanguageDownloads(t *testing.T) {
	ocrTestEnv(t)
	withBundle(t)
	if err := os.MkdirAll(TessdataDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	part := LanguagePath("deu") + ".part"
	if err := os.WriteFile(part, []byte("half"), 0o644); err != nil {
		t.Fatal(err)
	}
	Sweep()
	if _, err := os.Stat(part); !os.IsNotExist(err) {
		t.Errorf("Sweep left %s behind: %v", part, err)
	}
}
