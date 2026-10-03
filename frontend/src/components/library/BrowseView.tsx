import {
  createEffect,
  createMemo,
  createSignal,
  createUniqueId,
  For,
  onCleanup,
  Show,
  untrack,
  type JSX,
} from "solid-js";
import * as api from "../../lib/api";
import { baseName } from "../../lib/format";
import { createPager, withScrollAnchor } from "../../lib/pager";
import { useAppStore } from "../../lib/store";
import type { Document, DocumentSummary, Source } from "../../lib/types";
import { ScrollSentinel } from "../ui/ScrollSentinel";
import { Button, ConfirmDialog, EmptyState, Select, Skeleton, ViewHeading } from "../ui/primitives";
import { BrowseIcon, CheckIcon, ChevronLeft, CloseIcon, TrashIcon } from "../ui/icons";

/* Browse reads a collection the way a file manager does: the sources are a grid
   of folders, and a folder opens a sheet with the chunks inside it. Three
   levels, one of them only when asked for.

   The paging work underneath is unchanged, because it is what keeps a
   hundred-thousand-chunk library cheap: the folder grid is a bounded window of
   keyset pages, a file's chunks are read only when that file is opened, and a
   chunk's text is read only when the chunk is opened. */

/* Folder pages kept in memory. Past this the far page is dropped and fetched
   again if the user scrolls back, so a collection with thousands of files costs
   the same as a small one. */
const MAX_PAGES = 5;

