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
    <header class="flex h-13 shrink-0 items-center justify-between gap-3 border-b border-line bg-paper/70 px-6">
      <div class="flex min-w-0 items-center gap-4">
        <span class="data shrink-0 text-muted">all local</span>
        <span class="h-3 w-px shrink-0 bg-line-strong" aria-hidden="true" />
        <span class="data truncate text-muted">
          {totals().collections} collections · {totals().chunks.toLocaleString()} chunks
          {totals().size ? ` · ${totals().size}` : ""}
          {lastIndexedLabel(st()?.lastIndexed ?? "")}
        </span>
      </div>
      <div class="flex shrink-0 items-center gap-3">
        <button
          class="flex items-center gap-2 rounded-control px-2 py-1 text-[12.5px] text-muted transition-colors hover:bg-surface hover:text-ink"
          onClick={onSearch}
        >
          <span class="hidden md:inline">Jump to search</span>
          <Kbd>{navigator.platform.toLowerCase().includes("mac") ? "⌘K" : "Ctrl K"}</Kbd>
        </button>
        {props.version && (
          <button
            class="data hidden shrink-0 cursor-pointer text-muted transition-colors hover:text-indigo-deep md:inline"
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
