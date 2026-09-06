/* Types mirror the Go backend models (backend/services, backend/search,
   backend/config) so the UI talks to the Wails bindings without reshaping. */

export type CollectionType = "system" | "project" | "code";

export type SourceType =
  | "markdown"
  | "pdf"
  | "docx"
  | "html"
  | "plaintext"
  | "epub"
  | "code"
  | "commit"
  | "calibre-description"
  | "email"
  | "rss"; // macOS-only; kept for filter options

export interface Collection {
  id: number;
  name: string;
  type: string;
  description: string;
  sources: number;
  chunks: number;
  created: string;
  enabled: boolean;
  needsReindex: boolean;
  /** Most recent last_indexed_at among the collection's sources. */
  lastIndexed?: string;
}

export interface Source {
  id: number;
  collectionId: number;
  sourceType: string;
  path: string;
  chunks: number;
  lastIndexed?: string;
}

export interface Document {
  id: number;
  sourceId: number;
  collectionId: number;
  chunkIndex: number;
  title: string;
  content: string;
  metadata: Record<string, unknown> | null;
}

/** Mirrors search.SearchResult. */
export interface SearchResult {
  content: string;
  title: string;
  metadata: Record<string, unknown>;
  score: number;
  collection: string;
  sourcePath: string;
  sourceType: string;
}

/** Mirrors search.Filters. */
export interface SearchFilters {
  collection?: string;
  sourceType?: string;
  path?: string;
  sender?: string;
  dateFrom?: string;
  dateTo?: string;
  topK: number;
}

export type ViewId = "search" | "library" | "browse" | "index" | "settings";
export type ModelState = "loaded" | "idle" | "failed";

/** Mirrors services.Status. */
export interface Status {
  collections: number;
  sources: number;
  chunks: number;
  dbSize: number;
  modelState: ModelState;
  modelName: string;
  modelPath: string;
  modelError: string;
  lastIndexed?: string;
}

/** Mirrors config.SearchDefaults / GUIConfig / Config (snake_case JSON). */
export interface SearchDefaults {
  top_k: number;
  rrf_k: number;
  vector_weight: number;
  fts_weight: number;
}

/** Mirrors config.MascotConfig (sidebar-mascot display settings). */
export interface MascotConfig {
  show_searching: boolean;
  show_indexing: boolean;
  show_nothing: boolean;
}

export interface GUIConfig {
  auto_reindex: boolean;
  auto_reindex_interval_minutes: number;
  start_on_login: boolean;
  mascot: MascotConfig;
}

/** Mirrors config.MCPConfig (in-app MCP server settings). */
export interface MCPConfig {
  enabled: boolean;
  port: number;
  /** Gates the write tools (vectile_index / vectile_prune). */
  allow_write: boolean;
}

/** Mirrors mcp.MCPStatus (live server state from the backend). */
export interface MCPStatus {
  running: boolean;
  port: number;
  url: string;
}

export interface AppConfig {
  embedding_model: string;
  active_model: string;
  embedding_batch_size: number;
  chunk_size_tokens: number;
  chunk_overlap_tokens: number;
  obsidian_vaults: string[];
  obsidian_exclude_folders: string[];
  calibre_libraries: string[];
  repositories: Record<string, string[]>;
  projects: Record<string, string[]>;
  disabled_collections: string[];
  skip_cloud_placeholders: boolean;
  git_history_in_months: number;
  git_commit_subject_blacklist: string[];
  search_defaults: SearchDefaults;
  gui: GUIConfig;
  mcp: MCPConfig;
}

/** Mirrors services.IndexProgress / IndexComplete (indexing events). */
export interface IndexProgress {
  collection: string;
  current: number;
  total: number;
  item: string;
}

export interface IndexComplete {
  collection: string;
  indexed: number;
  skipped: number;
  errors: number;
  messages: string[];
}

/** Mirrors services.IndexFileProgress; emitted per indexed file. */
export interface IndexFileProgress {
  collection: string;
  file: string;
  indexed: number;
  total: number;
}

/** Mirrors services.IndexCancelled; emitted when a run is cancelled. */
export interface IndexCancelled {
  collection: string;
  indexed: number;
  skipped: number;
  errors: number;
}

/** Mirrors services.IndexState: snapshot of the active run, returned by
    getIndexingState() so a freshly-loaded frontend can rebuild the indexing
    UI. Live updates still arrive as events; this only seeds the initial state. */
export interface IndexState {
  active: boolean;
  all: boolean;
  collections: Record<string, IndexFileProgress>;
}

/** Mirrors db.Model: one installed embedding model. */
export interface ModelInfo {
  id: number;
  name: string;
  path: string;
  dimensions: number;
  contextWindow: number;
  batchSize: number;
  threads: number;
  isActive: boolean;
  created: string;
}

/** Mirrors services.SetActiveResult: setActiveModel() outcome. */
export interface SetActiveResult {
  needsRebuild: boolean;
  model: ModelInfo;
}

/** Mirrors services.CatalogModel: one model offered for in-app download. */
export interface CatalogModel {
  key: string;
  name: string;
  file: string;
  url: string;
  sha256: string;
  dimensions: number;
  sizeBytes: number;
  quantization: string;
  language: string;
  recommended: boolean;
  description: string;
  hfUrl: string;
}

/** Mirrors services.ModelDownloadProgress; emitted during a download. */
export interface ModelDownloadProgress {
  key: string;
  downloaded: number;
  total: number;
  percent: number;
  speed: number;
}

/** Mirrors services.ModelDownloadState: snapshot from getDownloadState(). */
export interface ModelDownloadState {
  active: boolean;
  key: string;
  status: "" | "downloading" | "done" | "failed" | "cancelled";
  downloaded: number;
  total: number;
  percent: number;
  speed: number;
  error: string;
}

/** Mirrors services.ModelDownloadError; emitted when a download fails. */
export interface ModelDownloadError {
  key: string;
  message: string;
}
