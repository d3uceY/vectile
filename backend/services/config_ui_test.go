package services

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"vectile/backend/config"
	"vectile/backend/startup"
)

// These tests drive config the way the Settings and Library UI do, asserting on
// both the live config and the file a restart would read.

// configUIEnv wires a Core and IndexService around a real config file.
func configUIEnv(t *testing.T) (*Core, *IndexService, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	core := NewCore(cfg)
	core.CfgPath = path
	return core, NewIndexService(core), path
}

// uiDraft is what the Settings form holds: a copy of the live config.
func uiDraft(t *testing.T, core *Core) config.Config {
	t.Helper()
	return *core.Config().Clone()
}

// readConfigFile loads what a restart would see.
func readConfigFile(t *testing.T, path string) *config.Config {
	t.Helper()
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// currentAutostart returns the machine's state, so a test can submit it
// unchanged instead of writing a real autostart entry.
func currentAutostart() bool {
	on, err := startup.IsEnabled()
	return err == nil && on
}

// TestSettingsSaveRoundTrip is the common case: several unrelated settings
// changed and saved at once. The model-owned fields must survive a draft holding
// stale copies of them.
func TestSettingsSaveRoundTrip(t *testing.T) {
	core, svc, path := configUIEnv(t)

	// A model switch wrote these just before the user opened the form.
	if err := core.UpdateConfig(func(c *config.Config) {
		c.ActiveModel = "/models/bge-small.gguf"
		c.EmbeddingModel = "bge-small"
		c.EmbeddingBatchSize = 64
	}); err != nil {
		t.Fatal(err)
	}
	held := core.Config() // a snapshot the auto-reindex loop took this tick

	form := uiDraft(t, core)
	form.GUI.AutoReindex = true
	form.GUI.AutoReindexIntervalMinutes = 15
	form.GUI.StartOnLogin = currentAutostart()
	form.ChunkSizeTokens = 750
	form.ChunkOverlapTokens = 80
	form.MCP.Enabled = true
	form.MCP.Port = 4545
	form.OCR.Languages = []string{"eng", "deu"}
	form.SearchDefaults.TopK = 12
	form.ObsidianExcludeFolders = []string{".trash"}
	form.SkipCloudPlaceholders = false
	// Stale copies of the model-owned values, as an older draft would hold.
	form.ActiveModel = ""
	form.EmbeddingModel = ""
	form.EmbeddingBatchSize = 0

	if err := svc.SetConfig(form); err != nil {
		t.Fatal(err)
	}

	for _, cfg := range []*config.Config{core.Config(), readConfigFile(t, path)} {
		if !cfg.GUI.AutoReindex || cfg.GUI.AutoReindexIntervalMinutes != 15 {
			t.Fatalf("auto-reindex not saved: %+v", cfg.GUI)
		}
		if cfg.ChunkSizeTokens != 750 || cfg.ChunkOverlapTokens != 80 {
			t.Fatalf("chunking not saved: %d/%d", cfg.ChunkSizeTokens, cfg.ChunkOverlapTokens)
		}
		if !cfg.MCP.Enabled || cfg.MCP.Port != 4545 {
			t.Fatalf("mcp not saved: %+v", cfg.MCP)
		}
		if len(cfg.OCR.Languages) != 2 {
			t.Fatalf("ocr languages not saved: %v", cfg.OCR.Languages)
		}
		if cfg.SearchDefaults.TopK != 12 {
			t.Fatalf("search defaults not saved: %+v", cfg.SearchDefaults)
		}
		if len(cfg.ObsidianExcludeFolders) != 1 || cfg.ObsidianExcludeFolders[0] != ".trash" {
			t.Fatalf("exclude folders not saved: %v", cfg.ObsidianExcludeFolders)
		}
		if cfg.SkipCloudPlaceholders {
			t.Fatal("skip_cloud_placeholders not saved")
		}
		// Owned by the model service, so the stale draft must not win.
		if cfg.ActiveModel != "/models/bge-small.gguf" || cfg.EmbeddingModel != "bge-small" || cfg.EmbeddingBatchSize != 64 {
			t.Fatalf("model-owned config reverted by a settings save: %q/%q/%d",
				cfg.ActiveModel, cfg.EmbeddingModel, cfg.EmbeddingBatchSize)
		}
	}

	// The snapshot the reindex loop already holds is untouched.
	if held.ChunkSizeTokens == 750 {
		t.Fatal("a published snapshot changed under a reader")
	}
}

// TestSourcePathEditsFromSettings adds and removes folders, and checks neither
// loses settings saved in between.
func TestSourcePathEditsFromSettings(t *testing.T) {
	core, svc, path := configUIEnv(t)

	form := uiDraft(t, core)
	form.GUI.AutoReindex = true
	form.GUI.AutoReindexIntervalMinutes = 20
	form.GUI.StartOnLogin = currentAutostart()
	if err := svc.SetConfig(form); err != nil {
		t.Fatal(err)
	}

	for _, add := range [][3]string{
		{"project", "Notes", "/docs/notes"},
		{"project", "Notes", "/docs/notes"}, // picked twice: one entry
		{"vault", "", "/vault"},
		{"repo", "vectile", "/src/vectile"},
		{"calibre", "", "/books"},
	} {
		if err := svc.AddSourcePath(add[0], add[1], add[2]); err != nil {
			t.Fatalf("add %q: %v", add[2], err)
		}
	}

	for _, cfg := range []*config.Config{core.Config(), readConfigFile(t, path)} {
		if got := cfg.Projects["Notes"]; len(got) != 1 || got[0] != "/docs/notes" {
			t.Fatalf("project path: %v", got)
		}
		if len(cfg.ObsidianVaults) != 1 || cfg.ObsidianVaults[0] != "/vault" {
			t.Fatalf("vault path: %v", cfg.ObsidianVaults)
		}
		if len(cfg.Repositories["vectile"]) != 1 || cfg.Repositories["vectile"][0] != "/src/vectile" {
			t.Fatalf("repo path: %v", cfg.Repositories)
		}
		if len(cfg.CalibreLibraries) != 1 {
			t.Fatalf("calibre library: %v", cfg.CalibreLibraries)
		}
		// Settings saved before the source edits are still there.
		if !cfg.GUI.AutoReindex || cfg.GUI.AutoReindexIntervalMinutes != 20 {
			t.Fatalf("settings lost when adding a source: %+v", cfg.GUI)
		}
	}

	if err := svc.RemoveSourcePath("project", "Notes", "/docs/notes"); err != nil {
		t.Fatal(err)
	}
	if got := core.Config().Projects["Notes"]; len(got) != 0 {
		t.Fatalf("removed path still present: %v", got)
	}
	if got := readConfigFile(t, path).Projects["Notes"]; len(got) != 0 {
		t.Fatalf("removed path still on disk: %v", got)
	}
	if len(core.Config().ObsidianVaults) != 1 {
		t.Fatal("removing one source dropped another")
	}

	// An unknown kind is rejected without disturbing the config.
	if err := svc.AddSourcePath("bogus", "", "/x"); err == nil {
		t.Fatal("unknown source kind should be rejected")
	}
	if len(readConfigFile(t, path).ObsidianVaults) != 1 {
		t.Fatal("a rejected add changed the config")
	}
}

// TestToggleCollectionEnabledFromLibrary covers the enable/disable switch in
// the Library view, including toggling the same collection twice.
func TestToggleCollectionEnabledFromLibrary(t *testing.T) {
	core, svc, path := configUIEnv(t)

	if err := svc.ToggleCollectionEnabled("obsidian", false); err != nil {
		t.Fatal(err)
	}
	if core.Config().IsCollectionEnabled("obsidian") {
		t.Fatal("disabled collection still enabled live")
	}
	if readConfigFile(t, path).IsCollectionEnabled("obsidian") {
		t.Fatal("disabled collection still enabled on disk")
	}

	// Re-disabling must not add the name twice.
	if err := svc.ToggleCollectionEnabled("obsidian", false); err != nil {
		t.Fatal(err)
	}
	if got := core.Config().DisabledCollections; len(got) != 1 {
		t.Fatalf("disabled list = %v, want one entry", got)
	}

	if err := svc.ToggleCollectionEnabled("obsidian", true); err != nil {
		t.Fatal(err)
	}
	if !core.Config().IsCollectionEnabled("obsidian") {
		t.Fatal("re-enabled collection is still disabled")
	}
	if got := readConfigFile(t, path).DisabledCollections; len(got) != 0 {
		t.Fatalf("disabled list on disk = %v, want empty", got)
	}
}

// TestSettingsSaveKeepsActiveModel is the sequence "switch model, then tweak
// settings": the model service owns ActiveModel, and a stale form must not
// revert it.
func TestSettingsSaveKeepsActiveModel(t *testing.T) {
	svc := modelTestEnv(t)
	a := writeFakeModel(t, "a.gguf", 384)
	if _, err := svc.ListModels(); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetActiveModel(a, false); err != nil {
		t.Fatal(err)
	}

	idx := NewIndexService(svc.core)
	form := *svc.core.Config().Clone()
	form.ActiveModel = ""
	form.EmbeddingModel = ""
	form.EmbeddingBatchSize = 0
	form.GUI.AutoReindex = true
	form.GUI.StartOnLogin = currentAutostart()
	if err := idx.SetConfig(form); err != nil {
		t.Fatal(err)
	}

	live := svc.core.Config()
	if live.ActiveModel != a {
		t.Fatalf("active model reverted by a settings save: %q, want %q", live.ActiveModel, a)
	}
	if live.EmbeddingBatchSize != defaultBatchSize {
		t.Fatalf("batch size = %d, want %d", live.EmbeddingBatchSize, defaultBatchSize)
	}
	if !live.GUI.AutoReindex {
		t.Fatal("settings change lost")
	}
	if onDisk := readConfigFile(t, svc.core.CfgPath); onDisk.ActiveModel != a {
		t.Fatalf("active model on disk = %q, want %q", onDisk.ActiveModel, a)
	}
}

// TestFailedSaveLeavesConfigUntouched: an unwritable path must fail without
// publishing, so the app keeps running the config that is actually on disk.
func TestFailedSaveLeavesConfigUntouched(t *testing.T) {
	core, svc, _ := configUIEnv(t)
	before := core.Config()

	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	core.CfgPath = filepath.Join(blocker, "config.json")

	form := *before.Clone()
	form.ChunkSizeTokens = 777
	form.GUI.StartOnLogin = currentAutostart()
	if err := svc.SetConfig(form); err == nil {
		t.Fatal("expected the save to fail")
	}
	if got := core.Config().ChunkSizeTokens; got != before.ChunkSizeTokens {
		t.Fatalf("failed save was published: chunk size = %d", got)
	}
}

// TestConcurrentSourceAddsAreNotLost: simultaneous adds must all land, because
// each update reads what the previous one published.
func TestConcurrentSourceAddsAreNotLost(t *testing.T) {
	core, svc, path := configUIEnv(t)

	const n = 20
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := svc.AddSourcePath("project", "Notes", fmt.Sprintf("/p/%d", i)); err != nil {
				t.Errorf("add %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	if got := len(core.Config().Projects["Notes"]); got != n {
		t.Fatalf("live project paths = %d, want %d", got, n)
	}
	if got := len(readConfigFile(t, path).Projects["Notes"]); got != n {
		t.Fatalf("on-disk project paths = %d, want %d", got, n)
	}
}
