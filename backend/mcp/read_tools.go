package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"vectile/backend/db"
	"vectile/backend/embeddings"
	"vectile/backend/search"
	"vectile/backend/services"
)

// This file holds the reading and orientation tools: the ones an assistant
// needs after a search has handed it a result. Everything here is read-only.

// --- vectile_get_chunk ---

var getChunkTool = mcp.NewTool("vectile_get_chunk",
	mcp.WithDescription(
		"Read one indexed chunk in full, optionally with neighbouring chunks "+
			"from the same source. Pass the 'id' a search or grep result gave "+
			"you. Read-only."),
	mcp.WithNumber("id",
		mcp.Required(),
		mcp.Description("Chunk id from vectile_search or vectile_grep.")),
	mcp.WithNumber("before",
		mcp.Description("Neighbouring chunks to include before it (default 0, max 20).")),
	mcp.WithNumber("after",
		mcp.Description("Neighbouring chunks to include after it (default 0, max 20).")),
)

func handleGetChunk(core *services.Core) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := int64(request.GetInt("id", 0))
		if id <= 0 {
			return mcp.NewToolResultError("id is required"), nil
		}
		before := clampInt(request.GetInt("before", 0), 0, 0, 20)
		after := clampInt(request.GetInt("after", 0), 0, 0, 20)

		var sourceID int64
		var chunkIndex int
		if err := db.DB.QueryRow(
			"SELECT source_id, chunk_index FROM documents WHERE id = ?", id).
			Scan(&sourceID, &chunkIndex); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("chunk %d not found", id)), nil
		}

		rows, err := db.DB.Query(`
			SELECT d.id, d.chunk_index, d.title, d.content, d.metadata,
			       s.source_path, s.source_type
			FROM documents d
			JOIN sources s ON d.source_id = s.id
			WHERE d.source_id = ? AND d.chunk_index BETWEEN ? AND ?
			ORDER BY d.chunk_index`,
			sourceID, chunkIndex-before, chunkIndex+after)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read chunks: %v", err)), nil
		}
		defer rows.Close()

		var chunks []map[string]any
		var sourcePath, sourceType string
		for rows.Next() {
			var (
				cid         int64
				idx         int
				content     string
				title, meta sql.NullString
			)
			if err := rows.Scan(&cid, &idx, &title, &content, &meta, &sourcePath, &sourceType); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("scan chunk: %v", err)), nil
			}
			md := decodeMetadata(meta)
			chunks = append(chunks, map[string]any{
				"id":          cid,
				"chunk_index": idx,
				"title":       title.String,
				"content":     content,
				"source_uri":  buildSourceURI(sourcePath, sourceType, md, core.Cfg),
				"metadata":    md,
			})
		}
		if err := rows.Err(); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read chunks: %v", err)), nil
		}

		out := map[string]any{
			"seed_id":     id,
			"source_path": sourcePath,
			"source_type": sourceType,
			"chunks":      chunks,
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

// --- vectile_read_source ---

var readSourceTool = mcp.NewTool("vectile_read_source",
	mcp.WithDescription(
		"Read a whole indexed source (a note, a book section, a code file) "+
			"reassembled in order. Address it with the source_path a search "+
			"result returned. Read-only."),
	mcp.WithString("source_path",
		mcp.Required(),
		mcp.Description("Absolute path of the source, as returned by vectile_search.")),
	mcp.WithString("collection",
		mcp.Description("Set this when the same path exists in more than one collection.")),
	mcp.WithString("heading",
		mcp.Description("Return only chunks whose markdown heading_path contains this text.")),
	mcp.WithNumber("max_chars",
		mcp.Description("Cap on characters returned (default 60000).")),
)

func handleReadSource(core *services.Core) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := request.RequireString("source_path")
		if err != nil {
			return mcp.NewToolResultError("source_path is required"), nil
		}
		collection := request.GetString("collection", "")
		heading := strings.ToLower(request.GetString("heading", ""))
		maxChars := clampInt(request.GetInt("max_chars", 0), 60000, 1000, 500000)

		query := `SELECT s.id, s.source_type, c.name
			FROM sources s
			JOIN collections c ON s.collection_id = c.id
			WHERE s.source_path = ?`
		args := []any{path}
		if collection != "" {
			query += " AND c.name = ?"
			args = append(args, collection)
		}

		rows, err := db.DB.Query(query, args...)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("look up source: %v", err)), nil
		}
		type candidate struct {
			ID         int64  `json:"source_id"`
			Type       string `json:"source_type"`
			Collection string `json:"collection"`
		}
		var found []candidate
		for rows.Next() {
			var c candidate
			if err := rows.Scan(&c.ID, &c.Type, &c.Collection); err != nil {
				rows.Close()
				return mcp.NewToolResultError(fmt.Sprintf("scan source: %v", err)), nil
			}
			found = append(found, c)
		}
		rows.Close()

		if len(found) == 0 {
			return mcp.NewToolResultError(fmt.Sprintf("no indexed source at %q", path)), nil
		}
		if len(found) > 1 {
			data, _ := json.MarshalIndent(map[string]any{
				"source_path": path,
				"candidates":  found,
				"hint":        "call again with collection set to one of these",
			}, "", "  ")
			return mcp.NewToolResultText(string(data)), nil
		}

		src := found[0]
		chunkRows, err := db.DB.Query(
			`SELECT id, chunk_index, title, content, metadata
			 FROM documents WHERE source_id = ? ORDER BY chunk_index`, src.ID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read source chunks: %v", err)), nil
		}
		defer chunkRows.Close()

		var b strings.Builder
		returned := 0
		truncated := false
		for chunkRows.Next() {
			var (
				cid         int64
				idx         int
				content     string
				title, meta sql.NullString
			)
			if err := chunkRows.Scan(&cid, &idx, &title, &content, &meta); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("scan source chunk: %v", err)), nil
			}
			if heading != "" {
				hp, _ := decodeMetadata(meta)["heading_path"].(string)
				if !strings.Contains(strings.ToLower(hp), heading) {
					continue
				}
			}
			if b.Len() > 0 {
				if b.Len()+2 > maxChars {
					truncated = true
					break
				}
				b.WriteString("\n\n")
			}
			if b.Len()+len(content) > maxChars {
				content, _ = snippet(content, maxChars-b.Len())
				truncated = true
			}
			b.WriteString(content)
			returned++
			if truncated {
				break
			}
		}
		if err := chunkRows.Err(); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("read source chunks: %v", err)), nil
		}

		out := map[string]any{
			"source_path":     path,
			"source_type":     src.Type,
			"collection":      src.Collection,
			"chunks_returned": returned,
			"truncated":       truncated,
			"content":         b.String(),
		}
		if heading != "" {
			out["heading_filter"] = heading
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

// --- vectile_grep ---

// grepScanCap bounds the rows a regex scan will read before giving up. Literal
// matching is pushed into SQLite, so it does not need the cap.
const grepScanCap = 20000

var grepTool = mcp.NewTool("vectile_grep",
	mcp.WithDescription(
		"Find an exact string in chunk text, or a regular expression when regex "+
			"is true. Use it for identifiers, error strings, TODOs, and config "+
			"keys, where an exact match beats a meaning match. Read-only."),
	mcp.WithString("pattern",
		mcp.Required(),
		mcp.Description("Text to find. Case-insensitive unless regex is true.")),
	mcp.WithBoolean("regex",
		mcp.Description("Treat pattern as a regular expression (default false).")),
	mcp.WithString("collection",
		mcp.Description("Limit to one collection name.")),
	mcp.WithString("source_type",
		mcp.Description("Limit to one source type, e.g. 'code' or 'markdown'.")),
	mcp.WithString("path",
		mcp.Description("Only sources whose path contains this text.")),
	mcp.WithNumber("limit",
		mcp.Description("Maximum matches (default 20, max 200).")),
)

func handleGrep(core *services.Core) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		pattern, err := request.RequireString("pattern")
		if err != nil || pattern == "" {
			return mcp.NewToolResultError("pattern is required"), nil
		}
		isRegex := request.GetBool("regex", false)
		limit := clampInt(request.GetInt("limit", 0), 20, 1, 200)

		var re *regexp.Regexp
		if isRegex {
			re, err = regexp.Compile(pattern)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("bad regex: %v", err)), nil
			}
		}

		where := []string{}
		args := []any{}
		if c := request.GetString("collection", ""); c != "" {
			where = append(where, "c.name = ?")
			args = append(args, c)
		}
		if st := request.GetString("source_type", ""); st != "" {
			where = append(where, "s.source_type = ?")
			args = append(args, st)
		}
		if p := request.GetString("path", ""); p != "" {
			where = append(where, "instr(lower(s.source_path), lower(?)) > 0")
			args = append(args, p)
		}
		if !isRegex {
			where = append(where, "instr(lower(d.content), lower(?)) > 0")
			args = append(args, pattern)
		}

		query := `SELECT d.id, d.chunk_index, d.title, d.content, d.metadata,
			       s.source_path, s.source_type, c.name
			FROM documents d
			JOIN sources s ON d.source_id = s.id
			JOIN collections c ON d.collection_id = c.id`
		if len(where) > 0 {
			query += " WHERE " + strings.Join(where, " AND ")
		}
		if isRegex {
			query += fmt.Sprintf(" ORDER BY d.id LIMIT %d", grepScanCap)
		} else {
			query += " ORDER BY d.id LIMIT ?"
			args = append(args, limit)
		}

		rows, err := db.DB.Query(query, args...)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("grep failed: %v", err)), nil
		}
		defer rows.Close()

		matches := make([]map[string]any, 0, limit)
		scanned := 0
		for rows.Next() {
			var (
				id                           int64
				chunkIndex                   int
				content                      string
				title, meta                  sql.NullString
				sourcePath, sourceType, coll string
			)
			if err := rows.Scan(&id, &chunkIndex, &title, &content, &meta,
				&sourcePath, &sourceType, &coll); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("scan match: %v", err)), nil
			}

			start, end := -1, -1
			if isRegex {
				scanned++
				if loc := re.FindStringIndex(content); loc != nil {
					start, end = loc[0], loc[1]
				}
			} else {
				if i := strings.Index(strings.ToLower(content), strings.ToLower(pattern)); i >= 0 {
					start, end = i, i+len(pattern)
				}
			}
			if start < 0 {
				continue
			}

			md := decodeMetadata(meta)
			match := map[string]any{
				"id":          id,
				"chunk_index": chunkIndex,
				"title":       title.String,
				"collection":  coll,
				"source_type": sourceType,
				"source_path": sourcePath,
				"source_uri":  buildSourceURI(sourcePath, sourceType, md, core.Cfg),
				"line":        strings.Count(content[:start], "\n") + 1,
				"context":     matchContext(content, start, end, 120),
			}
			for _, key := range []string{"start_line", "end_line"} {
				if v, ok := md[key]; ok {
					match[key] = v
				}
			}
			matches = append(matches, match)
			if len(matches) >= limit {
				break
			}
		}
		if err := rows.Err(); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("grep failed: %v", err)), nil
		}

		out := map[string]any{
			"pattern":       pattern,
			"regex":         isRegex,
			"matches":       matches,
			"limit_reached": len(matches) >= limit,
		}
		if isRegex {
			out["scanned"] = scanned
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

// --- vectile_status ---

var statusTool = mcp.NewTool("vectile_status",
	mcp.WithDescription(
		"Report library health: collection, source, and chunk counts, database "+
			"size, when something was last indexed, and the active model. Check "+
			"this before trusting a search, because a stale index may be missing "+
			"what you want. Read-only."),
)

func handleStatus(core *services.Core) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		st := services.NewAppService(core).GetStatus()
		out := map[string]any{
			"collections":   st.Collections,
			"sources":       st.Sources,
			"chunks":        st.Chunks,
			"db_size_bytes": st.DBSize,
			"last_indexed":  st.LastIndexed,
			"model":         st.ModelName,
			"model_state":   st.ModelState,
		}
		if st.ModelError != "" {
			out["model_error"] = st.ModelError
		}
		if core.Cfg != nil && len(core.Cfg.DisabledCollections) > 0 {
			out["disabled_collections"] = core.Cfg.DisabledCollections
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

// --- vectile_list_sources ---

var listSourcesTool = mcp.NewTool("vectile_list_sources",
	mcp.WithDescription(
		"List the sources indexed in one collection: paths with chunk counts "+
			"and last-indexed time. Use it to see what a collection holds before "+
			"reading a whole source. Read-only."),
	mcp.WithString("collection",
		mcp.Required(),
		mcp.Description("Collection name. Use vectile_list_collections() to discover names.")),
	mcp.WithString("match",
		mcp.Description("Only paths containing this text.")),
	mcp.WithNumber("limit",
		mcp.Description("Maximum sources (default 200, max 1000).")),
)

func handleListSources(core *services.Core) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, err := request.RequireString("collection")
		if err != nil {
			return mcp.NewToolResultError("collection parameter is required"), nil
		}
		id, err := collectionID(name)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		match := request.GetString("match", "")
		limit := clampInt(request.GetInt("limit", 0), 200, 1, 1000)

		query := `SELECT s.source_path, s.source_type,
			       (SELECT COUNT(*) FROM documents d WHERE d.source_id = s.id),
			       s.last_indexed_at
			FROM sources s
			WHERE s.collection_id = ?`
		args := []any{id}
		if match != "" {
			query += " AND instr(lower(s.source_path), lower(?)) > 0"
			args = append(args, match)
		}
		query += " ORDER BY s.source_path LIMIT ?"
		args = append(args, limit)

		rows, err := db.DB.Query(query, args...)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("list sources: %v", err)), nil
		}
		defer rows.Close()

		sources := []map[string]any{}
		for rows.Next() {
			var (
				path, sourceType string
				chunks           int
				lastIndexed      sql.NullString
			)
			if err := rows.Scan(&path, &sourceType, &chunks, &lastIndexed); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("scan source: %v", err)), nil
			}
			sources = append(sources, map[string]any{
				"source_path":  path,
				"source_type":  sourceType,
				"chunks":       chunks,
				"last_indexed": lastIndexed.String,
			})
		}
		if err := rows.Err(); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("list sources: %v", err)), nil
		}

		out := map[string]any{
			"collection": name,
			"sources":    sources,
			"count":      len(sources),
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

// --- vectile_facets ---

// facetKeys are the metadata keys worth showing top values for. The first two
// hold arrays, the rest hold a single string; json_each expands both shapes.
var facetKeys = []string{"tags", "authors", "language", "symbol_type", "sender"}

var facetsTool = mcp.NewTool("vectile_facets",
	mcp.WithDescription(
		"List the metadata keys and common values in the library (tags, "+
			"authors, languages, symbol types, senders) with the date range. Use "+
			"it to build metadata_filter values for vectile_search instead of "+
			"guessing them. Read-only."),
	mcp.WithString("collection",
		mcp.Description("Limit to one collection. Omit for the whole library.")),
)

func handleFacets(core *services.Core) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := request.GetString("collection", "")
		var collID *int64
		if name != "" {
			id, err := collectionID(name)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			collID = &id
		}

		out := map[string]any{}
		if name != "" {
			out["collection"] = name
		}

		// Source type counts, straight from the sources table.
		typeQuery := "SELECT source_type, COUNT(*) FROM sources"
		typeArgs := []any{}
		if collID != nil {
			typeQuery += " WHERE collection_id = ?"
			typeArgs = append(typeArgs, *collID)
		}
		out["source_types"] = queryPairs(typeQuery+" GROUP BY source_type ORDER BY COUNT(*) DESC", typeArgs...)

		// Metadata keys, most common first.
		keyQuery := `SELECT je.key, COUNT(*) c FROM documents d, json_each(d.metadata) je`
		keyArgs := []any{}
		if collID != nil {
			keyQuery += " WHERE d.collection_id = ?"
			keyArgs = append(keyArgs, *collID)
		}
		out["keys"] = queryPairs(keyQuery+" GROUP BY je.key ORDER BY c DESC LIMIT 40", keyArgs...)

		// Top values for the filterable keys.
		values := map[string]any{}
		for _, key := range facetKeys {
			valueQuery := `SELECT je.value, COUNT(*) c FROM documents d, json_each(d.metadata, ?) je`
			valueArgs := []any{"$." + key}
			if collID != nil {
				valueQuery += " WHERE d.collection_id = ?"
				valueArgs = append(valueArgs, *collID)
			}
			if pairs := queryPairs(valueQuery+" GROUP BY je.value ORDER BY c DESC LIMIT 8", valueArgs...); len(pairs) > 0 {
				values[key] = pairs
			}
		}
		if len(values) > 0 {
			out["values"] = values
		}

		// Date range across whichever date a document carries.
		dateQuery := `SELECT MIN(substr(` + docDateExpr + `,1,10)),
			       MAX(substr(` + docDateExpr + `,1,10))
			FROM documents d`
		dateArgs := []any{}
		if collID != nil {
			dateQuery += " WHERE d.collection_id = ?"
			dateArgs = append(dateArgs, *collID)
		}
		var from, to sql.NullString
		if err := db.DB.QueryRow(dateQuery, dateArgs...).Scan(&from, &to); err == nil {
			out["date_range"] = map[string]any{"from": from.String, "to": to.String}
		}

		data, _ := json.MarshalIndent(out, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

// --- vectile_find_related ---

var findRelatedTool = mcp.NewTool("vectile_find_related",
	mcp.WithDescription(
		"Find chunks nearest to a chunk you already have, by embedding "+
			"similarity. Use it to expand one good hit into the rest of the "+
			"discussion. Read-only."),
	mcp.WithNumber("chunk_id",
		mcp.Required(),
		mcp.Description("Seed chunk id, from vectile_search.")),
	mcp.WithString("collection",
		mcp.Description("Limit results to one collection name.")),
	mcp.WithNumber("top_k",
		mcp.Description("Results to return (default the configured top-k).")),
	mcp.WithNumber("max_chars",
		mcp.Description("Snippet length per result (default 1200).")),
)

func handleFindRelated(core *services.Core) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		seed := int64(request.GetInt("chunk_id", 0))
		if seed <= 0 {
			return mcp.NewToolResultError("chunk_id is required"), nil
		}
		maxChars := clampChars(request.GetInt("max_chars", defaultSnippetChars))

		var blob []byte
		if err := db.DB.QueryRow(
			"SELECT embedding FROM vec_documents WHERE document_id = ?", seed).Scan(&blob); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf(
				"chunk %d has no embedding: re-index it, or use vectile_search with its text", seed)), nil
		}

		topK := request.GetInt("top_k", 0)
		filters := search.Filters{
			Collection: request.GetString("collection", ""),
			TopK:       topK,
		}
		results, err := search.Related(db.DB, embeddings.DeserializeFloat32(blob), seed, topK, filters, core.Cfg.SearchDefaults)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("related search failed: %v", err)), nil
		}

		output := make([]map[string]any, 0, len(results))
		for _, r := range results {
			text, truncated := snippet(r.Content, maxChars)
			output = append(output, map[string]any{
				"id":          r.ID,
				"title":       r.Title,
				"snippet":     text,
				"truncated":   truncated,
				"collection":  r.Collection,
				"source_type": r.SourceType,
				"source_path": r.SourcePath,
				"source_uri":  buildSourceURI(r.SourcePath, r.SourceType, r.Metadata, core.Cfg),
				"distance":    fmt.Sprintf("%.4f", r.Score),
			})
		}

		out := map[string]any{
			"seed_id": seed,
			"results": output,
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

// --- vectile_timeline ---

// docDateExpr is the date a document carries, in priority order: a note's
// frontmatter date, a commit's author date, then when the source was indexed.
// Every branch is ISO-like, so a plain string sort is chronological.
const docDateExpr = `COALESCE(json_extract(d.metadata,'$.date'), json_extract(d.metadata,'$.author_date'), s.last_indexed_at)`

var timelineTool = mcp.NewTool("vectile_timeline",
	mcp.WithDescription(
		"List documents in date order, newest first, using the date each source "+
			"carries (a note's date, a commit's author date) and falling back to "+
			"when it was indexed. Use it for questions about what happened around "+
			"a time. Read-only."),
	mcp.WithString("collection",
		mcp.Description("Limit to one collection name.")),
	mcp.WithString("source_type",
		mcp.Description("Limit to one source type, e.g. 'commit' or 'markdown'.")),
	mcp.WithString("date_from",
		mcp.Description("Only documents dated on or after this day (YYYY-MM-DD).")),
	mcp.WithString("date_to",
		mcp.Description("Only documents dated on or before this day (YYYY-MM-DD).")),
	mcp.WithNumber("limit",
		mcp.Description("Maximum documents (default 50, max 500).")),
)

func handleTimeline(core *services.Core) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := clampInt(request.GetInt("limit", 0), 50, 1, 500)
		where := []string{}
		args := []any{}
		if c := request.GetString("collection", ""); c != "" {
			where = append(where, "c.name = ?")
			args = append(args, c)
		}
		if st := request.GetString("source_type", ""); st != "" {
			where = append(where, "s.source_type = ?")
			args = append(args, st)
		}
		if from := request.GetString("date_from", ""); from != "" {
			where = append(where, "substr("+docDateExpr+",1,10) >= ?")
			args = append(args, from)
		}
		if to := request.GetString("date_to", ""); to != "" {
			where = append(where, "substr("+docDateExpr+",1,10) <= ?")
			args = append(args, to)
		}

		query := `SELECT d.id, d.title, s.source_path, s.source_type, c.name, d.metadata,
			       ` + docDateExpr + ` AS doc_date
			FROM documents d
			JOIN sources s ON d.source_id = s.id
			JOIN collections c ON d.collection_id = c.id`
		if len(where) > 0 {
			query += " WHERE " + strings.Join(where, " AND ")
		}
		query += " ORDER BY doc_date DESC LIMIT ?"
		args = append(args, limit)

		rows, err := db.DB.Query(query, args...)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("timeline failed: %v", err)), nil
		}
		defer rows.Close()

		documents := []map[string]any{}
		for rows.Next() {
			var (
				id                           int64
				title, meta, docDate         sql.NullString
				sourcePath, sourceType, coll string
			)
			if err := rows.Scan(&id, &title, &sourcePath, &sourceType, &coll, &meta, &docDate); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("scan timeline row: %v", err)), nil
			}
			md := decodeMetadata(meta)
			documents = append(documents, map[string]any{
				"id":          id,
				"date":        docDate.String,
				"title":       title.String,
				"collection":  coll,
				"source_type": sourceType,
				"source_path": sourcePath,
				"source_uri":  buildSourceURI(sourcePath, sourceType, md, core.Cfg),
			})
		}
		if err := rows.Err(); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("timeline failed: %v", err)), nil
		}

		out := map[string]any{
			"documents": documents,
			"count":     len(documents),
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		return mcp.NewToolResultText(string(data)), nil
	}
}

