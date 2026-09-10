package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"vectile/backend/config"
	"vectile/backend/db"
	"vectile/backend/search"
	"vectile/backend/services"
)

// --- vectile_search ---

func handleSearch(core *services.Core) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, err := request.RequireString("query")
		if err != nil {
			return mcp.NewToolResultError("query parameter is required"), nil
		}

		topK := request.GetInt("top_k", core.Cfg.SearchDefaults.TopK)

		metadataFilters := make(map[string]string)
		if mf, ok := request.GetArguments()["metadata_filter"].(map[string]any); ok {
			for k, v := range mf {
				metadataFilters[k] = fmt.Sprintf("%v", v)
			}
		}

		filters := search.Filters{
			Collection:      request.GetString("collection", ""),
			SourceType:      request.GetString("source_type", ""),
			Path:            request.GetString("path", ""),
			DateFrom:        request.GetString("date_from", ""),
			DateTo:          request.GetString("date_to", ""),
			Sender:          request.GetString("sender", ""),
			Author:          request.GetString("author", ""),
			MetadataFilters: metadataFilters,
			TopK:            topK,
		}

		resp, err := search.Search(db.DB, query, filters, core.Embedder, core.Cfg.SearchDefaults)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
		}
		results := resp.Results

		output := make([]map[string]any, 0, len(results))
		for _, r := range results {
			output = append(output, map[string]any{
				"title":       r.Title,
				"content":     r.Content,
				"collection":  r.Collection,
				"source_type": r.SourceType,
				"source_path": r.SourcePath,
				"source_uri":  buildSourceURI(r.SourcePath, r.SourceType, r.Metadata, core.Cfg),
				"score":       fmt.Sprintf("%.4f", r.Score),
				"metadata":    r.Metadata,
			})
		}

		data, _ := json.MarshalIndent(output, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

// --- vectile_list_collections ---

func handleListCollections(core *services.Core) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rows, err := db.DB.Query(`
			SELECT c.name, c.collection_type, c.description, c.created_at,
				(SELECT COUNT(*) FROM sources s WHERE s.collection_id = c.id),
				(SELECT COUNT(*) FROM documents d WHERE d.collection_id = c.id),
				(SELECT MAX(s2.last_indexed_at) FROM sources s2
				  WHERE s2.collection_id = c.id AND s2.last_indexed_at IS NOT NULL AND s2.last_indexed_at != '')
			FROM collections c ORDER BY c.name`)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("query collections: %v", err)), nil
		}
		defer rows.Close()

		var collections []map[string]any
		for rows.Next() {
			var name, collType, createdAt string
			var description, lastIndexed sql.NullString
			var sourceCount, chunkCount int
			if err := rows.Scan(&name, &collType, &description, &createdAt,
				&sourceCount, &chunkCount, &lastIndexed); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("scan collection: %v", err)), nil
			}

			entry := map[string]any{
				"name":         name,
				"type":         collType,
				"source_count": sourceCount,
				"chunk_count":  chunkCount,
				"created_at":   createdAt,
				"enabled":      core.Cfg.IsCollectionEnabled(name),
			}
			if description.Valid {
				if text, repos := describeCollection(description.String); text != "" {
					entry["description"] = text
				} else if repos > 0 {
					entry["repositories"] = repos
				}
			}
			if lastIndexed.Valid {
				entry["last_indexed"] = lastIndexed.String
			}
			collections = append(collections, entry)
		}
		if err := rows.Err(); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("iterate collections: %v", err)), nil
		}

		data, _ := json.MarshalIndent(collections, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

// --- vectile_collection_info ---

func handleCollectionInfo(core *services.Core) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		collection, err := request.RequireString("collection")
		if err != nil {
			return mcp.NewToolResultError("collection parameter is required"), nil
		}

		var id int64
		var collType, createdAt string
		var description sql.NullString
		err = db.DB.QueryRow("SELECT id, collection_type, created_at, description FROM collections WHERE name = ?", collection).
			Scan(&id, &collType, &createdAt, &description)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("collection %q not found", collection)), nil
		}

		var sourceCount, chunkCount int
		_ = db.DB.QueryRow("SELECT COUNT(*) FROM sources WHERE collection_id = ?", id).Scan(&sourceCount)
		_ = db.DB.QueryRow("SELECT COUNT(*) FROM documents WHERE collection_id = ?", id).Scan(&chunkCount)

		var lastIndexed sql.NullString
		_ = db.DB.QueryRow("SELECT MAX(last_indexed_at) FROM sources WHERE collection_id = ?", id).Scan(&lastIndexed)

		// Source type breakdown.
		sourceTypes := map[string]int{}
		if typeRows, err := db.DB.Query(
			"SELECT source_type, COUNT(*) FROM sources WHERE collection_id = ? GROUP BY source_type", id); err == nil {
			for typeRows.Next() {
				var st string
				var cnt int
				if typeRows.Scan(&st, &cnt) == nil {
					sourceTypes[st] = cnt
				}
			}
			typeRows.Close()
		}

		// Sample titles.
		var sampleTitles []string
		if titleRows, err := db.DB.Query(
			"SELECT DISTINCT title FROM documents WHERE collection_id = ? AND title IS NOT NULL LIMIT 10", id); err == nil {
			for titleRows.Next() {
				var t string
				if titleRows.Scan(&t) == nil && t != "" {
					sampleTitles = append(sampleTitles, t)
				}
			}
			titleRows.Close()
		}

		output := map[string]any{
			"name":          collection,
			"type":          collType,
			"created_at":    createdAt,
			"source_count":  sourceCount,
			"chunk_count":   chunkCount,
			"source_types":  sourceTypes,
			"sample_titles": sampleTitles,
			"enabled":       core.Cfg.IsCollectionEnabled(collection),
		}
		if description.Valid {
			if text, repos := describeCollection(description.String); text != "" {
				output["description"] = text
			} else if repos > 0 {
				output["repositories"] = repos
			}
		}
		if lastIndexed.Valid {
			output["last_indexed"] = lastIndexed.String
		}

		data, _ := json.MarshalIndent(output, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

// --- vectile_index ---

func handleIndex(core *services.Core) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if !core.Cfg.MCP.AllowWrite {
			return mcp.NewToolResultError(
				"write tools are disabled: enable 'Allow write tools' in vectile Settings"), nil
		}
		collection, err := request.RequireString("collection")
		if err != nil {
			return mcp.NewToolResultError("collection parameter is required"), nil
		}
		force := request.GetBool("force", false)

		result, err := services.NewIndexService(core).IndexSynchronous(collection, force)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("index failed: %v", err)), nil
		}

		output := map[string]any{
			"collection":  collection,
			"indexed":     result.Indexed,
			"skipped":     result.Skipped,
			"errors":      result.Errors,
			"total_found": result.TotalFound,
		}
		if len(result.ErrorMessages) > 0 {
			output["error_messages"] = result.ErrorMessages
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		if result.Errors > 0 {
			return mcp.NewToolResultError(string(data)), nil
		}
		return mcp.NewToolResultText(string(data)), nil
	}
}

