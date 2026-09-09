package indexer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"vectile/backend/chunker"
	"vectile/backend/config"
	"vectile/backend/parser"
)

// Exclude patterns for files that shouldn't be indexed even if tracked.
var excludePatterns = map[string]bool{
	".DS_Store":           true,
	".terraform.lock.hcl": true,
	"go.sum":              true,
	"package-lock.json":   true,
	"yarn.lock":           true,
	"pnpm-lock.yaml":      true,
	"Cargo.lock":          true,
	"poetry.lock":         true,
	"uv.lock":             true,
}

var excludeDirPatterns = map[string]bool{
	".idea":         true,
	".vscode":       true,
	"node_modules":  true,
	"__pycache__":   true,
	".mypy_cache":   true,
	".pytest_cache": true,
	".tox":          true,
	"dist":          true,
	"build":         true,
	".egg-info":     true,
	"vendor":        true,
	".terraform":    true,
	"cdk.out":       true,
}

const watermarkPrefix = "git:"

// CommitInfo holds parsed git commit metadata.
type CommitInfo struct {
	SHA         string
	AuthorName  string
	AuthorEmail string
	AuthorDate  string
	Subject     string
}

// FileChange represents a single file change from a git commit.
type FileChange struct {
	FilePath  string
	Additions int
	Deletions int
	IsBinary  bool
}

// IndexGitRepo indexes a git repository using tree-sitter for code parsing,
// plus optional commit history.
func IndexGitRepo(ctx context.Context, conn *sql.DB, cfg *config.Config, repoPath, collectionName string, force, indexHistory bool, progress ProgressCallback, embedder Embedder) *IndexResult {
	repoPath, _ = filepath.Abs(repoPath)

	if err := CheckNameConflict(cfg, collectionName); err != nil {
		slog.Error("refusing to index", "name", collectionName, "err", err)
		return failedResult(err)
	}

	if !isGitRepo(repoPath) {
		slog.Error("not a git repository", "path", repoPath)
		return &IndexResult{Errors: 1, ErrorMessages: []string{"not a git repository"}}
	}

	headSHA := getHeadSHA(repoPath)
	if headSHA == "" {
		slog.Warn("skipping repo with no commits", "path", repoPath)
		return &IndexResult{}
	}

	collectionID, err := getOrCreate(conn, collectionName, "code")
	if err != nil {
		return failedResult(err)
	}

	// Read existing watermarks (per-repo index state, stored in description).
	var desc sql.NullString
	_ = conn.QueryRow("SELECT description FROM collections WHERE id = ?", collectionID).Scan(&desc)
	watermarks := parseWatermarks(desc.String)
	oldSHA := watermarks[repoPath]

	var filesToIndex, filesToDelete []string
	if !force && oldSHA != "" {
		if oldSHA == headSHA {
			slog.Info("no new commits since last index", "sha", truncateSHA(headSHA))
			if indexHistory {
				return indexGitHistory(ctx, conn, cfg, repoPath, collectionID, force, cfg.GitHistoryInMonths, false, embedder)
			}
			return &IndexResult{}
		}

		if commitExists(repoPath, oldSHA) {
			trackedSet := make(map[string]bool)
			for _, f := range gitLsFiles(repoPath) {
				trackedSet[f] = true
			}
			for _, f := range gitDiffNames(repoPath, oldSHA, "HEAD") {
				if trackedSet[f] {
					filesToIndex = append(filesToIndex, f)
				} else {
					filesToDelete = append(filesToDelete, f)
				}
			}
		} else {
			slog.Warn("previous watermark commit not found, doing full index", "sha", truncateSHA(oldSHA))
			filesToIndex = gitLsFiles(repoPath)
		}
	} else {
		filesToIndex = gitLsFiles(repoPath)
	}

	for _, relPath := range filesToDelete {
		deleteSource(conn, collectionID, filepath.Join(repoPath, relPath))
	}

	var indexable []string
	for _, f := range filesToIndex {
		if shouldIndexFile(f) {
			indexable = append(indexable, f)
		}
	}

	result := &IndexResult{TotalFound: len(indexable)}
	cleared := clearRepoForRebuild(conn, collectionID, repoPath, force)

	indexItemsBatched(ctx, conn, cfg, collectionID, collectionName, len(indexable),
		func(i int) *indexItem { return codeFileToItem(conn, cfg, repoPath, indexable[i], collectionID, force) },
		embedder, result, progress, cleared)

	// Advance the watermark only on a clean run, so a failed file is retried
	// next run instead of being skipped forever.
	if result.Errors == 0 {
		watermarks[repoPath] = headSHA
		_, _ = conn.Exec("UPDATE collections SET description = ? WHERE id = ?",
			makeWatermarks(watermarks), collectionID)
	} else {
		slog.Warn("not advancing watermark: run had errors, files will be retried next run",
			"errors", result.Errors, "repo", repoPath)
	}

	if indexHistory {
		result.Merge(indexGitHistory(ctx, conn, cfg, repoPath, collectionID, force, cfg.GitHistoryInMonths, cleared, embedder))
	}
	return result
}

