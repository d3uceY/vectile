package indexer

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"vectile/backend/config"
	"vectile/backend/parser"
)

// Directories to skip when walking an Obsidian vault.
var obsidianSkipDirs = map[string]bool{
	".obsidian": true,
	".trash":    true,
	".git":      true,
}

// IndexObsidian indexes all supported files in Obsidian vaults.
func IndexObsidian(ctx context.Context, conn *sql.DB, cfg *config.Config, force bool, progress ProgressCallback, embedder Embedder) *IndexResult {
	collectionID, err := getOrCreate(conn, "obsidian", "system")
	if err != nil {
		return failedResult(err)
	}

	excludeFolders := make(map[string]bool)
	for _, f := range cfg.ObsidianExcludeFolders {
		excludeFolders[f] = true
	}

	var allFiles []string
	for _, vault := range cfg.ObsidianVaults {
		vault = expandPath(vault)
		info, err := os.Stat(vault)
		if err != nil || !info.IsDir() {
			slog.Warn("vault path does not exist or is not a directory", "path", vault)
			continue
		}
		slog.Info("indexing Obsidian vault", "path", vault)
		allFiles = append(allFiles, walkVault(vault, excludeFolders, cfg.SkipCloudPlaceholders)...)
	}

	result := &IndexResult{TotalFound: len(allFiles)}
	cleared := clearForRebuild(conn, collectionID, force)

	indexItemsBatched(ctx, conn, cfg, collectionID, "obsidian", len(allFiles),
		func(i int) *indexItem { return fileToItem(ctx, conn, cfg, allFiles[i], collectionID, force, result) },
		embedder, result, progress, cleared)
	return result
}

func walkVault(vaultPath string, excludeFolders map[string]bool, skipPlaceholders bool) []string {
	var results []string
	filepath.Walk(vaultPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if obsidianSkipDirs[name] || excludeFolders[name] || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}
		if parser.SourceTypeForPath(path) == "" {
			return nil
		}
		if skipPlaceholders && isCloudPlaceholder(info) {
			return nil
		}
		results = append(results, path)
		return nil
	})
	return results
}
