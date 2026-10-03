// Package mcp exposes vectile's local library to MCP (Model Context Protocol)
// clients over a loopback HTTP server, using either the Streamable HTTP or the
// SSE transport. Claude Desktop, Claude Code, and any other MCP client can
// search the library and inspect collections, and can trigger indexing and
// pruning when the user has enabled allow-write. Write tools are gated at call
// time by config.MCPConfig.AllowWrite.
package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"vectile/backend/services"
)

// CreateServer builds the MCP server with the vectile tools. Search, reading,
// and inspection tools are always registered; the write tools (index/prune)
// are registered too but refuse to run unless allow-write is enabled.
func CreateServer(core *services.Core) *server.MCPServer {
	s := server.NewMCPServer(
		"vectile",
		"1.1.0",
		server.WithInstructions(
			"Search and read a private, local knowledge library: Obsidian vaults, "+
				"books, code, and project documents indexed with hybrid vector + "+
				"full-text search. A search returns chunk ids and snippets; use "+
				"vectile_get_chunk or vectile_read_source to read the full text, and "+
				"vectile_grep for exact strings. Everything is read-only except "+
				"indexing and pruning, which need allow-write in Settings."),
	)

	s.AddTools(
		server.ServerTool{Tool: searchTool, Handler: handleSearch(core)},
		server.ServerTool{Tool: listCollectionsTool, Handler: handleListCollections(core)},
		server.ServerTool{Tool: collectionInfoTool, Handler: handleCollectionInfo(core)},
		server.ServerTool{Tool: getChunkTool, Handler: handleGetChunk(core)},
		server.ServerTool{Tool: readSourceTool, Handler: handleReadSource(core)},
		server.ServerTool{Tool: grepTool, Handler: handleGrep(core)},
		server.ServerTool{Tool: statusTool, Handler: handleStatus(core)},
		server.ServerTool{Tool: listSourcesTool, Handler: handleListSources(core)},
		server.ServerTool{Tool: facetsTool, Handler: handleFacets(core)},
		server.ServerTool{Tool: findRelatedTool, Handler: handleFindRelated(core)},
		server.ServerTool{Tool: timelineTool, Handler: handleTimeline(core)},
		server.ServerTool{Tool: indexTool, Handler: handleIndex(core)},
		server.ServerTool{Tool: pruneTool, Handler: handlePrune(core)},
	)

	return s
}

// Tool definitions

var searchTool = mcp.NewTool("vectile_search",
	mcp.WithDescription(
		"Search the local knowledge library using hybrid vector + full-text "+
			"search with Reciprocal Rank Fusion. Searches across all indexed "+
			"collections by default, combining semantic similarity with keyword "+
			"matching. Each result carries a chunk id and a snippet; call "+
			"vectile_get_chunk with the id for the full text. Read-only."),
	mcp.WithString("query",
		mcp.Required(),
		mcp.Description("Search query text (natural language or keywords)")),
	mcp.WithString("collection",
		mcp.Description("Filter by collection name. Omit to search all.")),
	mcp.WithNumber("top_k",
		mcp.Description("Number of results to return (default: the configured top-k)")),
	mcp.WithNumber("max_chars",
		mcp.Description("Snippet length per result, in characters (default 1200, max 8000)")),
	mcp.WithString("source_type",
		mcp.Description("Filter by type: 'markdown', 'pdf', 'docx', 'epub', 'html', "+
			"'plaintext', 'code', 'commit', or 'calibre-description'.")),
	mcp.WithString("path",
		mcp.Description("Filter by source path (case-insensitive substring of the "+
			"absolute file path). Use to scope to a subfolder or repo, e.g. "+
			"'backend/services' or 'infrastructure/modules'.")),
	mcp.WithString("date_from",
		mcp.Description("Results after this date (YYYY-MM-DD)")),
	mcp.WithString("date_to",
		mcp.Description("Results before this date (YYYY-MM-DD)")),
	mcp.WithString("sender",
		mcp.Description("Filter by email sender (case-insensitive substring)")),
	mcp.WithString("author",
		mcp.Description("Filter by book author (case-insensitive substring)")),
	mcp.WithObject("metadata_filter",
		mcp.Description("Filter by arbitrary metadata fields. JSON object of "+
			"key-value string pairs. Matches are case-insensitive substring for "+
			"strings, element-wise for arrays.")),
)

var listCollectionsTool = mcp.NewTool("vectile_list_collections",
	mcp.WithDescription(
		"List all collections in the library with source file counts, chunk "+
			"counts, and last-indexed time. Collections of type 'code' represent "+
			"repository collections that may contain multiple git repos."),
)

var collectionInfoTool = mcp.NewTool("vectile_collection_info",
	mcp.WithDescription(
		"Get detailed information about a specific collection: source count, "+
			"chunk count, source type breakdown, last indexed timestamp, and a "+
			"sample of document titles."),
	mcp.WithString("collection",
		mcp.Required(),
		mcp.Description("The collection name. Use vectile_list_collections() to "+
			"discover available names.")),
)

var indexTool = mcp.NewTool("vectile_index",
	mcp.WithDescription(
		"Trigger an indexing run for one collection: an Obsidian vault, a "+
			"Calibre library, or a configured project/repo collection. Blocks until "+
			"the run finishes and returns the summary (indexed/skipped/errors). "+
			"Requires 'Allow write tools' to be enabled in vectile Settings."),
	mcp.WithString("collection",
		mcp.Required(),
		mcp.Description("The collection name. Use vectile_list_collections() to "+
			"discover available names.")),
	mcp.WithBoolean("force",
		mcp.Description("Re-index everything, clearing existing data (default: "+
			"false, incremental).")),
)

var pruneTool = mcp.NewTool("vectile_prune",
	mcp.WithDescription(
		"Remove stale indexed entries whose originals no longer exist: deleted "+
			"files from a library, removed books, and deleted code files. "+
			"Requires 'Allow write tools' to be enabled in vectile Settings."),
	mcp.WithString("collection",
		mcp.Description("Collection to prune. Omit to prune all collections.")),
)
