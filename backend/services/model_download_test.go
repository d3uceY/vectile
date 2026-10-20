package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vectile/backend/appdata"
)

func TestCatalogContainsRecommended(t *testing.T) {
	cat := ModelCatalog()
	if len(cat) == 0 {
		t.Fatal("catalog is empty")
	}
	if _, ok := CatalogByKey("bge-small-en-v1.5-q8_0"); !ok {
		t.Fatal("missing recommended model key")
	}
}

// DownloadModel rejects a model that is already installed.
func TestDownloadRejectsInstalled(t *testing.T) {
	appdata.Dir = t.TempDir()
	dir := appdata.ModelsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cat, _ := CatalogByKey("bge-small-en-v1.5-q8_0")
	if err := os.WriteFile(filepath.Join(dir, cat.File), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &ModelService{core: &Core{}}
	if _, err := s.DownloadModel("bge-small-en-v1.5-q8_0"); err == nil {
		t.Fatal("expected an error when the model is already installed")
	}
}

// A downloaded file that isn't a valid embedding GGUF must fail and remove the
// partial file. The body is pinned so the download reaches the GGUF check
// rather than stopping at the checksum gate.
func TestDownloadRejectsInvalidGGUF(t *testing.T) {
	dir := testModelsDir(t)
	body := []byte("this is not a GGUF file")
	srv := testModelServer(t, body)

	cat, _ := CatalogByKey("bge-small-en-v1.5-q8_0")
	cat = pinToBody(cat, body)
	cat.URL = srv
	s := &ModelService{core: &Core{}}
	s.runDownload(context.Background(), cat)

	st := s.GetDownloadState()
	if st.Status != "failed" {
		t.Fatalf("expected failed, got %q (%s)", st.Status, st.Error)
	}
	if !strings.Contains(strings.ToLower(st.Error), "gguf") {
		t.Fatalf("error = %q, want the GGUF check to be what rejected it", st.Error)
	}
	assertNotInstalled(t, dir, cat.File)
}

// TestDownloadRejectsBadChecksum: any 200 OK body used to be written, parsed and
// installed; a body that misses the pinned SHA-256 must now be dropped first.
func TestDownloadRejectsBadChecksum(t *testing.T) {
	dir := testModelsDir(t)
	body := []byte("plausible looking model bytes")
	srv := testModelServer(t, body)

	cat, _ := CatalogByKey("bge-small-en-v1.5-q8_0")
	cat = pinToBody(cat, body)
	cat.SHA256 = strings.Repeat("ab", 32) // right size, wrong digest
	cat.URL = srv

	s := &ModelService{core: &Core{}}
	s.runDownload(context.Background(), cat)

	st := s.GetDownloadState()
	if st.Status != "failed" {
		t.Fatalf("expected failed, got %q (%s)", st.Status, st.Error)
	}
	if !strings.Contains(st.Error, "checksum mismatch") {
		t.Fatalf("error = %q, want a checksum mismatch", st.Error)
	}
	assertNotInstalled(t, dir, cat.File)
}

// A body larger than the catalog pins is refused, so a server that keeps
// sending can't fill the disk before the checks run.
func TestDownloadRejectsOversizeBody(t *testing.T) {
	dir := testModelsDir(t)
	body := bytes.Repeat([]byte("x"), 4096)
	srv := testModelServer(t, body)

	cat, _ := CatalogByKey("bge-small-en-v1.5-q8_0")
	cat.SHA256 = strings.Repeat("00", 32)
	cat.SizeBytes = 64 // pin a 64-byte file, then serve 4 KB
	cat.URL = srv

	s := &ModelService{core: &Core{}}
	s.runDownload(context.Background(), cat)

	st := s.GetDownloadState()
	if st.Status != "failed" {
		t.Fatalf("expected failed, got %q (%s)", st.Status, st.Error)
	}
	if !strings.Contains(st.Error, "expected 64") {
		t.Fatalf("error = %q, want a size mismatch", st.Error)
	}
	assertNotInstalled(t, dir, cat.File)
}

// TestDownloadRefusesUnpinnedModel: a valid embedding GGUF that isn't the file
// the catalog pinned used to be renamed into models/ and activated.
func TestDownloadRefusesUnpinnedModel(t *testing.T) {
	dir := testModelsDir(t)
	body := testGGUF(384, 512) // valid GGUF, wrong bytes
	srv := testModelServer(t, body)

	cat, _ := CatalogByKey("all-minilm-l6-v2-q4_k_m")
	cat.SizeBytes = int64(len(body)) // right length, so only the digest can catch it
	cat.SHA256 = strings.Repeat("cd", 32)
	cat.URL = srv

	s := &ModelService{core: &Core{}}
	s.runDownload(context.Background(), cat)

	if st := s.GetDownloadState(); st.Status != "failed" {
		t.Fatalf("expected failed, got %q", st.Status)
	}
	assertNotInstalled(t, dir, cat.File)
}

// Every catalog entry has to pin a digest and an exact size, or the download
// gate above silently degrades to "anything goes".
func TestCatalogModelsArePinned(t *testing.T) {
	for _, m := range ModelCatalog() {
		if _, err := hex.DecodeString(m.SHA256); err != nil || len(m.SHA256) != sha256.Size*2 {
			t.Errorf("%s: sha256 = %q", m.Key, m.SHA256)
		}
		if m.SizeBytes <= 0 {
			t.Errorf("%s: sizeBytes = %d", m.Key, m.SizeBytes)
		}
	}
}

// testGGUF builds a minimal but valid GGUF carrying an embedding length and a
// context length, i.e. something ReadMetadata accepts as an embedding model.
func testGGUF(dims, contextLen int) []byte {
	var kv bytes.Buffer
	kvs := uint64(0)
	str := func(key, val string) {
		_ = binary.Write(&kv, binary.LittleEndian, uint64(len(key)))
		kv.WriteString(key)
		_ = binary.Write(&kv, binary.LittleEndian, uint32(8)) // GGUF_TYPE_STRING
		_ = binary.Write(&kv, binary.LittleEndian, uint64(len(val)))
		kv.WriteString(val)
		kvs++
	}
	num := func(key string, val int) {
		_ = binary.Write(&kv, binary.LittleEndian, uint64(len(key)))
		kv.WriteString(key)
		_ = binary.Write(&kv, binary.LittleEndian, uint32(4)) // GGUF_TYPE_UINT32
		_ = binary.Write(&kv, binary.LittleEndian, uint32(val))
		kvs++
	}
	str("general.architecture", "bert")
	num("bert.embedding_length", dims)
	num("bert.context_length", contextLen)

	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, uint32(0x46554747)) // "GGUF"
	_ = binary.Write(&out, binary.LittleEndian, uint32(3))          // version
	_ = binary.Write(&out, binary.LittleEndian, uint64(0))          // tensor count
	_ = binary.Write(&out, binary.LittleEndian, kvs)
	out.Write(kv.Bytes())
	return out.Bytes()
}

// testModelsDir points appdata at a temp dir with a models/ folder.
func testModelsDir(t *testing.T) string {
	t.Helper()
	appdata.Dir = t.TempDir()
	dir := appdata.ModelsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// testModelServer serves body for every request.
func testModelServer(t *testing.T, body []byte) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// pinToBody makes cat describe exactly body, so a test can get past the
// checksum gate and exercise what comes after it.
func pinToBody(cat CatalogModel, body []byte) CatalogModel {
	sum := sha256.Sum256(body)
	cat.SHA256 = hex.EncodeToString(sum[:])
	cat.SizeBytes = int64(len(body))
	return cat
}

// assertNotInstalled checks that neither the model nor its .part file is left
// behind by a rejected download.
func assertNotInstalled(t *testing.T, dir, file string) {
	t.Helper()
	for _, name := range []string{file, file + ".part"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s should not exist after a rejected download", name)
		}
	}
}
