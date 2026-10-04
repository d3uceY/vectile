import { createSignal, onMount, Show } from "solid-js";
import { useAppStore } from "../../../lib/store";
import { Button, ConfirmDialog } from "../../ui/primitives";
import { CacheNavIcon } from "../../ui/nav-icons";
import { fmtBytes } from "../../../lib/format";
import { Section } from "../fields";

export function CacheSection() {
  const store = useAppStore();

  onMount(() => void store.loadCacheStats());

  const cacheCount = () => store.cacheStats()?.entries ?? 0;
  const cacheBytes = () => store.cacheStats()?.bytes ?? 0;
  const cacheLoaded = () => store.cacheStats() !== null;
  const [confirmClearCache, setConfirmClearCache] = createSignal(false);
  const [clearingCache, setClearingCache] = createSignal(false);

  const clearTheCache = async () => {
    setClearingCache(true);
    try {
      await store.clearCache();
    } finally {
      setClearingCache(false);
      setConfirmClearCache(false);
    }
  };

  return (
    <Section
      icon={<CacheNavIcon size={16} />}
      title="Cache"
      note="Repeated searches reuse the query embedding instead of computing it again."
    >
      <div class="space-y-6">
        <div class="rounded-[8px] border border-line bg-surface px-4 py-3.5">
          <div class="flex items-center gap-2">
            <span
              class={`h-2 w-2 shrink-0 rounded-full ${cacheCount() > 0 ? "bg-leaf" : "bg-ghost"}`}
            />
            <span class="text-[12px] font-semibold leading-none text-ink-soft">
              {cacheCount() === 0
                ? cacheLoaded()
                  ? "nothing cached yet"
                  : "checking…"
                : `${cacheCount().toLocaleString()} quer${cacheCount() === 1 ? "y" : "ies"} cached`}
            </span>
            <Show when={cacheBytes() > 0}>
              <span class="ml-auto shrink-0 font-mono text-[11px] text-muted">
                {fmtBytes(cacheBytes())}
              </span>
            </Show>
          </div>
          <div class="mt-2.5 border-t border-line" aria-hidden="true" />
          <p class="mt-2 text-[11px] font-medium leading-4 text-muted">
            only the query embedding is stored, never your results · cleared when you
            reindex or change the active model
          </p>
        </div>

        <div class="flex flex-wrap items-center justify-between gap-3">
          <p class="max-w-[46ch] text-[12px] font-medium leading-4 text-muted">
            Clearing costs one extra embedding per repeated query.
          </p>
          <Button
            size="sm"
            variant="danger"
            disabled={cacheCount() === 0}
            onClick={() => setConfirmClearCache(true)}
          >
            Clear cache
          </Button>
        </div>

        <ConfirmDialog
          open={confirmClearCache()}
          title="Clear the query cache?"
          body={
            <p>
              Every cached query embedding is dropped. The next search of the same text
              embeds it again. Your library, indexes, and settings are untouched.
            </p>
          }
          confirmLabel="Clear cache"
          busyLabel="Clearing…"
          busy={clearingCache()}
          onCancel={() => setConfirmClearCache(false)}
          onConfirm={() => void clearTheCache()}
        />
      </div>
    </Section>
  );
}
