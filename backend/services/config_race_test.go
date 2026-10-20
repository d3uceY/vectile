package services

import (
	"path/filepath"
	"sync"
	"testing"

	"vectile/backend/config"
)

// TestConfigAccessIsRaceFree runs readers against a stream of saves: the shape
// that raced. Run with -race; without the detector it still pins the snapshot
// contract.
func TestConfigAccessIsRaceFree(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	core := NewCore(cfg)
	core.CfgPath = path

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Readers: the read shape of the auto-reindex loop and of a search.
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				c := core.Config()
				_ = c.GUI.AutoReindex
				_ = c.GUI.AutoReindexIntervalMinutes
				_ = c.IsCollectionEnabled("obsidian")
				_ = len(c.Projects["notes"])
			}
		}()
	}

	// Writer: a settings save, which publishes a whole new config.
	for i := 0; i < 50; i++ {
		if err := core.UpdateConfig(func(c *config.Config) {
			c.GUI.AutoReindexIntervalMinutes = 60
			c.ObsidianVaults = append(c.ObsidianVaults, "/vault")
			c.Projects["notes"] = append(c.Projects["notes"], "/notes")
		}); err != nil {
			t.Fatal(err)
		}
	}
	close(stop)
	wg.Wait()

	// A snapshot taken before a save keeps its values: readers see the previous
	// config or the new one, never a half-updated struct.
	before := core.Config()
	if err := core.UpdateConfig(func(c *config.Config) { c.EmbeddingBatchSize = 99 }); err != nil {
		t.Fatal(err)
	}
	if before.EmbeddingBatchSize == 99 {
		t.Fatal("a published snapshot was mutated in place")
	}
	if got := core.Config().EmbeddingBatchSize; got != 99 {
		t.Fatalf("EmbeddingBatchSize after save = %d, want 99", got)
	}
	if got := core.Config().Projects["notes"]; len(got) != 50 {
		t.Fatalf("saved project paths = %d, want 50", len(got))
	}
}

// TestUpdateConfigSavesWithoutTouchingCallers: the mutator's own slice must not
// end up inside the published config.
func TestUpdateConfigSavesWithoutTouchingCallers(t *testing.T) {
	core := NewCore(&config.Config{Projects: map[string][]string{}, GUI: config.GUIConfig{AutoReindex: true}})
	core.CfgPath = filepath.Join(t.TempDir(), "config.json")

	paths := []string{"/one"}
	if err := core.UpdateConfig(func(c *config.Config) {
		c.Projects["notes"] = paths
	}); err != nil {
		t.Fatal(err)
	}

	paths[0] = "/mutated"
	if got := core.Config().Projects["notes"][0]; got != "/one" {
		t.Fatalf("published config aliases the caller's slice: %q", got)
	}
}
