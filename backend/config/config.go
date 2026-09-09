// Package config loads and saves the vectile configuration — a single JSON
// file (config.json) in the app-data directory. Absent fields fall back to
// defaults; unknown keys survive a save so the file is never clobbered.
package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SearchDefaults holds the hybrid search parameters surfaced in Settings.
type SearchDefaults struct {
	TopK         int     `json:"top_k"`
	RRFK         int     `json:"rrf_k"`
	VectorWeight float64 `json:"vector_weight"`
	FTSWeight    float64 `json:"fts_weight"`
}

// MascotConfig controls which sidebar-mascot (Vexter) interactions are shown.
// Each flag gates one resting-state trigger; all three off hides Vexter for
// every moment at once.
type MascotConfig struct {
	ShowSearching bool `json:"show_searching"`
	ShowIndexing  bool `json:"show_indexing"`
	ShowNothing   bool `json:"show_nothing"`
}

// GUIConfig holds app-level (non-source) settings.
type GUIConfig struct {
	AutoReindex                bool         `json:"auto_reindex"`
	AutoReindexIntervalMinutes int          `json:"auto_reindex_interval_minutes"`
	StartOnLogin               bool         `json:"start_on_login"`
	Mascot                     MascotConfig `json:"mascot"`
}

// MCPConfig holds the in-app MCP (Model Context Protocol) server settings.
// The server binds to 127.0.0.1 only. Search tools are always available;
// AllowWrite gates the write tools (index/prune) that can change the library.
type MCPConfig struct {
	Enabled    bool `json:"enabled"`
	Port       int  `json:"port"`
	AllowWrite bool `json:"allow_write"`
}

// Config is the full application configuration.
type Config struct {
	EmbeddingModel            string              `json:"embedding_model"`
	ActiveModel               string              `json:"active_model"`
	EmbeddingBatchSize        int                 `json:"embedding_batch_size"`
	ChunkSizeTokens           int                 `json:"chunk_size_tokens"`
	ChunkOverlapTokens        int                 `json:"chunk_overlap_tokens"`
	ObsidianVaults            []string            `json:"obsidian_vaults"`
	ObsidianExcludeFolders    []string            `json:"obsidian_exclude_folders"`
	CalibreLibraries          []string            `json:"calibre_libraries"`
	Repositories              map[string][]string `json:"repositories"`
	Projects                  map[string][]string `json:"projects"`
	DisabledCollections       []string            `json:"disabled_collections"`
	SkipCloudPlaceholders     bool                `json:"skip_cloud_placeholders"`
	GitHistoryInMonths        int                 `json:"git_history_in_months"`
	GitCommitSubjectBlacklist []string            `json:"git_commit_subject_blacklist"`
	SearchDefaults            SearchDefaults      `json:"search_defaults"`
	GUI                       GUIConfig           `json:"gui"`
	MCP                       MCPConfig           `json:"mcp"`
}

// IsCollectionEnabled reports whether the named collection is not disabled.
// It scans DisabledCollections directly (the list is tiny) so it never holds
// mutable shared state: the old lazy-built cache was written and read
// concurrently by the index goroutine and frontend handlers, which could
// trigger a fatal "concurrent map read and map write" and crash the app.
func (c *Config) IsCollectionEnabled(name string) bool {
	for _, n := range c.DisabledCollections {
		if n == name {
			return false
		}
	}
	return true
}

// systemCollections are the reserved names owned by the built-in indexers.
var systemCollections = []string{"obsidian", "calibre"}

// NameConflict is a collection name claimed by more than one source.
type NameConflict struct {
	Name  string
	Kinds []string
}

func (c NameConflict) String() string {
	return fmt.Sprintf("%q is configured under %s", c.Name, strings.Join(c.Kinds, " and "))
}

// CollectionNameConflicts reports names claimed by more than one source.
func (c *Config) CollectionNameConflicts() []NameConflict {
	kinds := map[string][]string{}
	add := func(name, kind string) {
		for _, k := range kinds[name] {
			if k == kind {
				return
			}
		}
		kinds[name] = append(kinds[name], kind)
	}

	for name := range c.Repositories {
		add(name, "repositories")
	}
	for name := range c.Projects {
		add(name, "projects")
	}
	for _, name := range systemCollections {
		if _, isRepo := c.Repositories[name]; isRepo {
			add(name, "system collections")
		}
		if _, isProject := c.Projects[name]; isProject {
			add(name, "system collections")
		}
	}

	var conflicts []NameConflict
	for name, ks := range kinds {
		if len(ks) > 1 {
			sort.Strings(ks)
			conflicts = append(conflicts, NameConflict{Name: name, Kinds: ks})
		}
	}
	sort.Slice(conflicts, func(i, j int) bool { return conflicts[i].Name < conflicts[j].Name })
	return conflicts
}

