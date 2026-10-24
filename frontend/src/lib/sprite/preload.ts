const sheets = new Map<string, { img: HTMLImageElement; ready: Promise<void> }>();

/** Fetches and decodes a sheet ahead of its first frame. The Image is kept so
    the decoded bitmap stays warm for the `<img>` that shows it; without this a
    clip change paints the old bitmap (or nothing) under the new frame geometry
    until the new strip arrives. Never rejects. */
export function loadSheet(src: string): Promise<void> {
  let entry = sheets.get(src);
  if (!entry) {
    const img = new Image();
    img.src = src;
    entry = { img, ready: img.decode().catch(() => {}) };
    sheets.set(src, entry);
  }
  return entry.ready;
}
