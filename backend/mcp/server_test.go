package mcp

import (
	"context"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"vectile/backend/config"
	"vectile/backend/services"
)

func TestCreateServerRegistersTools(t *testing.T) {
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	core := &services.Core{Cfg: cfg}
	s := CreateServer(core)
	if s == nil {
		t.Fatal("CreateServer returned nil")
	}

	tools := s.ListTools()
	want := []string{
		"vectile_search", "vectile_list_collections", "vectile_collection_info",
		"vectile_index", "vectile_prune",
	}
	if len(tools) != len(want) {
		t.Fatalf("expected %d tools, got %d", len(want), len(tools))
	}
	for _, name := range want {
		if tools[name] == nil {
			t.Errorf("tool %q not registered", name)
		}
	}
}

func TestWriteToolsGated(t *testing.T) {
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	// AllowWrite defaults to false, so the write tools must refuse to run.
	core := &services.Core{Cfg: cfg}
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "vectile_index",
			Arguments: map[string]any{"collection": "obsidian"},
		},
	}

	res, err := handleIndex(core)(context.Background(), req)
	if err != nil {
		t.Fatalf("handleIndex unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected an error result when write tools are disabled, got %+v", res)
	}

	req.Params.Name = "vectile_prune"
	pr, err := handlePrune(core)(context.Background(), req)
	if err != nil {
		t.Fatalf("handlePrune unexpected error: %v", err)
	}
	if !pr.IsError {
		t.Fatalf("expected an error result when write tools are disabled, got %+v", pr)
	}
}

func TestBuildSourceURI(t *testing.T) {
	cfg, err := config.Load(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ObsidianVaults = []string{"/home/me/MyVault"}

	// The obsidian URI encodes the platform-relative path, so compute the
	// expected value instead of hard-coding the separator.
	obsRel, err := filepath.Rel("/home/me/MyVault", "/home/me/MyVault/notes/a.md")
	if err != nil {
		t.Fatal(err)
	}
	obsWant := "obsidian://open?vault=MyVault&file=" + url.QueryEscape(obsRel)

	cases := []struct {
		name       string
		path       string
		sourceType string
		metadata   map[string]any
		want       any
	}{
		{"plain file", "/home/me/docs/note.md", "markdown", nil, "file:///home/me/docs/note.md"},
		{"code", "/home/me/repo/main.go", "code", map[string]any{"start_line": float64(42)},
			"vscode://file/home/me/repo/main.go:42"},
		{"code no line", "/home/me/repo/main.go", "code", nil,
			"vscode://file/home/me/repo/main.go:1"},
		{"obsidian", "/home/me/MyVault/notes/a.md", "markdown", nil, obsWant},
		{"calibre", "calibre:///lib/book", "calibre-description", nil, nil},
		{"commit", "git:///repo#sha", "commit", nil, nil},
		{"doc with url", "/some/path.md", "markdown", map[string]any{"url": "https://example.com/x"}, "https://example.com/x"},
	}

	for _, c := range cases {
		got := buildSourceURI(c.path, c.sourceType, c.metadata, cfg)
		if got != c.want {
			t.Errorf("%s: buildSourceURI(%q, %q) = %v, want %v", c.name, c.path, c.sourceType, got, c.want)
		}
	}
}

func TestBuildObsidianURI(t *testing.T) {
	vault := "/home/me/MyVault"
	file := "/home/me/MyVault/notes/test.md"
	rel, err := filepath.Rel(vault, file)
	if err != nil {
		t.Fatal(err)
	}
	// The encoded file path uses the platform separator (windows → %5C,
	// posix → %2F), so the expectation is computed rather than hard-coded.
	want := "obsidian://open?vault=MyVault&file=" + url.QueryEscape(rel)
	if got := buildObsidianURI(file, vault); got != want {
		t.Errorf("buildObsidianURI = %q, want %q", got, want)
	}
}

func TestDescribeCollection(t *testing.T) {
	// Human description passes through.
	if text, repos := describeCollection("my project notes"); text != "my project notes" || repos != 0 {
		t.Errorf("expected passthrough, got %q / %d", text, repos)
	}
	// Empty stays empty.
	if text, repos := describeCollection(""); text != "" || repos != 0 {
		t.Errorf("expected empty, got %q / %d", text, repos)
	}
	// Git watermark JSON is summarised into a repo count.
	watermark := `{"C:\\repo\\a": "abc123", "C:\\repo\\a:history": "def456", "C:\\repo\\b": "abc124"}`
	if text, repos := describeCollection(watermark); text != "" || repos != 2 {
		t.Errorf("expected 2 repos, got %q / %d", text, repos)
	}
}
