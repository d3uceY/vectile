package config

import (
	"os"
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
	if cfg.MCP.Transport != TransportStreamableHTTP {
		t.Fatalf("default MCP transport = %q, want %q", cfg.MCP.Transport, TransportStreamableHTTP)
	}
	if len(cfg.ProjectExcludeFolders) != 1 || cfg.ProjectExcludeFolders[0] != "node_modules" {
		t.Fatalf("project folders should skip node_modules by default, got %v", cfg.ProjectExcludeFolders)
	}
	if len(cfg.RepositoryExcludeFolders) != 0 || len(cfg.CalibreExcludeFolders) != 0 || len(cfg.ObsidianExcludeFolders) != 0 {
		t.Fatalf("skip lists should start empty, got %v / %v / %v",
			cfg.ObsidianExcludeFolders, cfg.RepositoryExcludeFolders, cfg.CalibreExcludeFolders)
	}
	if !cfg.OCR.Enabled {
		t.Fatal("OCR should default to on; it only runs for pages with no text")
	}
	if len(cfg.OCR.Languages) != 1 || cfg.OCR.Languages[0] != "eng" {
		t.Fatalf("default OCR languages = %v, want [eng]", cfg.OCR.Languages)
	}
}

// TestLoadKeepsTransportWhenKeyAbsent covers the upgrade path: a config.json
// written before mcp.transport existed must come back on the default, not as
// the empty string.
func TestLoadKeepsTransportWhenKeyAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	raw := `{"mcp": {"enabled": true, "port": 31234, "allow_write": false}}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.MCP.Enabled || cfg.MCP.Port != 31234 {
		t.Fatalf("mcp config not read: %+v", cfg.MCP)
	}
	if cfg.MCP.Transport != TransportStreamableHTTP {
		t.Fatalf("absent transport = %q, want the default %q", cfg.MCP.Transport, TransportStreamableHTTP)
	}
}

func TestNormalizeTransport(t *testing.T) {
	cases := map[string]string{
		"":                      TransportStreamableHTTP,
		"nonsense":              TransportStreamableHTTP,
		TransportStreamableHTTP: TransportStreamableHTTP,
		TransportSSE:            TransportSSE,
	}
	for in, want := range cases {
		if got := NormalizeTransport(in); got != want {
			t.Errorf("NormalizeTransport(%q) = %q, want %q", in, got, want)
		}
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
	cfg.MCP.Transport = TransportSSE
	cfg.ProjectExcludeFolders = []string{"node_modules", "references"}
	cfg.RepositoryExcludeFolders = []string{"testdata", "go.sum"}
	cfg.CalibreExcludeFolders = []string{"Samples"}
	cfg.OCR.Enabled = false
	cfg.OCR.Languages = []string{"eng", "deu"}

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
	if got.MCP.Transport != TransportSSE {
		t.Fatalf("mcp transport not round-tripped: %+v", got.MCP)
	}
	if len(got.ProjectExcludeFolders) != 2 || got.ProjectExcludeFolders[1] != "references" {
		t.Fatalf("project exclude folders not round-tripped: %v", got.ProjectExcludeFolders)
	}
	if len(got.RepositoryExcludeFolders) != 2 || got.RepositoryExcludeFolders[1] != "go.sum" {
		t.Fatalf("repository exclude folders not round-tripped: %v", got.RepositoryExcludeFolders)
	}
	if len(got.CalibreExcludeFolders) != 1 || got.CalibreExcludeFolders[0] != "Samples" {
		t.Fatalf("calibre exclude folders not round-tripped: %v", got.CalibreExcludeFolders)
	}
	// Save whitelists keys explicitly, so a section missing from that list
	// silently never persists. This is the check that catches it.
	if got.OCR.Enabled {
		t.Fatal("ocr.enabled not round-tripped")
	}
	if len(got.OCR.Languages) != 2 || got.OCR.Languages[1] != "deu" {
		t.Fatalf("ocr.languages not round-tripped: %v", got.OCR.Languages)
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
