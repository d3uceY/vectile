/* Windowed, bidirectional keyset paging for the Browse chunk stream and the
   Library source list.

   This is the same shape as TanStack's useInfiniteQuery (cursor pages,
   fetchNextPage / fetchPreviousPage, a maxPages window) without the dependency:
   the backend hands back opaque cursors for the neighbouring pages, and the
   window keeps at most `maxPages` of them in memory. Reaching an end is simply
   "the cursor that way is empty". */

import { createMemo, createSignal, untrack, type Accessor } from "solid-js";

export interface Page<T> {
  items: T[];
  /** Cursor for the page just before these rows ("" = the list starts here). */
  before: string;
  /** Cursor for the page just after these rows ("" = the list ends here). */
  after: string;
}

export interface Pager<T> {
  items: Accessor<T[]>;
  /** False until the first page has come back, so a skeleton can cover the gap. */
  loaded: Accessor<boolean>;
  loadingTop: Accessor<boolean>;
  loadingBottom: Accessor<boolean>;
  atStart: Accessor<boolean>;
  atEnd: Accessor<boolean>;  /** Feed these to the sentinels: true while a page in that direction is in
      flight, has failed, or the list has ended. They keep a visible sentinel
      from re-requesting the same page forever. */
  stopNext: Accessor<boolean>;
  stopPrev: Accessor<boolean>;
  /** Loading that direction's next page failed; its row offers a retry. */
  failedNext: Accessor<boolean>;
  failedPrev: Accessor<boolean>;  loadNext: () => void;
  loadPrev: () => void;
  /** Drops every page and re-fetches from the top (mutations, library switch). */
  reset: () => void;
}

export function createPager<T>(opts: {
  /** Pages kept in memory. Beyond this the far page is dropped and re-fetched
      if the user scrolls back to it. */
  maxPages: number;
  fetch: (cursor: string, backward: boolean) => Promise<Page<T>>;
  /** Wraps a window mutation so the viewport can stay still (see
      withScrollAnchor). */
  onShift?: (mutate: () => void) => void;
}): Pager<T> {
  const [pages, setPages] = createSignal<Page<T>[]>([], { equals: false });
  const [loaded, setLoaded] = createSignal(false);
  const [loadingTop, setLoadingTop] = createSignal(false);
  const [loadingBottom, setLoadingBottom] = createSignal(false);
  const [failedTop, setFailedTop] = createSignal(false);
  const [failedBottom, setFailedBottom] = createSignal(false);

  // Bumped by reset(): a response that lands after the window was thrown away
  // must not be merged into the new one.
  let generation = 0;

  const items = createMemo(() => pages().flatMap((p) => p.items));

  const atStart = () => {
    const list = pages();
    return list.length > 0 && list[0].before === "";
  };
  const atEnd = () => {
    const list = pages();
    return list.length > 0 && list[list.length - 1].after === "";
  };

  const load = async (backward: boolean) => {
    const list = pages();
    // One page in flight per direction: a second concurrent fetch on the same
    // cursor would append the same rows twice.
    if (backward ? loadingTop() : loadingBottom()) return;
    if (backward ? list.length === 0 || atStart() : atEnd()) return;

    // An empty window means "start at the top", which is the only case where a
    // forward load has no cursor to resume from.
    const cursor =
      list.length === 0 ? "" : backward ? list[0].before : list[list.length - 1].after;
    const gen = generation;
    if (backward) {
      setFailedTop(false);
      setLoadingTop(true);
    } else {
      setFailedBottom(false);
      setLoadingBottom(true);
    }

    try {
      const page = await opts.fetch(cursor, backward);
      if (gen !== generation) return;

      // An empty page is kept too: its empty cursor on that side is how
      // atStart/atEnd learn the list has ended, and it contributes no rows.
      let next = backward ? [page, ...pages()] : [...pages(), page];
      if (next.length > opts.maxPages) {
        next = backward ? next.slice(0, opts.maxPages) : next.slice(next.length - opts.maxPages);
      }
      const apply = () => setPages(next);
      if (opts.onShift) opts.onShift(apply);
      else apply();
    } catch {
      // A failed page leaves the window as it was. Flag it so the visible
      // sentinel stops retrying every frame; its row offers a retry instead.
      if (gen === generation) {
        if (backward) setFailedTop(true);
        else setFailedBottom(true);
      }
    } finally {
      if (gen === generation) {
        if (backward) setLoadingTop(false);
        else setLoadingBottom(false);
        setLoaded(true);
      }
    }
  };

  return {
    items,
    loaded,
    loadingTop,
    loadingBottom,
    atStart,
    atEnd,
    stopNext: () => atEnd() || loadingBottom() || failedBottom(),
    stopPrev: () => pages().length === 0 || atStart() || loadingTop() || failedTop(),
    failedNext: failedBottom,
    failedPrev: failedTop,
    loadNext: () => untrack(() => void load(false)),
    loadPrev: () => untrack(() => void load(true)),
    reset: () => {
      generation++;
      setPages([]);
      setLoaded(false);
      setLoadingTop(false);
      setLoadingBottom(false);
      setFailedTop(false);
      setFailedBottom(false);
    },
  };
}

/** Runs `mutate` while keeping the rows above the viewport from dragging it
    along: the first still-visible row is used as an anchor, so prepending a page
    or evicting the top one leaves the scroll position exactly where it was.
    Solid patches the DOM synchronously, so measuring either side of the call is
    enough. */
export function withScrollAnchor(getScroller: () => HTMLElement | undefined, mutate: () => void) {
  const el = getScroller();
  if (!el) return mutate();

  const scrollerTop = el.getBoundingClientRect().top;
  const anchor = firstVisibleRow(el, scrollerTop);
  const anchorId = anchor?.dataset.rowId;
  const anchorTop = anchor?.getBoundingClientRect().top ?? 0;

  mutate();

  if (!anchorId) return;
  const moved = el.querySelector<HTMLElement>(`[data-row-id="${anchorId}"]`);
  if (moved) el.scrollTop += moved.getBoundingClientRect().top - anchorTop;
}

/** The first row still on screen. Rows above the viewport are exactly the ones
    an eviction drops, so anchoring on a visible row outlives the mutation. */
function firstVisibleRow(el: HTMLElement, scrollerTop: number): HTMLElement | undefined {
  for (const row of el.querySelectorAll<HTMLElement>("[data-row-id]")) {
    if (row.getBoundingClientRect().bottom > scrollerTop) return row;
  }
  return undefined;
}