// codeFileToItem prepares one tracked file for indexing, or returns nil if it
// should be skipped — unchanged, unsupported language, or nothing parseable.
func codeFileToItem(conn *sql.DB, cfg *config.Config, repoPath, relPath string, collectionID int64, force bool) *indexItem {
	absPath := filepath.Join(repoPath, relPath)
	if !fileExists(absPath) {
		return nil
	}

	fh, err := fileHash(absPath)
	if err != nil {
		slog.Warn("cannot hash file, skipping", "path", relPath, "err", err)
		return nil
	}

	if !force && isSourceUnchanged(conn, collectionID, absPath, fh) {
		return nil
	}

	language := parser.GetCodeLanguage(relPath)
	if language == "" {
		return nil
	}

	doc := parser.ParseCodeFile(absPath, language, relPath, cfg.ChunkSizeTokens, cfg.ChunkOverlapTokens)
	if doc == nil || len(doc.Blocks) == 0 {
		return nil
	}

	chunks := codeBlocksToChunks(doc, relPath, cfg)
	if len(chunks) == 0 {
		return nil
	}

	mtime := ""
	if info, err := os.Stat(absPath); err == nil {
		mtime = info.ModTime().UTC().Format(time.RFC3339)
	}

	return &indexItem{
		SourcePath: absPath,
		SourceType: "code",
		Chunks:     chunks,
		FileHash:   fh,
		Mtime:      mtime,
	}
}

// commitToItem prepares one commit for indexing, or returns nil if it should
// be skipped — already indexed, or an empty/unparseable diff.
func commitToItem(conn *sql.DB, cfg *config.Config, repoPath, repoKey string, collectionID int64, commit CommitInfo, force bool) *indexItem {
	sourcePath := fmt.Sprintf("git://%s#%s", repoKey, commit.SHA)

	if !force && isSourceExists(conn, collectionID, sourcePath) {
		return nil
	}

	fileChanges := getCommitFileChanges(repoPath, commit.SHA)
	if len(fileChanges) == 0 {
		return nil
	}

	chunks := commitToChunks(commit, fileChanges, repoPath, cfg)
	if len(chunks) == 0 {
		return nil
	}

	return &indexItem{
		SourcePath: sourcePath,
		SourceType: "commit",
		Chunks:     chunks,
		FileHash:   commit.SHA,
		Mtime:      commit.AuthorDate,
	}
}

func codeBlocksToChunks(doc *parser.CodeDocument, relPath string, cfg *config.Config) []chunker.Chunk {
	// Each size-bounded, AST-aligned block maps to one chunk. A compact context
	// header carries the file span and enclosing symbol path so retrieval sees
	// where a snippet lives (e.g. a method inside a class).
	var chunks []chunker.Chunk

	for i, block := range doc.Blocks {
		symbolDisplay := block.SymbolName
		anon := block.SymbolName == "" || strings.HasPrefix(block.SymbolName, "(")
		switch {
		case block.SymbolPath != "" && anon:
			symbolDisplay = block.SymbolPath
		case block.SymbolPath != "":
			symbolDisplay = block.SymbolPath + " > " + block.SymbolName
		}

		prefix := fmt.Sprintf("[%s:%d-%d] [%s] [%s: %s]\n",
			relPath, block.StartLine, block.EndLine,
			block.Language, block.SymbolType, symbolDisplay)

		metadata := map[string]any{
			"language":    block.Language,
			"symbol_name": block.SymbolName,
			"symbol_type": block.SymbolType,
			"start_line":  block.StartLine,
			"end_line":    block.EndLine,
			"file_path":   block.FilePath,
		}
		if block.SymbolPath != "" {
			metadata["symbol_path"] = block.SymbolPath
		}

		chunks = append(chunks, chunker.Chunk{
			Text:       prefix + block.Text,
			Title:      relPath,
			Metadata:   metadata,
			ChunkIndex: i,
		})
	}
	return chunks
}

