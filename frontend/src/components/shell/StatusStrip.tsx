import { useAppStore } from "../../lib/store";
import { fmtBytes } from "../../lib/format";
import { lastIndexedLabel } from "../../lib/time";
import { HOME_URL, openExternal } from "../../lib/update";
import { Kbd } from "../ui/primitives";

/** Top strip: the model-engine state on the left, library summary on the
    right, and the app version on the far right (click → the GitHub repo). */
export function StatusStrip(props: { version?: string }) {
  const store = useAppStore();
  const st = () => store.status();
  const totals = () => {
    const s = st();
    if (s) {
      return { collections: s.collections, chunks: s.chunks, size: fmtBytes(s.dbSize) };
    }
    const cols = store.collections();
    return {
      collections: cols.length,
      chunks: cols.reduce((n, c) => n + c.chunks, 0),
      size: "",
    };
  };
  const onSearch = () => store.view() !== "search" && store.focusSearch();

  return (
    <header class="flex h-11 shrink-0 items-center justify-between gap-3 border-b border-line bg-paper px-3.5">
      <div class="flex min-w-0 items-center gap-4">
        <span class="h-3 w-px shrink-0 bg-line-strong" aria-hidden="true" />
        <span class="truncate text-[12px] font-medium text-muted">
          {totals().collections} collections · {totals().chunks.toLocaleString()} chunks
          {totals().size ? ` · ${totals().size}` : ""}
          {lastIndexedLabel(st()?.lastIndexed ?? "")}
        </span>
      </div>
      <div class="flex shrink-0 items-center gap-2">
        <button
          class="flex items-center gap-2 rounded-full px-2.5 py-1 text-[12px] font-semibold text-muted transition-colors duration-100 hover:bg-surface-2 hover:text-ink"
          onClick={onSearch}
        >
          <span class="hidden md:inline">Jump to search</span>
          <Kbd>{navigator.platform.toLowerCase().includes("mac") ? "⌘K" : "Ctrl K"}</Kbd>
        </button>
        {props.version && (
          <button
            class="hidden shrink-0 cursor-pointer text-[12px] font-medium text-muted transition-colors duration-100 hover:text-leaf-deep md:inline"
            title="vectile on GitHub"
            onClick={() => openExternal(HOME_URL)}
          >
            {props.version}
          </button>
        )}
      </div>
    </header>
  );
}