// Load reads the config at path ("" → default) and returns a Config with
// defaults applied for any missing fields.
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}

	cfg := defaults()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	// Unmarshal straight into the struct: absent fields keep their defaults.
	if err := json.Unmarshal(data, cfg); err != nil {
		slog.Error("failed to parse config, using defaults", "path", path, "err", err)
		return cfg, nil
	}
	expandConfigPaths(cfg)

	for _, conflict := range cfg.CollectionNameConflicts() {
		slog.Warn("collection name conflict: indexing will merge two corpora until renamed",
			"conflict", conflict.String())
	}
	return cfg, nil
}

// Save writes cfg to path ("" → default), preserving unknown keys.
func Save(cfg *Config, path string) error {
	if path == "" {
		path = DefaultConfigPath
	}

	existing := make(map[string]any)
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &existing)
	}
	// Overlay current values, keeping any keys this build doesn't know about.
	existing["embedding_model"] = cfg.EmbeddingModel
	existing["active_model"] = cfg.ActiveModel
	existing["embedding_batch_size"] = cfg.EmbeddingBatchSize
	existing["chunk_size_tokens"] = cfg.ChunkSizeTokens
	existing["chunk_overlap_tokens"] = cfg.ChunkOverlapTokens
	existing["obsidian_vaults"] = cfg.ObsidianVaults
	existing["obsidian_exclude_folders"] = cfg.ObsidianExcludeFolders
	existing["calibre_libraries"] = cfg.CalibreLibraries
	existing["repositories"] = cfg.Repositories
	existing["projects"] = cfg.Projects
	existing["disabled_collections"] = cfg.DisabledCollections
	existing["skip_cloud_placeholders"] = cfg.SkipCloudPlaceholders
	existing["git_history_in_months"] = cfg.GitHistoryInMonths
	existing["git_commit_subject_blacklist"] = cfg.GitCommitSubjectBlacklist
	existing["search_defaults"] = cfg.SearchDefaults
	existing["gui"] = cfg.GUI
	existing["mcp"] = cfg.MCP

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// DefaultConfigPath is the config location inside the app-data dir.
var DefaultConfigPath = ""

// defaults returns a Config with all default values applied.
func defaults() *Config {
	return &Config{
		EmbeddingModel:            "bge-m3",
		EmbeddingBatchSize:        32,
		ChunkSizeTokens:           500,
		ChunkOverlapTokens:        50,
		ObsidianVaults:            []string{},
		ObsidianExcludeFolders:    []string{},
		CalibreLibraries:          []string{},
		Repositories:              map[string][]string{},
		Projects:                  map[string][]string{},
		DisabledCollections:       []string{},
		SkipCloudPlaceholders:     true,
		GitHistoryInMonths:        6,
		GitCommitSubjectBlacklist: []string{},
		SearchDefaults: SearchDefaults{
			TopK:         10,
			RRFK:         60,
			VectorWeight: 0.7,
			FTSWeight:    0.3,
		},
		GUI: GUIConfig{
			AutoReindex:                false,
			AutoReindexIntervalMinutes: 60,
			StartOnLogin:               false,
			Mascot: MascotConfig{
				ShowSearching: true,
				ShowIndexing:  true,
				ShowNothing:   true,
			},
		},
		MCP: MCPConfig{
			Enabled: false,
			Port:    31123,
		},
	}
}

func expandConfigPaths(cfg *Config) {
	for i, v := range cfg.ObsidianVaults {
		cfg.ObsidianVaults[i] = expandPath(v)
	}
	for i, v := range cfg.CalibreLibraries {
		cfg.CalibreLibraries[i] = expandPath(v)
	}
	for name, paths := range cfg.Repositories {
		expanded := make([]string, len(paths))
		for i, p := range paths {
			expanded[i] = expandPath(p)
		}
		cfg.Repositories[name] = expanded
	}
	for name, paths := range cfg.Projects {
		expanded := make([]string, len(paths))
		for i, p := range paths {
			expanded[i] = expandPath(p)
		}
		cfg.Projects[name] = expanded
	}
}

func expandPath(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	if p == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	return p
}

// UnexpandPath converts an absolute path back to ~/… form under $HOME.
func UnexpandPath(p string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	rel, err := filepath.Rel(home, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return p
	}
	return "~/" + rel
}