func indexGitHistory(ctx context.Context, conn *sql.DB, cfg *config.Config, repoPath string, collectionID int64, force bool, months int, cleared bool, embedder Embedder) *IndexResult {
	historyKey := repoPath + ":history"

	var desc sql.NullString
	_ = conn.QueryRow("SELECT description FROM collections WHERE id = ?", collectionID).Scan(&desc)
	watermarks := parseWatermarks(desc.String)

	var sinceSHA string
	if !force {
		sinceSHA = watermarks[historyKey]
	}

	commits := getCommitsSince(repoPath, sinceSHA, months)
	if len(cfg.GitCommitSubjectBlacklist) > 0 && len(commits) > 0 {
		var filtered []CommitInfo
		for _, c := range commits {
			blacklisted := false
			for _, prefix := range cfg.GitCommitSubjectBlacklist {
				if strings.HasPrefix(c.Subject, prefix) {
					blacklisted = true
					break
				}
			}
			if !blacklisted {
				filtered = append(filtered, c)
			}
		}
		commits = filtered
	}

	if len(commits) == 0 {
		return &IndexResult{}
	}

	result := &IndexResult{TotalFound: len(commits)}
	newestSHA := commits[len(commits)-1].SHA

	indexItemsBatched(ctx, conn, cfg, collectionID, "commits", len(commits),
		func(i int) *indexItem {
			return commitToItem(conn, cfg, repoPath, repoPath, collectionID, commits[i], force)
		},
		embedder, result, nil, cleared)

	if result.Errors == 0 {
		watermarks[historyKey] = newestSHA
		_, _ = conn.Exec("UPDATE collections SET description = ? WHERE id = ?",
			makeWatermarks(watermarks), collectionID)
	}
	return result
}

func commitToChunks(commit CommitInfo, fileChanges []FileChange, repoPath string, cfg *config.Config) []chunker.Chunk {
	chunkSize := cfg.ChunkSizeTokens
	overlap := cfg.ChunkOverlapTokens
	var chunks []chunker.Chunk
	chunkIdx := 0
	repoName := filepath.Base(repoPath)
	shortSHA := commit.SHA
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}
	dateStr := commit.AuthorDate
	if len(dateStr) > 10 {
		dateStr = dateStr[:10]
	}

	for _, fc := range fileChanges {
		if fc.IsBinary {
			continue
		}
		diffText := getFileDiff(repoPath, commit.SHA, fc.FilePath)
		if diffText == "" {
			continue
		}

		prefix := fmt.Sprintf("[%s/%s] [commit: %s] [%s]\n", repoName, fc.FilePath, shortSHA, dateStr)
		body := commit.Subject + "\n\n" + diffText

		metadata := map[string]any{
			"commit_sha":       commit.SHA,
			"commit_sha_short": shortSHA,
			"author_name":      commit.AuthorName,
			"author_email":     commit.AuthorEmail,
			"author_date":      commit.AuthorDate,
			"commit_message":   commit.Subject,
			"file_path":        fc.FilePath,
			"additions":        fc.Additions,
			"deletions":        fc.Deletions,
		}

		prefixedText := prefix + body
		prefixWC := chunker.WordCount(prefix)

		if chunker.WordCount(prefixedText) <= chunkSize {
			chunks = append(chunks, chunker.Chunk{
				Text:       prefixedText,
				Title:      repoName + "/" + fc.FilePath,
				Metadata:   metadata,
				ChunkIndex: chunkIdx,
			})
			chunkIdx++
		} else {
			available := chunkSize - prefixWC
			if available < 50 {
				available = 50
			}
			for _, w := range chunker.SplitIntoWindows(body, available, overlap) {
				meta := copyMeta(metadata)
				chunks = append(chunks, chunker.Chunk{
					Text:       prefix + w,
					Title:      repoName + "/" + fc.FilePath,
					Metadata:   meta,
					ChunkIndex: chunkIdx,
				})
				chunkIdx++
			}
		}
	}
	return chunks
}

// ---- git subprocess helpers ----