// --- vectile_prune ---

func handlePrune(core *services.Core) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if !core.Cfg.MCP.AllowWrite {
			return mcp.NewToolResultError(
				"write tools are disabled: enable 'Allow write tools' in vectile Settings"), nil
		}
		collection := request.GetString("collection", "")

		result, err := services.NewIndexService(core).Prune(collection)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("prune failed: %v", err)), nil
		}

		name := collection
		if name == "" {
			name = "all"
		}
		output := map[string]any{
			"collection": name,
			"pruned":     result.Pruned,
			"checked":    result.Checked,
			"errors":     result.Errors,
		}
		if len(result.ErrorMessages) > 0 {
			output["error_messages"] = result.ErrorMessages
		}
		data, _ := json.MarshalIndent(output, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

// describeCollection turns a collection's stored description into something
// fit for a client. The description column is overloaded: system and project
// collections keep a human string there, but code collections store git
// watermarks, a JSON map of repository path to indexed commit SHA, which can
// run to tens of thousands of characters. Watermarks are internal bookkeeping
// and are summarised into a repository count instead.
func describeCollection(description string) (text string, repos int) {
	if description == "" {
		return "", 0
	}
	if strings.HasPrefix(description, "{") {
		var watermarks map[string]string
		if err := json.Unmarshal([]byte(description), &watermarks); err == nil {
			seen := map[string]struct{}{}
			for key := range watermarks {
				seen[strings.TrimSuffix(key, ":history")] = struct{}{}
			}
			return "", len(seen)
		}
	}
	return description, 0
}

// --- Source URI helpers ---

// buildSourceURI maps an indexed source to a URI a client can open, using the
// same conventions as the app: Obsidian vault paths become obsidian:// links,
// code files become vscode://file links at their start line, calibre and git
// virtual paths resolve to nil, and everything else is a plain file:// URL. A
// doc that carries a URL in its metadata (e.g. an external source) wins.
func buildSourceURI(sourcePath, sourceType string, metadata map[string]any, cfg *config.Config) any {
	if u, ok := metadata["url"].(string); ok && u != "" {
		return u
	}

	if sourceType == "commit" || strings.HasPrefix(sourcePath, "git://") {
		return nil
	}
	if strings.HasPrefix(sourcePath, "calibre://") {
		return nil
	}

	for _, vault := range cfg.ObsidianVaults {
		if strings.HasPrefix(sourcePath, vault) {
			if uri := buildObsidianURI(sourcePath, vault); uri != "" {
				return uri
			}
		}
	}

	if sourceType == "code" {
		startLine := 1
		if sl, ok := metadata["start_line"].(float64); ok {
			startLine = int(sl)
		}
		return fmt.Sprintf("vscode://file%s:%d", sourcePath, startLine)
	}

	return "file://" + sourcePath
}

func buildObsidianURI(sourcePath, vaultPath string) string {
	vaultName := filepath.Base(vaultPath)
	relPath, err := filepath.Rel(vaultPath, sourcePath)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("obsidian://open?vault=%s&file=%s",
		url.QueryEscape(vaultName),
		url.QueryEscape(relPath))
}
