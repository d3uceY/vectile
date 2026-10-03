export const clamp = (n: number, min: number, max: number) => Math.min(Math.max(n, min), max);

export type NumBounds = { min: number; max: number; step: number };

export const STATIC_BOUNDS: Record<
  | "embedding_batch_size"
  | "chunk_size_tokens"
  | "chunk_overlap_tokens"
  | "git_history_in_months"
  | "top_k"
  | "rrf_k"
  | "vector_weight"
  | "fts_weight"
  | "auto_reindex_interval_minutes"
  | "mcp_port",
  NumBounds
> = {
  embedding_batch_size: { min: 1, max: 512, step: 1 },
  chunk_size_tokens: { min: 50, max: 1500, step: 10 },
  chunk_overlap_tokens: { min: 0, max: 0, step: 5 }, // max is dynamic: chunk_size - 1
  git_history_in_months: { min: 1, max: 240, step: 1 },
  top_k: { min: 1, max: 200, step: 1 },
  rrf_k: { min: 1, max: 200, step: 1 },
  vector_weight: { min: 0, max: 1, step: 0.05 },
  fts_weight: { min: 0, max: 1, step: 0.05 },
  auto_reindex_interval_minutes: { min: 1, max: 10080, step: 1 },
  mcp_port: { min: 1024, max: 65535, step: 1 },
};

/** Effective bounds for a key; chunk overlap's max tracks the current chunk size. */
export function boundsFor(key: keyof typeof STATIC_BOUNDS, chunkSize: number): NumBounds {
  const b = STATIC_BOUNDS[key];
  return key === "chunk_overlap_tokens"
    ? { ...b, max: Math.max(b.min, chunkSize - 1) }
    : b;
}