// --- Shared helpers ---

// collectionID resolves a collection name to its row id.
func collectionID(name string) (int64, error) {
	var id int64
	if err := db.DB.QueryRow("SELECT id FROM collections WHERE name = ?", name).Scan(&id); err != nil {
		return 0, fmt.Errorf("collection %q not found", name)
	}
	return id, nil
}

// decodeMetadata turns a raw metadata column into a map, never nil.
func decodeMetadata(raw sql.NullString) map[string]any {
	md := map[string]any{}
	if raw.Valid && raw.String != "" {
		_ = json.Unmarshal([]byte(raw.String), &md)
	}
	return md
}

// clampInt applies a default when n is zero or negative, then a range.
func clampInt(n, def, lo, hi int) int {
	if n <= 0 {
		n = def
	}
	if n < lo {
		n = lo
	}
	if n > hi {
		n = hi
	}
	return n
}

// queryPairs runs a (label, count) query and returns it as value/count rows.
func queryPairs(query string, args ...any) []map[string]any {
	rows, err := db.DB.Query(query, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var label any
		var count int
		if rows.Scan(&label, &count) == nil {
			out = append(out, map[string]any{"value": fmt.Sprintf("%v", label), "count": count})
		}
	}
	return out
}

// matchContext returns a window of text around a match, trimmed on rune
// boundaries so a slice never splits a character.
func matchContext(content string, start, end, window int) string {
	lo := start - window
	if lo < 0 {
		lo = 0
	}
	hi := end + window
	if hi > len(content) {
		hi = len(content)
	}
	for lo > 0 && !utf8.RuneStart(content[lo]) {
		lo--
	}
	for hi < len(content) && !utf8.RuneStart(content[hi]) {
		hi++
	}
	out := strings.TrimSpace(content[lo:hi])
	if lo > 0 {
		out = "\u2026" + out
	}
	if hi < len(content) {
		out += "\u2026"
	}
	return out
}
