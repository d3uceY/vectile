package indexer

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// writeTree creates the given files (slash-separated, relative to the temp
// root) and returns the root directory.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	return root
}

// relFiles returns the collected paths relative to root, slash-separated.
func relFiles(t *testing.T, root string, paths []string) []string {
	t.Helper()
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		rel, err := filepath.Rel(root, p)
		if err != nil {
			t.Fatalf("rel: %v", err)
		}
		out = append(out, filepath.ToSlash(rel))
	}
	sort.Strings(out)
	return out
}

func TestCollectFilesSkipsExcludedDirs(t *testing.T) {
	root := writeTree(t, map[string]string{
		"README.md":                           "# keep\n",
		"dev-docs/notes.md":                   "# notes\n",
		"references/other/README.md":          "# other\n",
		"node_modules/loose.md":               "# loose\n",
		"node_modules/dep/README.md":          "# dep\n",
		"frontend/node_modules/dep/readme.md": "# dep\n",
		".git/config.md":                      "# git\n",
	})

	got := relFiles(t, root, collectFiles([]string{root}, false, excludedDirNames([]string{"node_modules"})))
	want := []string{"README.md", "dev-docs/notes.md", "references/other/README.md"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("walk with node_modules excluded = %v, want %v", got, want)
	}

	got = relFiles(t, root, collectFiles([]string{root}, false, excludedDirNames([]string{"node_modules", "references"})))
	want = []string{"README.md", "dev-docs/notes.md"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("walk with node_modules+references excluded = %v, want %v", got, want)
	}

	// Excluding a nested name prunes only that directory; its parent is still
	// walked, so files sitting directly in it are still collected.
	got = relFiles(t, root, collectFiles([]string{root}, false, excludedDirNames([]string{"dep"})))
	want = []string{"README.md", "dev-docs/notes.md", "node_modules/loose.md", "references/other/README.md"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("walk with only dep excluded = %v, want %v", got, want)
	}
}

// A path the user configured explicitly is indexed even when its own name is
// on the exclusion list; the list prunes subdirectories, not the roots.
func TestCollectFilesWalksExplicitRootName(t *testing.T) {
	root := writeTree(t, map[string]string{"node_modules/keep.md": "# keep\n"})
	nm := filepath.Join(root, "node_modules")

	got := relFiles(t, nm, collectFiles([]string{nm}, false, excludedDirNames([]string{"node_modules"})))
	if strings.Join(got, ",") != "keep.md" {
		t.Fatalf("explicitly configured path was pruned: %v", got)
	}
}

func TestExcludedDirNames(t *testing.T) {
	if excludedDirNames(nil) != nil {
		t.Fatal("no names should yield a nil set")
	}
	set := excludedDirNames([]string{" node_modules ", "", "dist"})
	if !set["node_modules"] || !set["dist"] || len(set) != 2 {
		t.Fatalf("unexpected set: %v", set)
	}
}
