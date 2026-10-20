package services

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"vectile/backend/appdata"
	"vectile/backend/config"
	"vectile/backend/db"
	"vectile/backend/embeddings"
)

// modelTestEnv wires a temp app-data dir + temp DB and returns a ModelService.
func modelTestEnv(t *testing.T) *ModelService {
	t.Helper()
	dir := t.TempDir()
	appdata.Dir = dir
	if err := os.MkdirAll(appdata.ModelsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := db.Open(filepath.Join(dir, "test.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	core := NewCore(cfg)
	core.CfgPath = filepath.Join(dir, "config.json")
	core.Embedder = embeddings.NewEmbedder(filepath.Join(appdata.ModelsDir(), "bge-m3.gguf"), 0, 0)
	return NewModelService(core)
}

// writeFakeModel drops a fake .gguf (with the given embedding dim and a native
// context length in its header) into the models folder and returns its path.
func writeFakeModel(t *testing.T, name string, dims int) string {
	t.Helper()
	path := filepath.Join(appdata.ModelsDir(), name)
	if err := os.WriteFile(path, fakeGGUFBytes(dims), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// fakeGGUFBytes builds a minimal .gguf header: architecture "bert", the given
// embedding dim, and a native context length of 8192.
func fakeGGUFBytes(dims int) []byte {
	var g bytes.Buffer
	modelGGUFStr(&g, "general.architecture")
	_ = binary.Write(&g, binary.LittleEndian, uint32(8))
	modelGGUFStr(&g, "bert")
	modelGGUFStr(&g, "bert.embedding_length")
	_ = binary.Write(&g, binary.LittleEndian, uint32(4))
	_ = binary.Write(&g, binary.LittleEndian, uint32(dims))
	modelGGUFStr(&g, "bert.context_length")
	_ = binary.Write(&g, binary.LittleEndian, uint32(4))
	_ = binary.Write(&g, binary.LittleEndian, uint32(8192))

	var hdr bytes.Buffer
	_ = binary.Write(&hdr, binary.LittleEndian, uint32(0x46554747))
	_ = binary.Write(&hdr, binary.LittleEndian, uint32(3))
	_ = binary.Write(&hdr, binary.LittleEndian, uint64(0))
	_ = binary.Write(&hdr, binary.LittleEndian, uint64(3))
	hdr.Write(g.Bytes())
	return hdr.Bytes()
}

func TestSyncModelsFromFolder(t *testing.T) {
	svc := modelTestEnv(t)
	path := writeFakeModel(t, "bge-m3.gguf", 1024)

	models, err := svc.ListModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].Dimensions != 1024 {
		t.Fatalf("ListModels after scan = %+v, want one 1024d model", models)
	}
	// Native context from the GGUF header and the default batch size must be
	// stored, not left at zero.
	if models[0].ContextWindow != 8192 || models[0].BatchSize != 32 {
		t.Fatalf("scanned model settings = ctx %d batch %d, want 8192/32",
			models[0].ContextWindow, models[0].BatchSize)
	}

	// File deleted -> row removed on the next fetch.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	models, err = svc.ListModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 0 {
		t.Fatalf("ListModels after delete = %+v, want empty", models)
	}
}

func TestImportModelCopiesAndRegisters(t *testing.T) {
	svc := modelTestEnv(t)
	src := filepath.Join(t.TempDir(), "mxbai.gguf")
	// Reuse the fake-model bytes by copying what writeFakeModel produces.
	if err := os.WriteFile(src, mustFakeGGUF(t, 1024), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := svc.ImportModel(src)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(m.Path) != appdata.ModelsDir() {
		t.Fatalf("imported model path %q not inside models dir", m.Path)
	}
	if _, err := os.Stat(m.Path); err != nil {
		t.Fatalf("copied model missing: %v", err)
	}
	if m.Dimensions != 1024 {
		t.Fatalf("imported dims = %d, want 1024", m.Dimensions)
	}
	if m.ContextWindow != 8192 || m.BatchSize != 32 {
		t.Fatalf("imported settings = ctx %d batch %d, want 8192/32",
			m.ContextWindow, m.BatchSize)
	}
}

func TestSetActiveModelDimChange(t *testing.T) {
	svc := modelTestEnv(t)
	a := writeFakeModel(t, "a.gguf", 1024)
	b := writeFakeModel(t, "b.gguf", 768)
	_, _ = svc.ListModels() // scan registers both

	// Same dim as the vector tables (1024), nothing embedded yet: applies
	// immediately, since a rebuild would destroy nothing.
	res, err := svc.SetActiveModel(a, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.NeedsRebuild {
		t.Fatal("switch with nothing embedded should not need a rebuild")
	}
	seedEmbedding(t, 1024)

	// Different dim without force: not applied, asks for confirmation.
	res, err = svc.SetActiveModel(b, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.NeedsRebuild {
		t.Fatal("different-dim switch should report NeedsRebuild")
	}
	active, _, err := db.GetActiveModel(db.DB)
	if err != nil || active.Path == b {
		t.Fatalf("active model changed before confirm: %+v err=%v", active, err)
	}

	// Confirmed switch: applied, vector tables rebuilt at the new dim.
	if _, err := svc.SetActiveModel(b, true); err != nil {
		t.Fatal(err)
	}
	active, ok, err := db.GetActiveModel(db.DB)
	if err != nil || !ok || active.Path != b {
		t.Fatalf("active after confirm = %+v ok=%v err=%v", active, ok, err)
	}
	if dim, _ := db.GetVectorDim(db.DB); dim != 768 {
		t.Fatalf("vector dim after rebuild = %d, want 768", dim)
	}
	if svc.core.Config().ActiveModel != b {
		t.Fatalf("cfg.ActiveModel = %q, want %q", svc.core.Config().ActiveModel, b)
	}
}

// TestSetActiveModelSameDimNeedsRebuild pins the rule that a model change is a
// reindex even when the dimensions match: two 384d models embed into different
// spaces, so the old vectors cannot be compared against the new model's query
// vectors.
func TestSetActiveModelSameDimNeedsRebuild(t *testing.T) {
	svc := modelTestEnv(t)
	a := writeFakeModel(t, "a.gguf", 384)
	b := writeFakeModel(t, "b.gguf", 384)
	_, _ = svc.ListModels()

	if _, err := svc.SetActiveModel(a, false); err != nil {
		t.Fatal(err)
	}
	seedEmbedding(t, 384)

	res, err := svc.SetActiveModel(b, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.NeedsRebuild {
		t.Fatal("same-dim model change should still report NeedsRebuild")
	}
	if active, _, _ := db.GetActiveModel(db.DB); active.Path != a {
		t.Fatalf("model switched before the user confirmed: %q", active.Path)
	}

	if _, err := svc.SetActiveModel(b, true); err != nil {
		t.Fatal(err)
	}
	if active, _, _ := db.GetActiveModel(db.DB); active.Path != b {
		t.Fatalf("active after confirm = %q, want %q", active.Path, b)
	}
	if n, _ := db.CountEmbeddings(db.DB); n != 0 {
		t.Fatalf("embeddings after a confirmed switch = %d, want the old space dropped", n)
	}
	if dim, _ := db.GetVectorDim(db.DB); dim != 384 {
		t.Fatalf("vector dim = %d, want 384", dim)
	}
}

// TestSetActiveModelSameModelIsFree makes sure re-applying the active model
// does not demand a reindex (the startup path does this on every launch).
func TestSetActiveModelSameModelIsFree(t *testing.T) {
	svc := modelTestEnv(t)
	a := writeFakeModel(t, "a.gguf", 384)
	_, _ = svc.ListModels()
	if _, err := svc.SetActiveModel(a, false); err != nil {
		t.Fatal(err)
	}
	seedEmbedding(t, 384)

	res, err := svc.SetActiveModel(a, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.NeedsRebuild {
		t.Fatal("re-applying the active model should not ask for a rebuild")
	}
	if n, _ := db.CountEmbeddings(db.DB); n != 1 {
		t.Fatalf("embeddings = %d, want the existing vector kept", n)
	}
}

// seedEmbedding stores one document with a vector of the given dimension.
func seedEmbedding(t *testing.T, dims int) {
	t.Helper()
	coll, err := db.GetOrCreateCollection(db.DB, "seed", "project", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := db.DB.Exec(
		`INSERT INTO sources (collection_id, source_type, source_path) VALUES (?, 'markdown', '/seed.md')`,
		coll)
	if err != nil {
		t.Fatal(err)
	}
	src, _ := res.LastInsertId()
	res, err = db.DB.Exec(
		`INSERT INTO documents (source_id, collection_id, chunk_index, title, content, metadata)
		 VALUES (?, ?, 0, 'seed', 'seed text', '{}')`, src, coll)
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := res.LastInsertId()
	if err := db.InsertEmbedding(db.DB, doc, make([]byte, dims*4)); err != nil {
		t.Fatal(err)
	}
}

// TestDownloadedModelNotAutoActivatedOverExistingVectors: a download must not
// silently replace the active model while vectors from the old one exist, even
// at the same dimension — the vectors would become meaningless.
func TestDownloadedModelNotAutoActivatedOverExistingVectors(t *testing.T) {
	t.Run("with existing vectors", func(t *testing.T) {
		svc := modelTestEnv(t)
		a := writeFakeModel(t, "a.gguf", 384)
		_, _ = svc.ListModels()
		if _, err := svc.SetActiveModel(a, false); err != nil {
			t.Fatal(err)
		}
		seedEmbedding(t, 384)

		bPath := writeFakeModel(t, "b.gguf", 384)
		_, _ = svc.ListModels()
		b, ok, err := db.GetModelByPath(db.DB, bPath)
		if err != nil || !ok {
			t.Fatalf("model b: ok=%v err=%v", ok, err)
		}
		svc.activateIfSafe(b)

		if active, _, _ := db.GetActiveModel(db.DB); active.Path != a {
			t.Fatalf("downloaded model auto-activated over existing vectors: %q", active.Path)
		}
		if n, _ := db.CountEmbeddings(db.DB); n != 1 {
			t.Fatalf("vectors after download = %d, want the existing one kept", n)
		}
	})

	t.Run("with nothing embedded", func(t *testing.T) {
		svc := modelTestEnv(t)
		p := writeFakeModel(t, "a.gguf", 384)
		_, _ = svc.ListModels()
		m, ok, err := db.GetModelByPath(db.DB, p)
		if err != nil || !ok {
			t.Fatalf("model: ok=%v err=%v", ok, err)
		}
		svc.activateIfSafe(m)

		if active, _, _ := db.GetActiveModel(db.DB); active.Path != p {
			t.Fatalf("fresh download should be active, got %q", active.Path)
		}
	})
}

func TestDeleteModelActiveBlocked(t *testing.T) {
	svc := modelTestEnv(t)
	a := writeFakeModel(t, "a.gguf", 1024)
	_, _ = svc.ListModels()

	if _, err := svc.SetActiveModel(a, false); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteModel(a); err == nil {
		t.Fatal("expected deleting the active model to be blocked")
	}
	if _, err := os.Stat(a); err != nil {
		t.Fatal("blocked delete must not remove the file")
	}

	// Deactivating first (switch to none isn't exposed, so register a second
	// model and make it active) then deleting the old one works.
	c := writeFakeModel(t, "c.gguf", 1024)
	_, _ = svc.ListModels()
	if _, err := svc.SetActiveModel(c, false); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteModel(a); err != nil {
		t.Fatalf("delete inactive model: %v", err)
	}
	if _, err := os.Stat(a); err == nil {
		t.Fatal("deleted model's file should be removed from models/")
	}
}

func mustFakeGGUF(t *testing.T, dims int) []byte {
	t.Helper()
	return fakeGGUFBytes(dims)
}

// modelGGUFStr writes a GGUF length-prefixed string into b.
func modelGGUFStr(b *bytes.Buffer, s string) {
	_ = binary.Write(b, binary.LittleEndian, uint64(len(s)))
	b.WriteString(s)
}
