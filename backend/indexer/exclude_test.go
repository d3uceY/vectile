package indexer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"vectile/backend/config"
	"vectile/backend/db"
)

func TestPathHasExcludedName(t *testing.T) {
	exclude := excludedNames([]string{"dist", "go.sum"})

	cases := []struct {
		rel  string
		want bool
	}{
		{"main.go", false},
		{"dist/bundle.js", true},
		{"src/dist/bundle.js", true},
		{"go.sum", true},
		{"sub/dir/go.sum", true},
		{"nodist/bundle.js", false}, // whole names only, not substrings
		{"src/go.sum.bak", false},   // and not prefixes
	}
	for _, c := range cases {
		if got := pathHasExcludedName(c.rel, exclude); got != c.want {
			t.Errorf("pathHasExcludedName(%q) = %v, want %v", c.rel, got, c.want)
		}
	}

	if pathHasExcludedName("dist/bundle.js", nil) {
		t.Error("a nil skip list should exclude nothing")
	}
}

// The repository list is additive to git's built-in skips, and it covers both
// folder and file components of a repo-relative path.
func TestShouldIndexFileRespectsSkipList(t *testing.T) {
	exclude := excludedNames([]string{"testdata", "notes.md"})

	if !shouldIndexFile("main.go", exclude) {
		t.Error("main.go should be indexed")
	}
	if shouldIndexFile("testdata/sample.go", exclude) {
		t.Error("files under testdata should be skipped")
	}
	if shouldIndexFile("docs/notes.md", exclude) {
		t.Error("an excluded file name should be skipped at any depth")
	}

	// The built-in skips stay in force with no user list at all.
	if shouldIndexFile("node_modules/dep/index.js", nil) {
		t.Error("node_modules should stay skipped")
	}
	if shouldIndexFile("go.sum", nil) {
		t.Error("lock files should stay skipped")
	}
}

func TestWalkVaultSkipsExcludedNames(t *testing.T) {
	root := writeTree(t, map[string]string{
		"index.md":           "# keep\n",
		"CHANGELOG.md":       "# drop\n",
		"templates/daily.md": "# drop\n",
		"notes/daily.md":     "# keep\n",
	})

	got := relFiles(t, root, walkVault(root, excludedNames([]string{"templates", "CHANGELOG.md"}), false))
	want := []string{"index.md", "notes/daily.md"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("vault walk = %v, want %v", got, want)
	}
}

// The project skip list has to survive the whole pipeline, not just the walk:
// an excluded folder or file name must leave no rows behind.
func TestIndexProjectSkipsExcludedNames(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"keep.md":            "# keep\n",
		"CHANGELOG.md":       "# drop\n",
		"vendored/deep.md":   "# drop\n",
		"notes/CHANGELOG.md": "# drop\n",
		"notes/other.md":     "# keep\n",
	})

	if err := db.Open(filepath.Join(t.TempDir(), "skip.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	cfg := &config.Config{
		EmbeddingBatchSize:    32,
		ChunkSizeTokens:       500,
		ChunkOverlapTokens:    50,
		ProjectExcludeFolders: []string{"vendored", "CHANGELOG.md"},
	}

	result := IndexProject(context.Background(), db.DB, cfg, "test", []string{dir}, false, nil, &stubEmbedder{})
	if result.Errors > 0 {
		t.Fatalf("index errors: %v", result.ErrorMessages)
	}
	if result.TotalFound != 2 {
		t.Fatalf("files found = %d, want 2 (only keep.md and notes/other.md)", result.TotalFound)
	}

	var paths []string
	rows, err := db.DB.Query(`SELECT source_path FROM sources ORDER BY source_path`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, filepath.Base(p))
	}
	if strings.Join(paths, ",") != "keep.md,other.md" {
		t.Fatalf("indexed sources = %v, want keep.md and other.md only", paths)
	}
}

// gitInTest runs one git command in dir with a throwaway identity, and skips
// the test when git is unavailable. Nothing here touches the real user config.
func gitInTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=vectile test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=vectile test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_NOSYSTEM=1",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("git %v failed (%v): %s", args, err, out)
	}
}

// The repository skip list has to reach the git tree filter, not just the
// shared matcher: IndexGitRepo must read RepositoryExcludeFolders.
func TestIndexGitRepoSkipsExcludedNames(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"main.go":          "package main\n\nfunc main() {}\n",
		"notes.md":         "# drop\n",
		"vendored/deep.go": "package vendored\n",
	})
	git := func(args ...string) { gitInTest(t, dir, args...) }
	git("init", "-q")
	git("add", "-A")
	git("commit", "-qm", "init")

	if err := db.Open(filepath.Join(t.TempDir(), "repo.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	cfg := &config.Config{
		EmbeddingBatchSize:       32,
		ChunkSizeTokens:          500,
		ChunkOverlapTokens:       50,
		RepositoryExcludeFolders: []string{"vendored", "notes.md"},
	}

	result := IndexGitRepo(context.Background(), db.DB, cfg, dir, "repo-test", false, false, nil, &stubEmbedder{})
	if result.Errors > 0 {
		t.Fatalf("index errors: %v", result.ErrorMessages)
	}
	if result.TotalFound != 1 {
		t.Fatalf("tracked files = %d, want 1 (only main.go)", result.TotalFound)
	}
}
