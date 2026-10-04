package indexer

import (
	"os"
	"path/filepath"
	"testing"

	"vectile/backend/appdata"
	"vectile/backend/db"
)

// openTestDB gives a scratch database with the schema applied, so these tests
// exercise the real source/result bookkeeping instead of a stub.
func openTestDB(t *testing.T) {
	t.Helper()
	// appdata.Init is what normally creates db/, models/, and plugins/; these
	// tests bypass it, so make the directory the DB path expects.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	appdata.Dir = dir
	if err := db.Open(appdata.DBPath()); err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
}

// The distinction that matters: a file that is already indexed is *skipped*,
// while a file that was read and produced no text is *failed*. Reporting the
// second as the first is what made a broken parse look like a healthy no-op.
func TestFileToItemSeparatesSkipFromFailure(t *testing.T) {
	openTestDB(t)
	cfg := parseCfg()

	dir := t.TempDir()
	collectionID, err := db.GetOrCreateCollection(db.DB, "t", "project", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// A PDF with no text layer and no OCR: read fine, extracts nothing.
	scan := filepath.Join(dir, "scan.pdf")
	if err := os.WriteFile(scan, minimalScanPDF(), 0o644); err != nil {
		t.Fatal(err)
	}

	res := &IndexResult{}
	if item := fileToItem(t.Context(), db.DB, cfg, scan, collectionID, true, res); item != nil {
		t.Fatal("a text-free PDF should produce no item")
	}
	if res.Failed != 1 || res.Skipped != 0 {
		t.Fatalf("Failed=%d Skipped=%d, want Failed=1 Skipped=0", res.Failed, res.Skipped)
	}
	if len(res.ErrorMessages) == 0 {
		t.Fatal("a failure should leave a reason in ErrorMessages")
	}

	// Now a real text file, indexed once, then re-run without force: the
	// second pass has nothing to do and must count as skipped.
	good := filepath.Join(dir, "note.md")
	if err := os.WriteFile(good, []byte("# Title\n\n"+repeatWord("content", 60)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res2 := &IndexResult{}
	item := fileToItem(t.Context(), db.DB, cfg, good, collectionID, false, res2)
	if item == nil {
		t.Fatal("a text file should produce an item")
	}
	if res2.Skipped != 0 || res2.Failed != 0 {
		t.Fatalf("first pass: Skipped=%d Failed=%d, want 0/0", res2.Skipped, res2.Failed)
	}

	// Record it so the unchanged check can find it. Vectors must be
	// EmbeddingDim wide: vec0 rejects zero-length ones.
	vecs := make([][]float32, len(item.Chunks))
	for i := range vecs {
		vecs[i] = make([]float32, db.EmbeddingDim)
		vecs[i][0] = 0.5
	}
	if err := storeItem(db.DB, collectionID, item, vecs); err != nil {
		t.Fatalf("storeItem: %v", err)
	}

	res3 := &IndexResult{}
	if again := fileToItem(t.Context(), db.DB, cfg, good, collectionID, false, res3); again != nil {
		t.Fatal("an unchanged file should be skipped")
	}
	if res3.Skipped != 1 || res3.Failed != 0 {
		t.Fatalf("second pass: Skipped=%d Failed=%d, want Skipped=1 Failed=0", res3.Skipped, res3.Failed)
	}
}

// A missing file must be a failure, not a silent skip.
func TestUnreadableFileCountsAsFailure(t *testing.T) {
	openTestDB(t)

	collectionID, err := db.GetOrCreateCollection(db.DB, "t", "project", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := &IndexResult{}
	missing := filepath.Join(t.TempDir(), "gone.md")
	_ = fileToItem(t.Context(), db.DB, parseCfg(), missing, collectionID, true, res)

	// No stat and no read: the hash fails, which is a failure.
	if res.Failed != 1 {
		t.Fatalf("Failed=%d, want 1 for an unreadable file", res.Failed)
	}
	if res.Skipped != 0 {
		t.Fatalf("Skipped=%d, want 0", res.Skipped)
	}
}

// Merge must carry the new counter, or an Index All run under-reports it.
func TestMergeCarriesFailed(t *testing.T) {
	a := &IndexResult{Failed: 2, Skipped: 1}
	b := &IndexResult{Failed: 3, Skipped: 4}
	a.Merge(b)
	if a.Failed != 5 || a.Skipped != 5 {
		t.Fatalf("Failed=%d Skipped=%d, want 5/5", a.Failed, a.Skipped)
	}
}

func repeatWord(w string, n int) string {
	out := make([]byte, 0, n*(len(w)+1))
	for i := 0; i < n; i++ {
		out = append(out, w...)
		out = append(out, ' ')
	}
	return string(out)
}

// minimalScanPDF is a one-page PDF with no text layer and no xref repair work.
func minimalScanPDF() []byte {
	return []byte("%PDF-1.4\n" +
		"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n" +
		"3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] >>\nendobj\n" +
		"trailer\n<< /Size 4 /Root 1 0 R >>\n%%EOF\n")
}