export function BrowseView() {
  const store = useAppStore();
  const drawerTitleId = createUniqueId();

  let scroller: HTMLDivElement | undefined;
  const getScroller = () => scroller;
  let drawerEl: HTMLElement | undefined;

  const [colId, setColId] = createSignal<number | null>(null);
  const collection = createMemo(
    () => store.collections().find((c) => c.id === colId()) ?? store.collections()[0] ?? null,
  );

  // --- the folder grid -------------------------------------------------

  const grid = createPager<Source>({
    maxPages: MAX_PAGES,
    fetch: async (cursor, backward) => {
      const c = collection();
      if (!c) return { items: [], before: "", after: "" };
      const page = await api.listSourcesPage(c.id, cursor, backward);
      return { items: page.sources, before: page.before, after: page.after };
    },
    // Prepending a page (or dropping the top one) shifts the grid below it, so
    // the scroll position is re-anchored on a tile that survives the change.
    onShift: (mutate) => withScrollAnchor(getScroller, mutate),
  });

  // --- the open file ---------------------------------------------------

  const [openSource, setOpenSource] = createSignal<Source | null>(null);
  const [chunks, setChunks] = createSignal<DocumentSummary[]>([]);
  const [chunkLoading, setChunkLoading] = createSignal(false);
  const [chunkDone, setChunkDone] = createSignal(false);
  const [chunkError, setChunkError] = createSignal(false);
  const [checked, setChecked] = createSignal<Set<number>>(new Set());
  let chunkCursor = "";
  let chunkGen = 0;
  /** The tile that opened the sheet, so focus can go back to it on close. */
  let opener: HTMLElement | null = null;

  /* A file's chunks are one contiguous run of the paged stream, which is
     ordered by (source, chunk index). Seek to just before the run and page
     forward until a row from a different source shows up: that is the run's
     end. Nothing is read until a file is opened, and a long file arrives a page
     at a time as the sheet scrolls. */
  const loadChunks = async (fresh: boolean) => {
    const s = openSource();
    const c = collection();
    if (!s || !c) return;
    if (fresh) {
      chunkGen++;
      chunkCursor = `${s.id}:-1`;
      setChunks([]);
      setChunkDone(false);
      setChunkError(false);
    } else if (chunkLoading() || chunkDone()) {
      return;
    }
    const gen = chunkGen;
    setChunkLoading(true);
    try {
      const page = await api.listDocumentsPage(c.id, chunkCursor, false);
      if (gen !== chunkGen) return;
      const mine = page.documents.filter((d) => d.sourceId === s.id);
      setChunks((prev) => (fresh ? mine : [...prev, ...mine]));
      chunkCursor = page.after;
      // A row from another source means we walked past the run; an empty page
      // or no cursor on the far side means the whole stream ended under us.
      setChunkDone(
        page.documents.length !== mine.length || page.documents.length === 0 || !page.after,
      );
    } catch {
      if (gen === chunkGen) setChunkError(true);
    } finally {
      if (gen === chunkGen) setChunkLoading(false);
    }
  };

  // --- the chunk being read --------------------------------------------

  const [chunkView, setChunkView] = createSignal<"list" | "content">("list");
  const [content, setContent] = createSignal<Document | null>(null);
  const [contentLoading, setContentLoading] = createSignal(false);
  const [contentError, setContentError] = createSignal(false);
  const [lastChunkId, setLastChunkId] = createSignal<number | null>(null);
  let contentGen = 0;

  const openChunk = (id: number) => {
    setLastChunkId(id);
    setChunkView("content");
    contentGen++;
    const gen = contentGen;
    setContent(null);
    setContentError(false);
    setContentLoading(true);
    api
      .getDocument(id)
      .then((d) => {
        if (gen === contentGen) setContent(d);
      })
      .catch(() => {
        if (gen === contentGen) setContentError(true);
      })
      .finally(() => {
        if (gen === contentGen) setContentLoading(false);
      });
  };

  const openFile = (s: Source) => {
    // Remembered so closing the sheet puts focus back where it came from.
    opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    setOpenSource(s);
    setChunkView("list");
    setChecked(new Set<number>());
    void loadChunks(true);
  };

  const closeDrawer = () => {
    chunkGen++; // a page still in flight belongs to the closed sheet
    contentGen++;
    setOpenSource(null);
    setChunks([]);
    setChecked(new Set<number>());
    setChunkView("list");
    setContent(null);
    opener?.focus();
    opener = null;
  };

  // --- lifecycle -------------------------------------------------------

  // A new collection, or any index/delete that changed the data, starts the
  // folder window over. An open sheet reloads in place rather than closing, so
  // deleting chunks refreshes it.
  createEffect(() => {
    const id = colId();
    store.libraryEpoch();
    grid.reset();
    setChecked(new Set<number>());
    if (id !== null) grid.loadNext();
    untrack(() => {
      const s = openSource();
      if (!s) return;
      if (s.collectionId !== id) closeDrawer();
      else void loadChunks(true);
    });
  });

  createEffect(() => {
    if (colId() === null) {
      const first = store.collections().find((c) => c.chunks > 0);
      if (first) setColId(first.id);
    }
  });

  // On open, focus the sheet; Escape then steps back one level: content →
  // chunk list → grid.
  createEffect(() => {
    if (!openSource()) return;
    drawerEl?.focus();
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      e.preventDefault();
      if (chunkView() === "content") setChunkView("list");
      else closeDrawer();
    };
    document.addEventListener("keydown", onKey);
    onCleanup(() => document.removeEventListener("keydown", onKey));
  });

  // --- selection -------------------------------------------------------

  const nChecked = () => checked().size;
  const toggleCheck = (id: number) =>
    setChecked((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  const allLoaded = () => chunks().length >= (openSource()?.chunks ?? 0);
  const selectAll = () =>
    setChecked((prev) => {
      const next = new Set(prev);
      for (const d of chunks()) next.add(d.id);
      return next;
    });

  // --- delete dialogs --------------------------------------------------

  const [confirm, setConfirm] = createSignal<
    | { kind: "library"; name: string; chunks: number }
    | { kind: "chunks"; count: number }
    | null
  >(null);
  const [deleting, setDeleting] = createSignal(false);

  const doDelete = async () => {
    const t = confirm();
    if (!t) return;
    setDeleting(true);
    const ok =
      t.kind === "library"
        ? await store.deleteCollection(t.name, t.name)
        : await store.deleteDocuments(
            [...checked()],
            `${t.count} selected chunk${t.count === 1 ? "" : "s"}`,
          );
    setDeleting(false);
    if (!ok) return;
    setConfirm(null);
    if (t.kind === "library") {
      const next = store.collections().find((c) => c.name !== t.name);
      setColId(next ? next.id : null);
      closeDrawer();
    }
  };

  const confirmTitle = () => {
    const t = confirm();
    if (!t) return "";
    return t.kind === "library"
      ? `Delete the ${t.name} library?`
      : `Delete ${t.count} chunk${t.count === 1 ? "" : "s"}?`;
  };

  const confirmBody = () => {
    const t = confirm();
    if (!t) return null;
    return t.kind === "library" ? (
      <span>
        Removes all {t.chunks.toLocaleString()} chunks in {t.name}. The sources stay configured, so
        re-indexing brings it back. Files on disk are untouched.
      </span>
    ) : (
      <span>
        The selected chunks leave the index: their embeddings and search entries go with them.
        Sources and files on disk stay.
      </span>
    );
  };

  const hasAnything = () => store.collections().some((c) => c.chunks > 0);

  return (
    <div class="relative flex h-full flex-col">
      <div class="relative">
        <ViewHeading title="Browse" note="Open a source to read its chunks." />
      </div>

      <div class="relative flex min-h-0 flex-1 flex-col pb-8">
        <Show
          when={hasAnything()}
          fallback={
            <div class="flex flex-1 items-center justify-center">
              <EmptyState
                icon={<BrowseIcon size={20} />}
                title={
                  store.collections().length > 0
                    ? "Indexed, but nothing to show yet"
                    : "Nothing to browse yet"
                }
                note={
                  store.collections().length > 0
                    ? "No sources indexed yet. Run an index pass."
                    : "Index a folder or library to see its files here."
                }
              >
                <Button
                  onClick={() =>
                    store.setView(store.collections().length > 0 ? "index" : "settings")
                  }
                >
                  {store.collections().length > 0 ? "Go to Index" : "Add sources in Settings"}
                </Button>
              </EmptyState>
            </div>
          }
        >
          <div class="mb-3 flex flex-wrap items-center gap-2">
            <Select
              aria-label="Collection"
              value={collection() ? String(collection()!.id) : ""}
              onChange={(v) => {
                const id = Number(v);
                setColId(Number.isFinite(id) && id > 0 ? id : null);
              }}
              options={store.collections().map((c) => ({ value: String(c.id), label: c.name }))}
            />
            <Show when={collection()}>
              <Button
                size="sm"
                variant="danger"
                class="ml-auto"
                onClick={() =>
                  setConfirm({
                    kind: "library",
                    name: collection()!.name,
                    chunks: collection()!.chunks,
                  })
                }
              >
                <TrashIcon size={14} /> Delete library
              </Button>
            </Show>
          </div>

          <div ref={scroller} class="sheet scroll-quiet min-h-0 flex-1 overflow-y-auto p-3">
            <ScrollSentinel root={getScroller} onVisible={grid.loadPrev} stop={grid.stopPrev} />
            <Show when={grid.loadingTop()}>
              <Hint>Loading earlier files</Hint>
            </Show>
            <Show when={grid.failedPrev()}>
              <Hint>
                Couldn't load earlier files. <Retry onClick={grid.loadPrev} />
              </Hint>
            </Show>

            <Show when={grid.loaded()} fallback={<GridSkeleton />}>
              <Show
                when={grid.items().length > 0}
                fallback={
                  <p class="note px-3 py-8 text-center text-[14px] leading-5 text-muted">
                    No files in this collection yet. Index it, or pick another collection.
                  </p>
                }
              >
                <ul class="grid grid-cols-[repeat(auto-fill,minmax(8.25rem,1fr))] gap-x-1 gap-y-1.5">
                  <For each={grid.items()}>
                    {(s) => <FileTile source={s} onOpen={() => openFile(s)} />}
                  </For>
                </ul>
              </Show>
            </Show>

            <Show when={grid.loadingBottom()}>
              <Hint>Loading more files</Hint>
            </Show>
            <Show when={grid.failedNext()}>
              <Hint>
                Couldn't load more files. <Retry onClick={grid.loadNext} />
              </Hint>
            </Show>
            <ScrollSentinel root={getScroller} onVisible={grid.loadNext} stop={grid.stopNext} />
          </div>
        </Show>
      </div>

      {/* The file's chunks slide in from the right; opening one swaps the sheet
          to its text, so a file is read without leaving the grid behind it. */}
      <Show when={openSource()}>
        {(s) => (
          <>
            <div
              class="scrim-in fixed inset-0 z-40 bg-ink/20"
              onClick={closeDrawer}
              aria-hidden="true"
            />
            <aside
              ref={drawerEl}
              role="dialog"
              aria-modal="true"
              aria-labelledby={drawerTitleId}
              tabindex={-1}
              class="drawer-in fixed inset-y-0 right-0 z-50 flex w-full max-w-140 flex-col border-l border-line bg-surface shadow-pop"
            >
              <header class="flex items-start gap-2 border-b border-line px-4 py-3">
                <Show when={chunkView() === "content"}>
                  <button
                    type="button"
                    class="-ml-1 mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-control text-muted transition-colors hover:bg-surface-2 hover:text-ink"
                    onClick={() => setChunkView("list")}
                    aria-label="Back to the chunk list"
                  >
                    <ChevronLeft size={16} />
                  </button>
                </Show>
                <div class="min-w-0 flex-1">
                  <h2
                    id={drawerTitleId}
                    class="title truncate text-[15px] leading-6 tracking-[-0.01em] text-ink"
                  >
                    {baseName(s().path)}
                  </h2>
                  <p class="data mt-0.5 text-muted">
                    {s().chunks.toLocaleString()} chunk{s().chunks === 1 ? "" : "s"}
                  </p>
                </div>
                <button
                  type="button"
                  class="-mr-1 mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-control text-muted transition-colors hover:bg-surface-2 hover:text-ink"
                  onClick={closeDrawer}
                  aria-label="Close"
                >
                  <CloseIcon size={16} />
                </button>
              </header>

              <Show
                when={chunkView() === "list"}
                fallback={
                  <div class="scroll-quiet min-h-0 flex-1 overflow-y-auto p-4">
                    <Show when={content()}>
                      {(doc) => (
                        <>
                          <p class="data mb-2 text-muted">chunk {doc().chunkIndex + 1}</p>
                          <p class="read max-w-[68ch] whitespace-pre-wrap text-[15px] leading-[1.7] text-ink-soft">
                            {doc().content}
                          </p>
                        </>
                      )}
                    </Show>
                    <Show when={!content() && contentLoading()}>
                      <div class="space-y-2">
                        <Skeleton class="h-4 w-full" />
                        <Skeleton class="h-4 w-5/6" />
                        <Skeleton class="h-4 w-3/4" />
                      </div>
                    </Show>
                    <Show when={!content() && !contentLoading() && contentError()}>
                      <p class="data text-[12px] text-muted">
                        Couldn't load this chunk.{" "}
                        <button
                          type="button"
                          class="text-indigo hover:underline"
                          onClick={() => {
                            const id = lastChunkId();
                            if (id !== null) openChunk(id);
                          }}
                        >
                          Retry
                        </button>
                      </p>
                    </Show>
                  </div>
                }
              >
                <div
                  class={`flex flex-wrap items-center gap-x-2 gap-y-1 px-3 py-1.5 ${
                    nChecked() > 0 ? "bg-indigo-soft" : ""
                  }`}
                >
                  <Show when={nChecked() > 0}>
                    <span class="data whitespace-nowrap text-[12px] font-medium text-ink">
                      {nChecked()} selected
                    </span>
                  </Show>
                  <button
                    type="button"
                    class="data text-[12px] text-indigo hover:underline"
                    onClick={selectAll}
                  >
                    {allLoaded() ? "Select all" : "Select loaded"}
                  </button>
                  <Show when={nChecked() > 0}>
                    <button
                      type="button"
                      class="data text-[12px] text-indigo hover:underline"
                      onClick={() => setChecked(new Set<number>())}
                    >
                      Clear
                    </button>
                  </Show>
                  <Button
                    size="sm"
                    variant="danger"
                    class="ml-auto"
                    disabled={nChecked() === 0}
                    onClick={() => setConfirm({ kind: "chunks", count: nChecked() })}
                  >
                    <TrashIcon size={12} /> Delete
                  </Button>
                </div>

                <div
                  onScroll={(e) => {
                    const el = e.currentTarget;
                    if (el.scrollHeight - el.scrollTop - el.clientHeight < 320) {
                      void loadChunks(false);
                    }
                  }}
                  class="scroll-quiet min-h-0 flex-1 overflow-y-auto p-2"
                >
                  <Show
                    when={chunks().length > 0}
                    fallback={
                      <Show when={!chunkLoading() && !chunkError()}>
                        <p class="note px-3 py-8 text-center text-[14px] leading-5 text-muted">
                          No chunks in this file.
                        </p>
                      </Show>
                    }
                  >
                    <ul class="enter-stagger">
                      <For each={chunks()}>
                        {(d) => (
                          <li class="group flex items-center gap-2 rounded-control py-0.5 pr-2 pl-1.5 transition-colors duration-100 ease-snappy hover:bg-surface-2">
                            <button
                              type="button"
                              role="checkbox"
                              aria-checked={checked().has(d.id)}
                              aria-label={`Select chunk ${d.chunkIndex + 1} of ${baseName(s().path)}`}
                              onClick={() => toggleCheck(d.id)}
                              class={`flex h-4 w-4 shrink-0 items-center justify-center rounded-sm border transition-colors duration-100 ease-snappy ${
                                checked().has(d.id)
                                  ? "border-indigo bg-indigo text-white"
                                  : "border-line-control bg-paper text-transparent hover:border-indigo/60"
                              }`}
                            >
                              <CheckIcon size={11} strokeWidth={2.5} />
                            </button>
                            <button
                              type="button"
                              onClick={() => openChunk(d.id)}
                              class="flex min-w-0 flex-1 items-center py-1.5 text-left"
                            >
                              <span class="truncate text-[13px] text-ink-soft">{d.title}</span>
                            </button>
                            <span class="data shrink-0 text-muted">chunk {d.chunkIndex + 1}</span>
                          </li>
                        )}
                      </For>
                    </ul>
                  </Show>

                  <Show when={chunkLoading()}>
                    <Hint>Loading chunks</Hint>
                  </Show>
                  <Show when={chunkError()}>
                    <Hint>
                      Couldn't load this file's chunks.{" "}
                      <Retry onClick={() => void loadChunks(false)} />
                    </Hint>
                  </Show>
                </div>
              </Show>
            </aside>
          </>
        )}
      </Show>

      <ConfirmDialog
        open={confirm() !== null}
        title={confirmTitle()}
        body={confirmBody()}
        busy={deleting()}
        onCancel={() => setConfirm(null)}
        onConfirm={doDelete}
      />
    </div>
  );
}

/* ---- pieces ---------------------------------------------------------- */

/** One source as a folder: the pinned folder art, its chunk count on the flap,
    its name under it. Nothing else, because nothing else is asked for. */
function FileTile(props: { source: Source; onOpen: () => void }) {
  return (
    <li data-row-id={props.source.id} class="min-w-0">
      <button
        type="button"
        onClick={props.onOpen}
        title={props.source.path}
        class="group flex w-full flex-col items-center gap-1.5 rounded-card px-2 pt-2 pb-1.5 text-center transition-colors duration-100 ease-snappy hover:bg-surface-2"
      >
        <span class="relative block w-full">
          <img
            src="/folder.svg"
            alt=""
            draggable={false}
            class="mx-auto block w-[74%] max-w-27"
          />
          <span class="data pointer-events-none absolute left-1/2 top-[64%] -translate-x-1/2 -translate-y-1/2 rounded-full border border-line bg-surface px-1.5 py-px text-[11px] font-semibold tabular-nums text-ink">
            {props.source.chunks.toLocaleString()}
            <span class="sr-only"> chunks</span>
          </span>
        </span>
        <span class="line-clamp-2 w-full text-[12.5px] leading-4 text-ink-soft group-hover:text-ink">
          {baseName(props.source.path)}
        </span>
      </button>
    </li>
  );
}

function GridSkeleton() {
  return (
    <div
      class="grid grid-cols-[repeat(auto-fill,minmax(8.25rem,1fr))] gap-x-1 gap-y-1.5"
      aria-hidden="true"
    >
      <For each={[0, 1, 2, 3, 4, 5, 6, 7]}>
        {() => (
          <div class="flex flex-col items-center gap-2 px-2 pt-2 pb-1.5">
            <Skeleton class="h-14 w-[74%] max-w-27" />
            <Skeleton class="h-3.5 w-3/4" />
          </div>
        )}
      </For>
    </div>
  );
}

function Hint(props: { children: JSX.Element }) {
  return <p class="data px-3 py-1.5 text-[12px] text-muted">{props.children}</p>;
}

function Retry(props: { onClick: () => void }) {
  return (
    <button type="button" class="text-indigo hover:underline" onClick={props.onClick}>
      Retry
    </button>
  );
}
