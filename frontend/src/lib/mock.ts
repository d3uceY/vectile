/** Example queries for the search idle state + snippet term splitting. */

export const exampleQueries = [
  "kubernetes rollout",
  "exposure triangle",
  "quorum",
  "how do we ship changes safely",
];

/** Query terms, for snippet highlighting. */
export function termsOf(query: string): string[] {
  return query
    .trim()
    .toLowerCase()
    .split(/\s+/)
    .filter((t) => t.length > 1);
}
