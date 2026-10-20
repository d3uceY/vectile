import { useAppStore } from "../../lib/store";
import { fmtBytes } from "../../lib/format";
import { lastIndexedLabel } from "../../lib/time";
import { HOME_URL, openExternal } from "../../lib/update";
import { McpIcon } from "../ui/icons";
import { Kbd } from "../ui/primitives";

/** Top strip: library status on the left; controls and app version on the right. */
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
  const mcpRunning = () => store.mcpStatus()?.running === true;
  const mcpHint = () =>
    mcpRunning() ? "MCP server running. Open settings" : "Connect an MCP client";

  return (
    <header class="flex h-11 shrink-0 items-center justify-between gap-4 border-b border-line bg-paper px-4">
      {/* Status: ambient, one quiet sentence. */}
      <span class="min-w-0 truncate text-[12px] font-medium text-muted">
        {totals().collections} collections · {totals().chunks.toLocaleString()} chunks
        {totals().size ? ` · ${totals().size}` : ""}
        {lastIndexedLabel(st()?.lastIndexed ?? "")}
      </span>

      <div class="flex shrink-0 items-center gap-4">
        {/* Controls: MCP is a destination, search the primary action. */}
        <div class="flex items-center gap-2">
          <button
            type="button"
            class="flex items-center gap-1 rounded-full px-2 py-1 text-[12px] font-semibold text-muted transition-colors duration-100 hover:bg-surface-2 hover:text-ink"
            title={mcpHint()}
            onClick={() => store.openSettings("connect")}
          >
            <McpIcon size={16} class={mcpRunning() ? "text-leaf-deep" : undefined} />
            <span>MCP</span>
          </button>
          <button
            class="flex items-center gap-2 rounded-full px-2 py-1 text-[12px] font-medium text-ink-soft transition-colors duration-100 hover:bg-surface-2 hover:text-ink"
            onClick={onSearch}
          >
            <span class="hidden md:inline">Jump to search</span>
            <Kbd>{navigator.platform.toLowerCase().includes("mac") ? "⌘K" : "Ctrl K"}</Kbd>
          </button>
        </div>

        {/* Meta: least important, furthest from the content. */}
        {props.version && (
          <button
            class="hidden shrink-0 cursor-pointer text-[12px] font-medium text-muted transition-colors duration-100 hover:text-ink md:inline"
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
