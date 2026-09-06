import { createMemo, createSignal, For, Show } from "solid-js";
import { CheckIcon, ChevronDown, ChevronRight, FileIcon, FolderIcon, FolderOpenIcon } from "./icons";

export type TreeViewElement = {
  id: string;
  name: string;
  type?: "file" | "folder";
  isSelectable?: boolean;
  children?: TreeViewElement[];
};

interface Row {
  node: TreeViewElement;
  depth: number;
  hasChildren: boolean;
}

function isFolder(n: TreeViewElement): boolean {
  return n.type === "folder" || (n.children?.length ?? 0) > 0;
}

function sortNodes(nodes: TreeViewElement[]): TreeViewElement[] {
  const cmp = (a: TreeViewElement, b: TreeViewElement) => a.name.localeCompare(b.name);
  const folders = nodes.filter(isFolder).sort(cmp);
  const files = nodes.filter((n) => !isFolder(n)).sort(cmp);
  return [...folders, ...files];
}

export function FileTree(props: {
  elements: TreeViewElement[];
  initialSelectedId?: string;
  initialExpandedItems?: string[];
  onSelect?: (id: string, node: TreeViewElement) => void;
  showExpandAll?: boolean;
  /** Show a checkbox on leaf nodes (for bulk select). Clicking it toggles
      the check without selecting the row. */
  checkable?: boolean;
  isChecked?: (id: string) => boolean;
  onToggleCheck?: (id: string, node: TreeViewElement) => void;
}) {
  const [selectedId, setSelectedId] = createSignal<string>(props.initialSelectedId ?? "");
  const [expanded, setExpanded] = createSignal<Set<string>>(
    new Set(props.initialExpandedItems ?? []),
  );

  const rowById = new Map<string, Row>();

  const visible = createMemo<Row[]>(() => {
    const out: Row[] = [];
    const walk = (nodes: TreeViewElement[], depth: number) => {
      for (const n of sortNodes(nodes)) {
        const hc = isFolder(n);
        const cached = rowById.get(n.id);
        const row =
          cached && cached.node === n && cached.depth === depth
            ? cached
            : { node: n, depth, hasChildren: hc };
        rowById.set(n.id, row);
        out.push(row);
        if (hc && expanded().has(n.id)) walk(n.children ?? [], depth + 1);
      }
    };
    walk(props.elements, 0);
    return out;
  });

  const toggle = (id: string) =>
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });

  const select = (row: Row) => {
    if (row.node.isSelectable === false) return;
    setSelectedId(row.node.id);
    props.onSelect?.(row.node.id, row.node);
  };

  const rows = () => visible();
  let refs: (HTMLButtonElement | undefined)[] = [];

  const focusAt = (i: number) => {
    const r = refs[i];
    if (r) {
      r.focus();
      const row = rows()[i];
      if (row) setSelectedId(row.node.id);
    }
  };

  const onKeyDown = (e: KeyboardEvent, i: number) => {
    const row = rows()[i];
    if (!row) return;
    if (e.key === "ArrowDown") {
      e.preventDefault();
      focusAt(Math.min(i + 1, rows().length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      focusAt(Math.max(i - 1, 0));
    } else if (e.key === "ArrowRight") {
      e.preventDefault();
      if (row.hasChildren) {
        if (!expanded().has(row.node.id)) toggle(row.node.id);
        else focusAt(i + 1);
      }
    } else if (e.key === "ArrowLeft") {
      e.preventDefault();
      if (row.hasChildren && expanded().has(row.node.id)) toggle(row.node.id);
    } else if (e.key === "Enter") {
      e.preventDefault();
      if (row.hasChildren) toggle(row.node.id);
      else select(row);
    }
  };

  const allExpanded = () => visible().length === flattenedCount(props.elements);
  const flattenedCount = (nodes: TreeViewElement[]): number => {
    let n = 0;
    const w = (list: TreeViewElement[]) => {
      for (const node of sortNodes(list)) {
        n++;
        if (isFolder(node) && node.children) w(node.children);
      }
    };
    w(nodes);
    return n;
  };

  const toggleAll = () =>
    setExpanded((prev) => {
      if (allExpanded()) return new Set<string>();
      const all = new Set<string>();
      const w = (nodes: TreeViewElement[]) => {
        for (const node of sortNodes(nodes)) {
          if (isFolder(node)) {
            all.add(node.id);
            if (node.children) w(node.children);
          }
        }
      };
      w(props.elements);
      return all;
    });

  return (
    <div>
      <Show when={props.showExpandAll}>
        <button
          class="mb-1 flex items-center gap-1.5 px-1 text-[12px] text-faint transition-colors hover:text-indigo"
          onClick={toggleAll}
        >
          <ChevronDown size={12} class={allExpanded() ? "rotate-180 transition-transform" : "transition-transform"} />
          {allExpanded() ? "Collapse all" : "Expand all"}
        </button>
      </Show>
      <div role="tree" aria-label="Indexed files">
        <For each={rows()}>
          {(row, i) => (
            <button
              ref={(el) => (refs[i()] = el)}
              role="treeitem"
              aria-selected={selectedId() === row.node.id}
              aria-expanded={row.hasChildren ? expanded().has(row.node.id) : undefined}
              aria-level={row.depth + 1}
              onClick={() => (row.hasChildren ? toggle(row.node.id) : select(row))}
              onKeyDown={(e) => onKeyDown(e, i())}
              class={`group relative flex w-full items-center gap-1.5 rounded-lg px-1.5 py-0.75 text-left text-[13px] transition-colors duration-100 ease-snappy ${
                selectedId() === row.node.id
                  ? "bg-indigo-mist text-ink"
                  : "text-ink-soft hover:bg-surface"
              } ${row.node.isSelectable === false ? "cursor-default" : "cursor-pointer"}`}
              style={{ "padding-inline-start": `${8 + row.depth * 16}px` }}
            >
              <Show when={row.depth > 0}>
                <span
                  aria-hidden="true"
                  class="pointer-events-none absolute inset-y-0 right-7! w-px bg-line-strong"
                  style={{ left: `${8 + row.depth * 13}px` }}
                />
              </Show>
              <Show when={props.checkable && !row.hasChildren}>
                <span
                  role="checkbox"
                  aria-checked={props.isChecked?.(row.node.id) ?? false}
                  tabIndex={-1}
                  onClick={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    props.onToggleCheck?.(row.node.id, row.node);
                  }}
                  class={`flex h-4 w-4 shrink-0 items-center justify-center rounded-sm border transition-colors duration-100 ease-snappy ${
                    props.isChecked?.(row.node.id)
                      ? "border-indigo bg-indigo text-white"
                      : "border-line-strong bg-paper text-transparent hover:border-indigo/60"
                  }`}
                >
                  <CheckIcon size={11} strokeWidth={2.5} />
                </span>
              </Show>
              <span
                class={`flex h-4 w-4 shrink-0 items-center justify-center text-faint transition-transform duration-150 ease-snappy ${
                  row.hasChildren ? (expanded().has(row.node.id) ? "rotate-90" : "") : "opacity-0"
                }`}
              >
                <ChevronRight size={12} />
              </span>
              <span class="flex h-4 w-4 shrink-0 items-center justify-center">
                {row.hasChildren
                  ? expanded().has(row.node.id)
                    ? <FolderOpenIcon size={15} class="text-indigo" />
                    : <FolderIcon size={15} class="text-faint" />
                  : <FileIcon size={14} class="text-faint" />}
              </span>
              <span class="truncate">{row.node.name}</span>
            </button>
          )}
        </For>
      </div>
    </div>
  );
}
