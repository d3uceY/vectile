package config

import (
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ChunkSizeTokens != 500 {
		t.Fatalf("default chunk size = %d", cfg.ChunkSizeTokens)
	}
	if cfg.SearchDefaults.TopK != 10 {
		t.Fatalf("default top_k = %d", cfg.SearchDefaults.TopK)
	}
	if !cfg.SkipCloudPlaceholders {
		t.Fatal("cloud placeholders should default to skipped")
	}
	if cfg.MCP.Enabled {
		t.Fatal("MCP server should default to disabled")
	}
	if cfg.MCP.Port != 31123 {
		t.Fatalf("default MCP port = %d, want 31123", cfg.MCP.Port)
	}
	if len(cfg.ProjectExcludeFolders) != 1 || cfg.ProjectExcludeFolders[0] != "node_modules" {
		t.Fatalf("project folders should skip node_modules by default, got %v", cfg.ProjectExcludeFolders)
	}
}

func TestSaveLoadRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := defaults()
	cfg.ObsidianVaults = []string{"~/vault"}
	cfg.ChunkSizeTokens = 300
	cfg.GUI.AutoReindex = true
	cfg.GUI.AutoReindexIntervalMinutes = 45
	cfg.MCP.Enabled = true
	cfg.MCP.Port = 40404
	cfg.ProjectExcludeFolders = []string{"node_modules", "references"}

	if err := Save(cfg, path); err != nil {
		t.Fatal(err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.ChunkSizeTokens != 300 {
		t.Fatalf("chunk size = %d", got.ChunkSizeTokens)
	}
	if len(got.ObsidianVaults) != 1 || got.ObsidianVaults[0] == "~/vault" {
		t.Fatalf("expected ~ expanded on load, got %v", got.ObsidianVaults)
	}
	if !got.GUI.AutoReindex || got.GUI.AutoReindexIntervalMinutes != 45 {
		t.Fatalf("gui config not round-tripped: %+v", got.GUI)
	}
	if !got.MCP.Enabled || got.MCP.Port != 40404 {
		t.Fatalf("mcp config not round-tripped: %+v", got.MCP)
	}
	if len(got.ProjectExcludeFolders) != 2 || got.ProjectExcludeFolders[1] != "references" {
		t.Fatalf("project exclude folders not round-tripped: %v", got.ProjectExcludeFolders)
	}
}

func TestIsCollectionEnabled(t *testing.T) {
	cfg := defaults()
	cfg.DisabledCollections = []string{"obsidian"}
	if cfg.IsCollectionEnabled("obsidian") {
		t.Fatal("obsidian should be disabled")
	}
	if !cfg.IsCollectionEnabled("calibre") {
		t.Fatal("calibre should be enabled")
	}
	// Re-reading after a change must reflect the new value.
	cfg.DisabledCollections = append(cfg.DisabledCollections, "calibre")
	if cfg.IsCollectionEnabled("calibre") {
		t.Fatal("calibre should be disabled after update")
	}
}

func TestCollectionNameConflicts(t *testing.T) {
	cfg := defaults()
	cfg.Repositories["shared"] = []string{"/a"}
	cfg.Projects["shared"] = []string{"/b"}
	conflicts := cfg.CollectionNameConflicts()
	if len(conflicts) != 1 || conflicts[0].Name != "shared" {
		t.Fatalf("expected one conflict on 'shared', got %v", conflicts)
	}
}
