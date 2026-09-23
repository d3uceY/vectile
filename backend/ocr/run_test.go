package ocr

import (
	"image"
	"os"
	"path/filepath"
	"testing"
)

// testImage is a 1x1 image, enough for the PNG encoder.
func testImage() image.Image { return image.NewGray(image.Rect(0, 0, 1, 1)) }

// writeLang plants a traineddata file so Languages() has something to find.
func writeLang(t *testing.T, lang string) {
	t.Helper()
	if err := os.MkdirAll(TessdataDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(TessdataDir(), lang+".traineddata"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestUseLangsFallsBackToEnglish(t *testing.T) {
	ocrTestEnv(t)
	writeLang(t, "eng")
	writeLang(t, "deu")

	cases := []struct {
		in   []string
		want []string
	}{
		{[]string{"eng"}, []string{"eng"}},
		{[]string{"eng", "deu"}, []string{"eng", "deu"}},
		// A language with no data is dropped rather than failing every page.
		{[]string{"fra"}, []string{"eng"}},
		{[]string{"fra", "deu"}, []string{"deu"}},
		{nil, []string{"eng"}},
	}
	for _, tc := range cases {
		got := useLangs(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("useLangs(%v) = %v, want %v", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("useLangs(%v) = %v, want %v", tc.in, got, tc.want)
				break
			}
		}
	}
}

func TestLanguagesReadsTessdata(t *testing.T) {
	ocrTestEnv(t)
	writeLang(t, "eng")
	writeLang(t, "osd")

	got := Languages()
	if len(got) != 2 || got[0] != "eng" || got[1] != "osd" {
		t.Fatalf("Languages() = %v, want [eng osd]", got)
	}
}

func TestLanguagesIsEmptyWithoutABundle(t *testing.T) {
	ocrTestEnv(t)
	if got := Languages(); len(got) != 0 {
		t.Fatalf("Languages() = %v, want none", got)
	}
}

func TestInstalledFalseWithoutABundle(t *testing.T) {
	ocrTestEnv(t)
	if Installed() {
		t.Fatal("Installed() must be false when nothing was downloaded")
	}
}

func TestRunWithoutABundleFails(t *testing.T) {
	ocrTestEnv(t)
	if _, err := Run(t.Context(), testImage(), []string{"eng"}); err == nil {
		t.Fatal("expected an error when the plugin is not installed")
	}
}

func TestValidateRejectsAMissingBinary(t *testing.T) {
	ocrTestEnv(t)
	if err := validate(filepath.Join(t.TempDir(), "nope"), t.TempDir()); err == nil {
		t.Fatal("expected an error for a binary that does not exist")
	}
}
