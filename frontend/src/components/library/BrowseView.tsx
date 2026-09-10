import { createEffect, createMemo, createSignal, For, onCleanup, Show, type JSX } from "solid-js";
import * as api from "../../lib/api";
import { baseName } from "../../lib/format";
import { createPager, withScrollAnchor } from "../../lib/pager";
import { useAppStore } from "../../lib/store";
import type { Document, DocumentSummary } from "../../lib/types";
import { ScrollSentinel } from "../ui/ScrollSentinel";
import {
  ViewHeading,
  Button,
  Chip,
  ConfirmDialog,
  EmptyState,
  Select,
  Skeleton,
} from "../ui/primitives";
import { BrowseIcon, CheckIcon, FileIcon, FolderIcon, TrashIcon } from "../ui/icons";

/* Pages of chunks kept in memory (the backend sends 100 rows per page). Past six
   the far page is dropped and fetched again if the user scrolls back to it, so a
   library of a hundred thousand chunks costs the same as a small one. */
const MAX_PAGES = 6;

type Row =
  | { kind: "source"; path: string; sourceType: string }
  | { kind: "chunk"; doc: DocumentSummary };

export function BrowseView() {
  const store = useAppStore();

  let scroller: HTMLDivElement | undefined;
  const getScroller = () => scroller;

  const [colId, setColId] = createSignal<number | null>(null);
  const [checked, setChecked] = createSignal<Set<number>>(new Set());
  const [selectedId, setSelectedId] = createSignal<number | null>(null);
  const [selectedMeta, setSelectedMeta] = createSignal<DocumentSummary | null>(null);
  const [detail, setDetail] = createSignal<Document | null>(null);
  const [detailLoading, setDetailLoading] = createSignal(false);

  const collection = createMemo(
    () => store.collections().find((c) => c.id === colId()) ?? store.collections()[0] ?? null,
  );

  const pager = createPager<DocumentSummary>({
    maxPages: MAX_PAGES,
    fetch: async (cursor, backward) => {
      const c = collection();
      if (!c) return { items: [], before: "", after: "" };
      const page = await api.listDocumentsPage(c.id, cursor, backward);
      return { items: page.documents, before: page.before, after: page.after };
    },
    // Prepending a page (or dropping the top one) shifts everything below it, so
    // the scroll position is re-anchored on a row that survives the change.
    onShift: (mutate) => withScrollAnchor(getScroller, mutate),
  });

  // A new library, or any index/delete that changed the data, starts the window
  // over. Nothing else has to reload: the window is the only copy in the UI.
  createEffect(() => {
    const id = colId();
    store.libraryEpoch();
    pager.reset();
    setChecked(new Set<number>());
    setSelectedId(null);
    if (id !== null) pager.loadNext();
  });

  // Select the first chunk so the reading pane has something in it.
  createEffect(() => {
    if (selectedId() !== null) return;
    const first = pager.items()[0];
    if (first) setSelectedId(first.id);
  });

  // Keep the row's own facts (its file path) after the row itself has been
  // evicted from the window by scrolling away.
  createEffect(() => {
    const d = pager.items().find((x) => x.id === selectedId());
    if (d) setSelectedMeta(d);
  });

  // The text lives in its own request: a page of rows drags no text along.
  createEffect(() => {
    const id = selectedId();
    if (id === null) {
      setDetail(null);
      return;
    }
    let stale = false;
    setDetailLoading(true);
    api
      .getDocument(id)
      .then((d) => {
        if (!stale) setDetail(d);
      })
      .catch(() => {
        if (!stale) setDetail(null);
      })
      .finally(() => {
        if (!stale) setDetailLoading(false);
      });
    onCleanup(() => {
      stale = true;
    });
  });

  // Chunk rows, with a file header wherever the source changes.
  const rows = createMemo<Row[]>(() => {
    const out: Row[] = [];
    let sourceId: number | null = null;
    for (const d of pager.items()) {
      if (d.sourceId !== sourceId) {
        sourceId = d.sourceId;
        out.push({ kind: "source", path: d.sourcePath, sourceType: d.sourceType });
      }
      out.push({ kind: "chunk", doc: d });
    }
    return out;
  });

  const total = () => collection()?.chunks ?? 0;
  const loadedCount = () => pager.items().length;
  const nChecked = () => checked().size;

  const toggleCheck = (id: number) =>
    setChecked((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  const selectAll = () =>
    setChecked((prev) => {
      const next = new Set(prev);
      for (const d of pager.items()) next.add(d.id);
      return next;
    });

  const onLibraryChange = (value: string) => {
    const id = Number(value);
    setColId(Number.isFinite(id) && id > 0 ? id : null);
  };

  createEffect(() => {
    if (colId() === null) {
      const first = store.collections().find((c) => c.chunks > 0);
      if (first) setColId(first.id);
    }
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
        Removes all {t.chunks.toLocaleString()} chunks in {t.name} and its sources, and removes it
        from Settings. Files on disk are untouched.
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
        <ViewHeading title="Browse" note="Read the chunks inside your files." />
      </div>

      {/* pb-8 leaves a deliberate gap under the panes instead of stretching them
          to the window edge. */}
      <div class="relative @container flex min-h-0 flex-1 flex-col pb-8">
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
                    ? "No documents indexed yet. Run an index pass."
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
              aria-label="Library"
              value={collection() ? String(collection()!.id) : ""}
              onChange={onLibraryChange}
              options={store.collections().map((c) => ({ value: String(c.id), label: c.name }))}
            />
            <Chip tone="mint">{total().toLocaleString()} chunks</Chip>
            <div class="flex-1" />
            <Show when={collection()}>
              <Button
                size="sm"
                variant="outline"
                class="border-danger/30 text-danger hover:border-danger/50 hover:text-danger"
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

          <div class="flex min-h-0 flex-1 flex-col gap-4 @lg:grid @lg:grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)]">
            <div class="flex min-h-0 flex-1 flex-col gap-2">
              <div class="flex items-center gap-2 rounded-lg bg-indigo-soft/70 px-2.5 py-1.5">
                <span class="data text-[12px] font-medium text-ink">
                  {nChecked() > 0 ? `${nChecked()} selected` : "Select chunks to delete"}
                </span>
                <button
                  type="button"
                  class="data text-[12px] text-indigo hover:underline"
                  onClick={selectAll}
                >
                  {loadedCount() < total() ? "Select loaded" : "Select all"}
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
                <div class="flex-1" />
                <button
                  type="button"
                  disabled={nChecked() === 0}
                  class="inline-flex items-center gap-1.5 rounded-control bg-danger px-2.5 py-1 text-[12px] font-medium text-white transition-opacity hover:opacity-90 disabled:pointer-events-none disabled:opacity-40"
                  onClick={() => setConfirm({ kind: "chunks", count: nChecked() })}
                >
                  <TrashIcon size={12} /> Delete
                </button>
              </div>

              <div ref={scroller} class="sheet scroll-quiet min-h-0 flex-1 overflow-y-auto py-1.5">
                <ScrollSentinel
                  root={getScroller}
                  onVisible={pager.loadPrev}
                  stop={pager.stopPrev}
                />
                <Show when={pager.loadingTop()}>
                  <Hint>Loading earlier chunks</Hint>
                </Show>
                <Show when={pager.failedPrev()}>
                  <Hint>
                    Couldn't load earlier chunks.{" "}
                    <button class="text-indigo hover:underline" onClick={pager.loadPrev}>
                      Retry
                    </button>
                  </Hint>
                </Show>

                <Show
                  when={loadedCount() > 0}
                  fallback={
                    <Show when={pager.loaded()}>
                      <div class="flex h-32 items-center justify-center">
                        <p class="note max-w-[16rem] text-center text-[14px] leading-5 text-muted">
                          No chunks yet. Index it or pick another library.
                        </p>
                      </div>
                    </Show>
                  }
                >
                  <For each={rows()}>
                    {(row) => (
                      <Show
                        when={"doc" in row ? row.doc : null}
                        fallback={
                          <SourceHeader
                            path={(row as { path: string }).path}
                            sourceType={(row as { sourceType: string }).sourceType}
                          />
                        }
                      >
                        {(doc) => (
                          <ChunkRow
                            doc={doc()}
                            selected={selectedId() === doc().id}
                            checked={checked().has(doc().id)}
                            onSelect={() => setSelectedId(doc().id)}
                            onToggleCheck={() => toggleCheck(doc().id)}
                          />
                        )}
                      </Show>
                    )}
                  </For>
                </Show>

                <Show when={pager.loadingBottom()}>
                  <Hint>Loading more chunks</Hint>
                </Show>
                <Show when={pager.failedNext()}>
                  <Hint>
                    Couldn't load more chunks.{" "}
                    <button class="text-indigo hover:underline" onClick={pager.loadNext}>
                      Retry
                    </button>
                  </Hint>
                </Show>
                <Show when={pager.atEnd() && loadedCount() > 0}>
                  <Hint>End of library</Hint>
                </Show>
                <ScrollSentinel root={getScroller} onVisible={pager.loadNext} stop={pager.stopNext} />
              </div>
            </div>

            <ReadingPane
              meta={selectedMeta()}
              doc={detail()}
              loading={detailLoading()}
              collectionName={collection()?.name ?? ""}
            />
          </div>
        </Show>
      </div>

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

function Hint(props: { children: JSX.Element }) {
  return <p class="data px-3 py-1.5 text-[12px] text-muted">{props.children}</p>;
}

function SourceHeader(props: { path: string; sourceType: string }) {
  return (
    <div class="sticky top-0 z-10 flex items-center gap-2 border-b border-line bg-paper/95 px-3 py-1.5 backdrop-blur-[2px]">
      <FolderIcon size={14} class="shrink-0 text-faint" />
      <span class="shrink-0 text-[12.5px] font-medium text-ink">{baseName(props.path)}</span>
      <span class="data min-w-0 flex-1 truncate text-muted">{props.path}</span>
      <span class="data shrink-0 text-faint">{props.sourceType}</span>
    </div>
  );
}

function ChunkRow(props: {
  doc: DocumentSummary;
  selected: boolean;
  checked: boolean;
  onSelect: () => void;
  onToggleCheck: () => void;
}) {
  return (
    <div
      data-row-id={props.doc.id}
      onClick={props.onSelect}
      class={`group mx-1.5 flex cursor-pointer items-center gap-2 rounded-lg py-1 pr-2 pl-1.5 transition-colors duration-100 ease-snappy ${
        props.selected ? "bg-indigo-mist" : "hover:bg-surface"
      }`}
    >
      <button
        type="button"
        role="checkbox"
        aria-checked={props.checked}
        aria-label={`Select chunk ${props.doc.chunkIndex + 1} of ${props.doc.title}`}
        onClick={(e) => {
          e.stopPropagation();
          props.onToggleCheck();
        }}
        class={`flex h-4 w-4 shrink-0 items-center justify-center rounded-sm border transition-colors duration-100 ease-snappy ${
          props.checked
            ? "border-indigo bg-indigo text-white"
            : "border-line-strong bg-paper text-transparent hover:border-indigo/60"
        }`}
      >
        <CheckIcon size={11} strokeWidth={2.5} />
      </button>
      <button
        type="button"
        onClick={(e) => {
          e.stopPropagation();
          props.onSelect();
        }}
        aria-current={props.selected}
        class="flex min-w-0 flex-1 items-center gap-2 text-left"
      >
        <FileIcon size={14} class="shrink-0 text-faint" />
        <span class="truncate text-[13px] text-ink-soft">{props.doc.title}</span>
      </button>
      <span class="data shrink-0 text-muted">chunk {props.doc.chunkIndex + 1}</span>
    </div>
  );
}

function ReadingPane(props: {
  meta: DocumentSummary | null;
  doc: Document | null;
  loading: boolean;
  collectionName: string;
}) {
  const tags = () => {
    const t = props.doc?.metadata?.tags;
    return Array.isArray(t) ? (t as string[]) : [];
  };

  return (
    <div class="sheet flex min-h-0 flex-1 flex-col overflow-hidden">
      <Show
        when={props.meta}
        fallback={
          <Show when={props.loading}>
            <div class="flex flex-1 items-center justify-center">
              <Skeleton class="h-24 w-2/3" />
            </div>
          </Show>
        }
      >
        {(meta) => (
          <>
            <div class="border-b border-line bg-paper/60 px-5 py-4">
              <h3 class="title text-[16px] leading-6 tracking-[-0.005em] text-ink">
                {meta().title}
              </h3>
              <p class="data mt-1.5 truncate text-muted">{meta().sourcePath}</p>
              <div class="mt-3 flex flex-wrap items-center gap-1.5">
                <Chip tone="mint">{props.collectionName}</Chip>
                <span class="data text-muted">chunk {meta().chunkIndex + 1}</span>
                <Show when={tags().length > 0}>
                  <span class="data text-muted">{tags().map((t) => `#${t}`).join(" ")}</span>
                </Show>
              </div>
            </div>
            <div class="scroll-quiet min-h-0 flex-1 overflow-y-auto p-5">
              <Show
                when={props.doc}
                fallback={
                  <Show when={props.loading}>
                    <div class="space-y-2">
                      <Skeleton class="h-4 w-full" />
                      <Skeleton class="h-4 w-5/6" />
                      <Skeleton class="h-4 w-3/4" />
                    </div>
                  </Show>
                }
              >
                {(doc) => (
                  <p class="read whitespace-pre-wrap text-[15px] leading-[1.7] text-ink-soft">
                    {doc().content}
                  </p>
                )}
              </Show>
            </div>
          </>
        )}
      </Show>
    </div>
  );
}