// truncateSHA shortens a git SHA for log lines, safely for any input length
// (a persisted watermark is user-visible JSON and may not be a full 40-char
// SHA, so slicing it unconditionally could panic).
func truncateSHA(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func runGit(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
	hideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func isGitRepo(repoPath string) bool {
	_, err := runGit(repoPath, "rev-parse", "--git-dir")
	return err == nil
}

func getHeadSHA(repoPath string) string {
	out, err := runGit(repoPath, "rev-parse", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func gitLsFiles(repoPath string) []string {
	out, err := runGit(repoPath, "ls-files")
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

func gitDiffNames(repoPath, fromSHA, toSHA string) []string {
	out, err := runGit(repoPath, "diff", "--name-only", fromSHA+".."+toSHA)
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

func commitExists(repoPath, sha string) bool {
	_, err := runGit(repoPath, "cat-file", "-t", sha)
	return err == nil
}

func getCommitsSince(repoPath, sinceSHA string, months int) []CommitInfo {
	args := []string{
		"log", "--no-merges",
		fmt.Sprintf("--since=%d months ago", months),
		"--pretty=format:%H|%an|%ae|%aI|%s",
	}
	if sinceSHA != "" {
		args = append(args, sinceSHA+"..HEAD")
	}

	out, err := runGit(repoPath, args...)
	if err != nil {
		slog.Warn("failed to get commit log", "err", err)
		return nil
	}

	var commits []CommitInfo
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 5)
		if len(parts) != 5 {
			continue
		}
		commits = append(commits, CommitInfo{
			SHA:         parts[0],
			AuthorName:  parts[1],
			AuthorEmail: parts[2],
			AuthorDate:  parts[3],
			Subject:     parts[4],
		})
	}

	// Reverse so oldest is first.
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}
	return commits
}

func getCommitFileChanges(repoPath, commitSHA string) []FileChange {
	out, err := runGit(repoPath, "show", "--numstat", "--format=", commitSHA)
	if err != nil {
		return nil
	}

	var changes []FileChange
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		isBinary := parts[0] == "-" && parts[1] == "-"
		adds, dels := 0, 0
		if !isBinary {
			adds, _ = strconv.Atoi(parts[0])
			dels, _ = strconv.Atoi(parts[1])
		}
		changes = append(changes, FileChange{
			FilePath:  parts[2],
			Additions: adds,
			Deletions: dels,
			IsBinary:  isBinary,
		})
	}
	return changes
}

func getFileDiff(repoPath, commitSHA, filePath string) string {
	out, err := runGit(repoPath, "show", commitSHA, "--", filePath)
	if err != nil {
		return ""
	}
	return out
}

func shouldIndexFile(relPath string) bool {
	if shouldExclude(relPath) {
		return false
	}
	return parser.IsCodeFile(relPath)
}

func shouldExclude(relPath string) bool {
	if excludePatterns[filepath.Base(relPath)] {
		return true
	}
	for _, part := range strings.Split(relPath, "/") {
		if excludeDirPatterns[part] {
			return true
		}
	}
	return false
}

func parseWatermarks(description string) map[string]string {
	if description == "" {
		return map[string]string{}
	}
	if strings.HasPrefix(description, "{") {
		var data map[string]string
		if err := json.Unmarshal([]byte(description), &data); err == nil {
			return data
		}
	}
	if strings.HasPrefix(description, watermarkPrefix) {
		rest := description[len(watermarkPrefix):]
		if idx := strings.LastIndex(rest, ":"); idx >= 0 {
			return map[string]string{rest[:idx]: rest[idx+1:]}
		}
	}
	return map[string]string{}
}

func makeWatermarks(watermarks map[string]string) string {
	b, _ := json.Marshal(watermarks)
	return string(b)
}

func isSourceExists(conn *sql.DB, collectionID int64, sourcePath string) bool {
	var id int64
	err := conn.QueryRow(
		"SELECT id FROM sources WHERE collection_id = ? AND source_path = ?",
		collectionID, sourcePath,
	).Scan(&id)
	return err == nil
}

// DiscoverGitRepos returns all git repository root paths at or under root.
// If root itself is a git repo it returns just [root]; otherwise it walks
// subdirectories, stopping descent when a .git dir is found (so submodules
// are not returned as separate repos).
func DiscoverGitRepos(root string) ([]string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("abs path: %w", err)
	}

	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", root)
	}

	if isGitRepo(root) {
		return []string{root}, nil
	}

	var repos []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			slog.Warn("discover repos: error walking", "path", path, "err", err)
			return filepath.SkipDir
		}
		if !d.IsDir() || path == root {
			return nil
		}
		if fi, statErr := os.Stat(filepath.Join(path, ".git")); statErr == nil && fi.IsDir() {
			repos = append(repos, path)
			return filepath.SkipDir
		}
		return nil
	})
	return repos, nil
}

// ResolveRepoPaths takes configured paths (git repos or parent dirs containing
// repos) and returns a deduplicated list of discovered git repository paths.
func ResolveRepoPaths(configPaths []string) []string {
	seen := make(map[string]bool)
	var resolved []string

	for _, p := range configPaths {
		abs, err := filepath.Abs(p)
		if err != nil {
			slog.Warn("resolve repo path: cannot get abs path", "path", p, "err", err)
			continue
		}
		discovered, err := DiscoverGitRepos(abs)
		if err != nil {
			slog.Warn("discover repos failed", "path", abs, "err", err)
			continue
		}
		for _, d := range discovered {
			if !seen[d] {
				seen[d] = true
				resolved = append(resolved, d)
			}
		}
	}
	return resolved
}
