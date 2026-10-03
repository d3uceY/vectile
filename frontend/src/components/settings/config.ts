import type { AppConfig, MascotConfig } from "../../lib/types";
import { clamp, STATIC_BOUNDS } from "./bounds";

/** The flat string-list keys in the config: what addPath/removePath touch. */
export type PathKey =
  | "obsidian_vaults"
  | "obsidian_exclude_folders"
  | "project_exclude_folders"
  | "repository_exclude_folders"
  | "calibre_exclude_folders"
  | "calibre_libraries";

export const DEFAULT_MASCOT: MascotConfig = {
  show_searching: true,
  show_indexing: true,
  show_nothing: true,
};

export const cloneCfg = (c: AppConfig): AppConfig => ({
  ...c,
  obsidian_vaults: [...c.obsidian_vaults],
  obsidian_exclude_folders: [...c.obsidian_exclude_folders],
  project_exclude_folders: [...c.project_exclude_folders],
  repository_exclude_folders: [...c.repository_exclude_folders],
  calibre_exclude_folders: [...c.calibre_exclude_folders],
  calibre_libraries: [...c.calibre_libraries],
  repositories: Object.fromEntries(Object.entries(c.repositories).map(([k, v]) => [k, [...v]])),
  projects: Object.fromEntries(Object.entries(c.projects).map(([k, v]) => [k, [...v]])),
  disabled_collections: [...c.disabled_collections],
  git_commit_subject_blacklist: [...c.git_commit_subject_blacklist],
  search_defaults: { ...c.search_defaults },
  gui: { ...c.gui, mascot: { ...(c.gui.mascot ?? DEFAULT_MASCOT) } },
  mcp: { ...c.mcp },
  ocr: { ...c.ocr, languages: [...c.ocr.languages] },
});

export const sanitizeConfig = (cfg: AppConfig): AppConfig => {
  cfg.embedding_batch_size = clamp(
    cfg.embedding_batch_size,
    STATIC_BOUNDS.embedding_batch_size.min,
    STATIC_BOUNDS.embedding_batch_size.max,
  );
  cfg.chunk_size_tokens = clamp(
    cfg.chunk_size_tokens,
    STATIC_BOUNDS.chunk_size_tokens.min,
    STATIC_BOUNDS.chunk_size_tokens.max,
  );
  cfg.chunk_overlap_tokens = clamp(
    cfg.chunk_overlap_tokens,
    STATIC_BOUNDS.chunk_overlap_tokens.min,
    Math.max(STATIC_BOUNDS.chunk_overlap_tokens.min, cfg.chunk_size_tokens - 1),
  );
  cfg.git_history_in_months = clamp(
    cfg.git_history_in_months,
    STATIC_BOUNDS.git_history_in_months.min,
    STATIC_BOUNDS.git_history_in_months.max,
  );
  cfg.search_defaults.top_k = clamp(
    cfg.search_defaults.top_k,
    STATIC_BOUNDS.top_k.min,
    STATIC_BOUNDS.top_k.max,
  );
  cfg.search_defaults.rrf_k = clamp(
    cfg.search_defaults.rrf_k,
    STATIC_BOUNDS.rrf_k.min,
    STATIC_BOUNDS.rrf_k.max,
  );
  cfg.search_defaults.vector_weight = clamp(
    cfg.search_defaults.vector_weight,
    STATIC_BOUNDS.vector_weight.min,
    STATIC_BOUNDS.vector_weight.max,
  );
  cfg.search_defaults.fts_weight = clamp(
    cfg.search_defaults.fts_weight,
    STATIC_BOUNDS.fts_weight.min,
    STATIC_BOUNDS.fts_weight.max,
  );
  cfg.gui.auto_reindex_interval_minutes = clamp(
    cfg.gui.auto_reindex_interval_minutes,
    STATIC_BOUNDS.auto_reindex_interval_minutes.min,
    STATIC_BOUNDS.auto_reindex_interval_minutes.max,
  );
  cfg.mcp = cfg.mcp ?? { enabled: false, port: 31123, allow_write: false, transport: "streamable-http" };
  cfg.mcp.transport = cfg.mcp.transport === "sse" ? "sse" : "streamable-http";
  cfg.mcp.port = clamp(cfg.mcp.port, STATIC_BOUNDS.mcp_port.min, STATIC_BOUNDS.mcp_port.max);
  cfg.gui.mascot = cfg.gui.mascot ?? { ...DEFAULT_MASCOT };
  return cfg;
};
