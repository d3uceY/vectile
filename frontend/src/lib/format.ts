/** Shared formatting helpers (bytes, path basenames). */

/** "12.4 MB" / "256 KB" / "1.2 GB" for byte counts; these are approximate display sizes. */
export function fmtBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
  return `${(n / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

/** Last path segment, handling both Windows and POSIX separators. */
export function baseName(p: string): string {
  return p.split(/[\\/]/).pop() ?? p;
}

/** Everything before the last segment, separator included; "" for a bare name. */
export function dirName(p: string): string {
  const cut = Math.max(p.lastIndexOf("/"), p.lastIndexOf("\\"));
  return cut < 0 ? "" : p.slice(0, cut + 1);
}
