import { createSignal, For, Show } from "solid-js";
import { useAppStore } from "../../lib/store";
import { Button, Chip, ConfirmDialog, EmptyState, Toggle, ViewHeading } from "../ui/primitives";
import { IndexIcon } from "../ui/icons";
import { IndexProgressBar } from "./IndexProgressBar";

type Configured = { name: string; type: string; enabled: boolean };

export function IndexView() {
  const store = useAppStore();
  const [confirming, setConfirming] = createSignal<string | null>(null);

  const configured = (): Configured[] => {
    const cfg = store.config();
    if (!cfg) return [];
    const disabled = new Set(cfg.disabled_collections);
    const list: Configured[] = [];
    if (cfg.obsidian_vaults.length) list.push({ name: "obsidian", type: "system", enabled: !disabled.has("obsidian") });
    if (cfg.calibre_libraries.length) list.push({ name: "calibre", type: "system", enabled: !disabled.has("calibre") });
    for (const [name, paths] of Object.entries(cfg.projects)) {
      if (paths.length) list.push({ name, type: "project", enabled: !disabled.has(name) });
    }
    for (const [name, paths] of Object.entries(cfg.repositories)) {
      if (paths.length) list.push({ name, type: "code", enabled: !disabled.has(name) });
    }
    return list;
  };

  const dbCol = (name: string) => store.collections().find((c) => c.name === name);
  const progressOf = (name: string) => store.indexByCollection()[name];

  return (
    <div class="relative flex h-full flex-col">
      <ViewHeading
        title="Index"
        note="Add sources in Settings, then index them here. Deleted files are pruned automatically. Index new adds only changed files; Re-index all re-embeds everything."
      >
        <Button id="setup-index-all" onClick={() => store.startIndexAll(false)} disabled={store.indexing()}>
          Index all
        </Button>
        <Button variant="outline" onClick={() => store.startIndexAll(true)} disabled={store.indexing()}>
          Re-index all
        </Button>
      </ViewHeading>

      {/* Last run summary */}
      <Show when={!store.indexing() && store.indexLast()}>
        <div class="mb-5 flex flex-wrap items-center gap-2 rounded-card border border-line bg-surface/40 px-4 py-2.5">
          <span class="data text-muted">last: {store.indexLast()!.collection}</span>
          <Chip tone="mint">{store.indexLast()!.indexed} new</Chip>
          <Show when={store.indexLast()!.skipped > 0}>
            <Chip>{store.indexLast()!.skipped} skipped</Chip>
          </Show>
          <Show when={store.indexLast()!.errors > 0}>
            <Chip tone="neutral">{store.indexLast()!.errors} errors</Chip>
          </Show>
        </div>
      </Show>

      {/* Collection rows */}
      <Show
        when={configured().length > 0}
        fallback={
          <div class="flex flex-1 items-center justify-center">
            <EmptyState
              icon={<IndexIcon size={20} />}
              title="Nothing configured yet"
              note="Add a folder or library under Settings."
            >
              <Button onClick={() => store.setView("settings")}>
                Add sources in Settings
              </Button>
            </EmptyState>
          </div>
        }
      >
        <div class="scroll-quiet -mr-2 flex-1 space-y-3 overflow-y-auto pr-2">
          <For each={configured()}>
            {(item) => {
              const col = () => dbCol(item.name);
              const prog = () => progressOf(item.name);
              return (
                <div class="sheet p-5">
                  <div class="flex flex-wrap items-center gap-x-5 gap-y-3">
                    <div class="min-w-0 flex-1">
                      <div class="flex items-center gap-2">
                        <span class="title truncate text-[15px] tracking-[-0.01em] text-ink">{item.name}</span>
                        <Chip tone={item.type === "code" ? "code" : "neutral"}>{item.type}</Chip>
                        {!item.enabled && <Chip>disabled</Chip>}
                        {col()?.needsReindex && <Chip tone="amber">needs reindex</Chip>}
                      </div>
                      <p class="data mt-1 text-muted">
                        {col() ? `${col()!.sources} sources · ${col()!.chunks.toLocaleString()} chunks` : "not indexed yet"}
                      </p>
                    </div>
                    <div class="flex items-center gap-2">
                      <Button
                        size="sm"
                        variant="primary"
                        disabled={store.indexing() || !item.enabled}
                        onClick={() => store.startIndex(item.name, false)}
                      >
                        Index new
                      </Button>
                      <Show when={col()}>
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={store.indexing() || !item.enabled}
                          onClick={() => store.startIndex(item.name, true)}
                        >
                          Re-index all
                        </Button>
                      </Show>
                      <Button size="sm" variant="ghost" disabled={store.indexing()} onClick={() => store.runPrune(item.name)}>
                        Prune
                      </Button>
                      <Toggle checked={item.enabled} onChange={(v) => store.toggleCollection(item.name, v)} label="Enabled" />
                    </div>
                  </div>

                  {/* Inline loader on the dir being indexed */}
                  <Show when={prog()}>
                    {(p) => (
                      <div class="mt-4">
                        <IndexProgressBar
                          collection={item.name}
                          current={p().indexed}
                          total={p().total}
                          file={p().file}
                          onCancel={() => setConfirming(item.name)}
                        />
                      </div>
                    )}
                  </Show>
                </div>
              );
            }}
          </For>
        </div>
      </Show>

      {/* Cancel-indexing warning */}
      <ConfirmDialog
        open={confirming() !== null}
        title={`Cancel indexing ${confirming()}?`}
        body={<p>Files already indexed are kept. The rest of this run will stop.</p>}
        confirmLabel="Cancel run"
        cancelLabel="Keep going"
        onCancel={() => setConfirming(null)}
        onConfirm={() => {
          void store.cancelIndex();
          setConfirming(null);
        }}
      />
    </div>
  );
}
