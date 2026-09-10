import { createEffect, createSignal, For, Show } from "solid-js";
import * as api from "../../lib/api";
import { createPager, withScrollAnchor } from "../../lib/pager";
import { useAppStore } from "../../lib/store";
import type { Source } from "../../lib/types";
import { ChevronDown, LibraryIcon, TrashIcon } from "../ui/icons";
import { ScrollSentinel } from "../ui/ScrollSentinel";
import { Button, Chip, ConfirmDialog, EmptyState, ViewHeading } from "../ui/primitives";

/* Pages of sources kept in memory while a collection is expanded. A calibre
   group can hold thousands of them, and only the expanded one is ever loaded. */
const MAX_PAGES = 5;

const typeLabel: Record<string, string> = {
  system: "system",
  project: "project",
  code: "code",
};

function TypeBadge(props: { type: string }) {
  const label = typeLabel[props.type] ?? props.type;
  return props.type === "code" ? (
    <Chip tone="code">{label}</Chip>
  ) : props.type === "project" ? (
    <Chip tone="mint">{label}</Chip>
  ) : (
    <Chip>{label}</Chip>
  );
}

export function LibraryView() {
  const store = useAppStore();
  const open = () => store.expandedCollection();
  const openId = () => {
    const o = open();
    return o === null ? null : Number(o);
  };

  let scroller: HTMLDivElement | undefined;
  const getScroller = () => scroller;

  // The page itself scrolls, so the expanded collection's sources page against
  // that same container, with the same window and the same re-anchoring.
  const pager = createPager<Source>({
    maxPages: MAX_PAGES,
    fetch: async (cursor, backward) => {
      const id = openId();
      if (id === null) return { items: [], before: "", after: "" };
      const page = await api.listSourcesPage(id, cursor, backward);
      return { items: page.sources, before: page.before, after: page.after };
    },
    onShift: (mutate) => withScrollAnchor(getScroller, mutate),
  });

  // Expanding a collection starts a fresh window; collapsing drops it, which is
  // also what keeps a big collection from staying in memory.
  createEffect(() => {
    const id = openId();
    store.libraryEpoch();
    pager.reset();
    if (id !== null) pager.loadNext();
  });

  const toggle = (id: number) =>
    store.setExpandedCollection(open() === String(id) ? null : String(id));

  const [confirm, setConfirm] = createSignal<
    | { kind: "collection"; id: number; name: string; type: string; chunks: number }
    | { kind: "source"; id: number; name: string; path: string; chunks: number }
    | null
  >(null);
  const [deleting, setDeleting] = createSignal(false);

  const doDelete = async () => {
    const t = confirm();
    if (!t) return;
    setDeleting(true);
    const ok =
      t.kind === "collection"
        ? await store.deleteCollection(t.name, t.name)
        : await store.deleteSource(t.id, t.path.split(/[\\/]/).pop() || t.path);
    setDeleting(false);
    if (ok) setConfirm(null);
  };

  return (
    <div class="relative flex h-full flex-col">
      <div class="relative">
        <ViewHeading title="Library" note="What's indexed." />
      </div>

      <div ref={scroller} class="scroll-quiet relative min-h-0 flex-1 overflow-y-auto">
        <Show
          when={store.collections().length > 0}
          fallback={
            <div class="flex h-full items-center justify-center">
              <EmptyState
                icon={<LibraryIcon size={20} />}
                title="No collections yet"
                note="Index a folder or library to see it here."
              >
                <Button onClick={() => store.setView("settings")}>Add sources in Settings</Button>
              </EmptyState>
            </div>
          }
        >
          <div class="sheet overflow-hidden">
            <div class="flex items-stretch border-b border-line bg-paper/60">
              <div class="grid flex-1 grid-cols-12 gap-2 px-5 py-2.5 text-[11px] font-medium uppercase tracking-[0.08em] text-muted">
                <span class="col-span-6">Collection</span>
                <span class="col-span-2">Files</span>
                <span class="col-span-2 text-right">Chunks</span>
                <span class="col-span-2 text-right">Indexed</span>
              </div>
              <div class="w-[43px] shrink-0" aria-hidden="true" />
            </div>
            <ul class="divide-y divide-line">
              <For each={store.collections()}>
                {(c) => {
                  const isOpen = () => open() === String(c.id);
                  return (
                    <li class="group">
                      <div class="flex items-stretch">
                        <button
                          class={`grid flex-1 grid-cols-12 items-center gap-2 px-5 py-3.5 text-left transition-colors duration-100 ease-snappy ${
                            isOpen() ? "bg-indigo-mist/40" : "hover:bg-surface-2"
                          }`}
                          onClick={() => toggle(c.id)}
                          aria-expanded={isOpen()}
                        >
                          <span class="col-span-6 flex items-center gap-2.5">
                            <ChevronDown
                              size={14}
                              class={`shrink-0 text-faint transition-transform duration-150 ease-snappy ${
                                isOpen() ? "rotate-0" : "-rotate-90"
                              }`}
                            />
                            <span class="flex min-w-0 items-center gap-2">
                              <span class="title truncate text-[14px] text-ink">{c.name}</span>
                              <TypeBadge type={c.type} />
                              {c.needsReindex && (
                                <span
                                  class="shrink-0 rounded-control bg-highlighter/60 px-1.5 py-0.5 text-[11px] font-medium text-ink"
                                  title="No embeddings yet. Re-index after switching to a model with a different embedding dimension."
                                >
                                  needs reindex
                                </span>
                              )}
                            </span>
                          </span>
                          <span class="data col-span-2 text-muted">{c.sources}</span>
                          <span class="data col-span-2 text-right text-muted">
                            {c.chunks.toLocaleString()}
                          </span>
                          <span class="data col-span-2 text-right text-muted">
                            {c.lastIndexed ? new Date(c.lastIndexed).toLocaleDateString() : "never"}
                          </span>
                        </button>
                        <button
                          type="button"
                          class="flex shrink-0 items-center px-3.5 text-faint opacity-0 transition-opacity duration-100 ease-snappy hover:text-danger focus-visible:opacity-100 group-hover:opacity-100"
                          onClick={() =>
                            setConfirm({
                              kind: "collection",
                              id: c.id,
                              name: c.name,
                              type: c.type,
                              chunks: c.chunks,
                            })
                          }
                          aria-label={`Delete collection ${c.name}`}
                          title={`Delete ${c.name} from the index and Settings`}
                        >
                          <TrashIcon size={15} />
                        </button>
                      </div>

                      <Show when={isOpen()}>
                        <div class="border-t border-line bg-paper/70">
                          <Show when={c.description}>
                            <p class="data px-5 pt-3 pb-1 text-muted">{c.description}</p>
                          </Show>
                          <ScrollSentinel
                            root={getScroller}
                            onVisible={pager.loadPrev}
                            stop={pager.stopPrev}
                          />
                          <Show when={pager.items().length > 0}>
                            <ul class="pb-2">
                              <For each={pager.items()}>
                                {(s) => (
                                  <li
                                    data-row-id={s.id}
                                    class="group flex items-stretch rounded-lg hover:bg-surface"
                                  >
                                    <div class="grid min-w-0 flex-1 grid-cols-12 items-center gap-2 px-5 py-1.5">
                                      <span class="col-span-6 flex min-w-0 items-center gap-2 pl-6">
                                        <span class="data min-w-0 flex-1 truncate text-ink-soft">
                                          {s.path}
                                        </span>
                                      </span>
                                      <span class="data col-span-2 truncate text-muted">
                                        {s.sourceType}
                                      </span>
                                      <span class="data col-span-2 text-right text-muted">
                                        {s.chunks.toLocaleString()}
                                      </span>
                                      <span class="data col-span-2 text-right text-muted">
                                        {s.lastIndexed
                                          ? new Date(s.lastIndexed).toLocaleDateString()
                                          : "never"}
                                      </span>
                                    </div>
                                    <button
                                      type="button"
                                      class="flex shrink-0 items-center px-3.5 text-faint opacity-0 transition-opacity duration-100 ease-snappy hover:text-danger focus-visible:opacity-100 group-hover:opacity-100"
                                      onClick={() =>
                                        setConfirm({
                                          kind: "source",
                                          id: s.id,
                                          name: s.path.split(/[\\/]/).pop() || s.path,
                                          path: s.path,
                                          chunks: s.chunks,
                                        })
                                      }
                                      aria-label={`Delete source ${s.path}`}
                                      title={`Remove ${s.path} from the index`}
                                    >
                                      <TrashIcon size={15} />
                                    </button>
                                  </li>
                                )}
                              </For>
                            </ul>
                          </Show>
                          <Show when={pager.items().length === 0 && pager.loaded()}>
                            <div class="px-5 py-3">
                              <p class="data text-muted">No sources indexed yet.</p>
                            </div>
                          </Show>
                          <Show when={pager.loadingBottom()}>
                            <p class="data px-5 py-2 text-muted">Loading more sources</p>
                          </Show>
                          <Show when={pager.failedNext()}>
                            <p class="data px-5 py-2 text-muted">
                              Couldn't load more sources.{" "}
                              <button class="text-indigo hover:underline" onClick={pager.loadNext}>
                                Retry
                              </button>
                            </p>
                          </Show>
                          <ScrollSentinel
                            root={getScroller}
                            onVisible={pager.loadNext}
                            stop={pager.stopNext}
                          />
                        </div>
                      </Show>
                    </li>
                  );
                }}
              </For>
            </ul>
          </div>

          <div class="mt-4 flex items-center gap-2 text-[12.5px] text-muted">
            <LibraryIcon size={14} class="text-faint" />
            <span>Deleted files are pruned automatically.</span>
          </div>
        </Show>
      </div>

      <ConfirmDialog
        open={confirm() !== null}
        title={confirm()?.kind === "collection" ? `Delete ${confirm()!.name}?` : "Remove this file from the index?"}
        busy={deleting()}
        onCancel={() => setConfirm(null)}
        onConfirm={() => void doDelete()}
        body={(() => {
          const t = confirm();
          if (t?.kind === "collection") {
            return (
              <p>
                Removes <span class="font-medium text-ink">“{t.name}”</span> and its{" "}
                <span class="font-medium text-ink">{t.chunks.toLocaleString()} chunks</span> from
                the index and from Settings
                {t.type === "system" ? " (all its vault/library paths)." : " (the whole group)."}{" "}
                Files on disk are untouched. Re-add the source in Settings to index it again.
              </p>
            );
          }
          return (
            <p>
              Removes <span class="font-medium text-ink">“{t?.name}”</span> and its{" "}
              <span class="font-medium text-ink">{t?.chunks.toLocaleString()} chunks</span> from the
              index. The file stays on disk and the path stays configured, so a re-index can bring
              it back.
            </p>
          );
        })()}
      />
    </div>
  );
}
