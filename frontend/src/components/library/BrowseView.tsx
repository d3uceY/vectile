import { createEffect, createMemo, createSignal, For, onCleanup, onMount, Show } from "solid-js";
import { useAppStore } from "../../lib/store";
import { FileTree, type TreeViewElement } from "../ui/FileTree";
import {
  ViewHeading,
  Button,
  Chip,
  ConfirmDialog,
  EmptyState,
  Select,
  Skeleton,
} from "../ui/primitives";
import { BrowseIcon, TrashIcon } from "../ui/icons";

const cid = (n: number) => `c-${n}`; // collection folder
const sid = (n: number) => `s-${n}`; // source folder
const did = (n: number) => `d-${n}`; // document (file)

export function BrowseView() {
  const store = useAppStore();
  const [loaded, setLoaded] = createSignal(false);

  const [colId, setColId] = createSignal<number | null>(null);
  const [checked, setChecked] = createSignal<Set<number>>(new Set());

  const collection = createMemo(
    () => store.collections().find((c) => c.id === colId()) ?? store.collections()[0] ?? null,
  );

  const libraryDocs = createMemo(() => {
    const c = collection();
    return c ? store.documents().filter((d) => d.collectionId === c.id) : [];
  });

  const tree = createMemo<TreeViewElement[]>(() => {
    const c = collection();
    if (!c) return [];
    return [
      {
        id: cid(c.id),
        name: c.name,
        type: "folder" as const,
        children: store
          .sources()
          .filter((s) => s.collectionId === c.id)
          .map((s) => ({
            id: sid(s.id),
            name: s.path.split(/[\\/]/).pop() || s.path,
            type: "folder" as const,
            children: libraryDocs()
              .filter((d) => d.sourceId === s.id)
              .map((d) => ({ id: did(d.id), name: d.title, type: "file" as const })),
          })),
      },
    ];
  });

  const initialExpanded = createMemo(() => {
    const out: string[] = [];
    const c = collection();
    if (c) out.push(cid(c.id));
    const firstSource = store.sources().find((s) => s.collectionId === c?.id);
    if (firstSource) out.push(sid(firstSource.id));
    return out;
  });

  const doc = () => {
    const sel = store.selectedDoc();
    if (sel && sel.collectionId === collection()?.id) return sel;
    return libraryDocs()[0] ?? null;
  };

  const onSelect = (tid: string) => {
    const d = libraryDocs().find((x) => did(x.id) === tid);
    if (d) store.setSelectedDoc(d);
  };

  // --- bulk chunk selection --------------------------------------------
  const nChecked = () => checked().size;

  const toggleCheck = (tid: string, node: TreeViewElement) => {
    const n = Number(tid.slice(2));
    if (!Number.isFinite(n)) return;
    setChecked((prev) => {
      const next = new Set(prev);
      if (next.has(n)) next.delete(n);
      else next.add(n);
      return next;
    });
  };

  const isChecked = (tid: string) => checked().has(Number(tid.slice(2)));

  const selectAll = () =>
    setChecked((prev) => {
      const next = new Set(prev);
      for (const d of libraryDocs()) next.add(d.id);
      return next;
    });

  const clearChecked = () => setChecked(new Set<number>());

  createEffect(() => {
    const live = new Set(libraryDocs().map((d) => d.id));
    setChecked((prev) => {
      let changed = false;
      const next = new Set<number>();
      for (const id of prev) {
        if (live.has(id)) next.add(id);
        else changed = true;
      }
      return changed ? next : prev;
    });
  });

  createEffect(() => {
    if (colId() === null) {
      const first = store
        .collections()
        .find((c) => store.documents().some((d) => d.collectionId === c.id));
      if (first) setColId(first.id);
    }
  });

  const onLibraryChange = (value: string) => {
    const id = Number(value);
    setColId(Number.isFinite(id) && id > 0 ? id : null);
    setChecked(new Set<number>());
    store.setSelectedDoc(null);
  };

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
    setChecked(new Set<number>());
    if (t.kind === "library") {
      const next = store.collections().find((c) => c.name !== t.name);
      setColId(next ? next.id : null);
      store.setSelectedDoc(null);
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
        Removes all {t.chunks.toLocaleString()} chunks in {t.name} and its sources, and removes
        it from Settings. Files on disk are untouched.
      </span>
    ) : (
      <span>
        The selected chunks leave the index: their embeddings and search entries go with them.
        Sources and files on disk stay.
      </span>
    );
  };

  onMount(() => {
    void store.loadLibrary().then(() => setLoaded(true));
  });
  onCleanup(() => setLoaded(false));

  return (
    <div class="relative flex h-full flex-col">
      <div class="relative">
        <ViewHeading title="Browse" note="Read the chunks inside your files." />
      </div>

      <div class="relative @container min-h-0 flex-1">
        <Show
          when={loaded() && store.documents().length > 0}
          fallback={
            <div class="flex h-full items-center justify-center">
              <Show when={loaded()} fallback={<BrowseLoading />}>
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
              </Show>
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
          <Chip tone="mint">{libraryDocs().length.toLocaleString()} chunks</Chip>
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

        <div class="flex h-full min-h-0 flex-col gap-4 @lg:grid @lg:grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)]">
          <div class="flex min-h-0 flex-1 flex-col gap-2">
            <Show when={libraryDocs().length > 0}>
              <div class="flex items-center gap-2 rounded-lg bg-indigo-soft/70 px-2.5 py-1.5">
                <span class="data text-[12px] font-medium text-ink">
                  {nChecked() > 0 ? `${nChecked()} selected` : "Select chunks to delete"}
                </span>
                <button
                  type="button"
                  class="data text-[12px] text-indigo hover:underline"
                  onClick={selectAll}
                >
                  Select all
                </button>
                <Show when={nChecked() > 0}>
                  <button
                    type="button"
                    class="data text-[12px] text-indigo hover:underline"
                    onClick={clearChecked}
                  >
                    Clear
                  </button>
                </Show>
                <div class="flex-1" />
                <button
                  type="button"
                  disabled={nChecked() === 0}
                  class="inline-flex items-center gap-1.5 rounded-control bg-danger px-2.5 py-1 text-[12px] font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-40 disabled:pointer-events-none"
                  onClick={() => setConfirm({ kind: "chunks", count: nChecked() })}
                >
                  <TrashIcon size={12} /> Delete
                </button>
              </div>
            </Show>
            <div class="sheet scroll-quiet min-h-0 flex-1 overflow-y-auto p-3">
              <Show
                when={libraryDocs().length > 0}
                fallback={
                  <div class="flex h-full items-center justify-center">
                    <p class="note max-w-[16rem] text-center text-[14px] leading-5 text-muted">
                      No chunks yet. Index it or pick another library.
                    </p>
                  </div>
                }
              >
                {/* Keyed by library id so the tree's internal expand/selection
                    state resets when you switch libraries. */}
                <For each={[{ id: collection()?.id ?? -1 }]}>
                  {() => (
                    <FileTree
                      elements={tree()}
                      initialSelectedId={doc() ? did(doc()!.id) : undefined}
                      initialExpandedItems={initialExpanded()}
                      onSelect={onSelect}
                      showExpandAll
                      checkable
                      isChecked={isChecked}
                      onToggleCheck={toggleCheck}
                    />
                  )}
                </For>
              </Show>
            </div>
          </div>

          <Show when={doc()} fallback={<div class="sheet min-h-0 flex-1" />}>
            {(d) => {
              const src = () => store.sources().find((s) => s.id === d().sourceId);
              const col = () => store.collections().find((c) => c.id === d().collectionId);
              const tags = () => {
                const t = (d().metadata as Record<string, unknown> | null)?.tags;
                return Array.isArray(t) ? (t as string[]) : [];
              };
              return (
                <div class="sheet flex min-h-0 flex-1 flex-col overflow-hidden">
                  <div class="border-b border-line bg-paper/60 px-5 py-4">
                    <h3 class="title text-[16px] leading-6 tracking-[-0.005em] text-ink">
                      {d().title}
                    </h3>
                    <p class="data mt-1.5 truncate text-muted">{src()?.path ?? ""}</p>
                    <div class="mt-3 flex flex-wrap items-center gap-1.5">
                      <Chip tone="mint">{col()?.name ?? String(d().collectionId)}</Chip>
                      <span class="data text-muted">chunk {d().chunkIndex + 1}</span>
                      {tags().length > 0 && (
                        <span class="data text-muted">{tags().map((t) => `#${t}`).join(" ")}</span>
                      )}
                    </div>
                  </div>
                  <div class="scroll-quiet min-h-0 flex-1 overflow-y-auto p-5">
                    <p class="read whitespace-pre-wrap text-[15px] leading-[1.7] text-ink-soft">
                      {d().content}
                    </p>
                  </div>
                </div>
              );
            }}
          </Show>
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

function BrowseLoading() {
  return (
    <div class="sheet h-full w-full max-w-[20rem] p-4">
      <Skeleton class="mb-3 h-6 w-2/3" />
      <Skeleton class="mb-2 h-4 w-full" />
      <Skeleton class="mb-2 h-4 w-5/6" />
      <Skeleton class="mb-2 h-4 w-3/4" />
      <Skeleton class="h-4 w-2/3" />
    </div>
  );
}
