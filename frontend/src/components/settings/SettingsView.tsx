import { createEffect, createSignal, createUniqueId, For, Show, type JSX } from "solid-js";
import { useAppStore } from "../../lib/store";
import { importModel, pickFolder, pickModelFile } from "../../lib/api";
import type { AppConfig, GUIConfig, MascotConfig, MCPConfig, ModelInfo, SearchDefaults } from "../../lib/types";
import { Button, ConfirmDialog, InfoTip, Select, StatusPill, Switch, Toggle, ViewHeading } from "../ui/primitives";
import { CheckIcon, CloseIcon, CodeIcon, CopyIcon, FileIcon, FolderOpenIcon, LibraryIcon, PlugIcon, SlashIcon } from "../ui/icons";
import { CatalogModelCard } from "../ui/CatalogModelCard";
import { MASCOT_ASSETS, MASCOT_STATIC } from "../shell/mascot/assets";
import { openExternal } from "../../lib/update";

/* ---- hard bounds for numeric settings ----
   Every numeric field is clamped to these ranges on load and on change, and
   the <input>s carry min/max/step so spinners and native validation agree.
   The bounds reflect what the backend can actually use:

   - embedding_batch_size  : <1 would mean "no batching"; the indexer treats
     anything below 1 as 1 anyway.
   - chunk_size_tokens     : must be >= 1 or the window splitter never
     advances (infinite loop); above ~1500 words a chunk overflows the
     model's 2048-token context (backend/embeddings) and gets truncated.
   - chunk_overlap_tokens  : must stay strictly below chunk_size or the window
     splitter walks backwards into a negative slice bound (panic). 0 = none.
   - git_history_in_months : whole months; <1 runs `git log --since=0 months
     ago`, which is meaningless.
   - top_k / rrf_k         : RRF's denominator is (k + rank + 1) so it must
     stay positive; the vector candidate pool saturates around 200.
   - vector/fts weight     : blend weights live in [0, 1].
   - auto_reindex_interval : the backend clamps <1min to 1min already; 0
     would mean "re-index every tick". */
const clamp = (n: number, min: number, max: number) => Math.min(Math.max(n, min), max);

type NumBounds = { min: number; max: number; step: number };

const STATIC_BOUNDS: Record<
  | "embedding_batch_size"
  | "chunk_size_tokens"
  | "chunk_overlap_tokens"
  | "git_history_in_months"
  | "top_k"
  | "rrf_k"
  | "vector_weight"
  | "fts_weight"
  | "auto_reindex_interval_minutes"
  | "mcp_port",
  NumBounds
> = {
  embedding_batch_size: { min: 1, max: 512, step: 1 },
  chunk_size_tokens: { min: 50, max: 1500, step: 10 },
  chunk_overlap_tokens: { min: 0, max: 0, step: 5 }, // max is dynamic: chunk_size - 1
  git_history_in_months: { min: 1, max: 240, step: 1 },
  top_k: { min: 1, max: 200, step: 1 },
  rrf_k: { min: 1, max: 200, step: 1 },
  vector_weight: { min: 0, max: 1, step: 0.05 },
  fts_weight: { min: 0, max: 1, step: 0.05 },
  auto_reindex_interval_minutes: { min: 1, max: 10080, step: 1 },
  mcp_port: { min: 1024, max: 65535, step: 1 },
};

/** Effective bounds for a key; chunk overlap's max tracks the current chunk size. */
function boundsFor(key: keyof typeof STATIC_BOUNDS, chunkSize: number): NumBounds {
  const b = STATIC_BOUNDS[key];
  return key === "chunk_overlap_tokens"
    ? { ...b, max: Math.max(b.min, chunkSize - 1) }
    : b;
}

/* ---- small building blocks ---- */

function Section(props: { title: string; note?: string; children: JSX.Element }) {
  return (
    <section class="sheet mb-5 p-5">
      <h2 class="title mb-1 text-[16px] tracking-[-0.01em] text-ink">{props.title}</h2>
      {props.note && <p class="note mb-4 text-[13.5px] leading-5 text-muted">{props.note}</p>}
      <div class="space-y-4">{props.children}</div>
    </section>
  );
}

function NumField(props: {
  label: string;
  value: number;
  onChange: (n: number) => void;
  hint?: string;
  min?: number;
  max?: number;
  step?: number;
}) {
  const uid = createUniqueId();
  return (
    <div class="flex items-center justify-between gap-4">
      <span class="flex items-center gap-1.5 text-[13.5px] text-ink-soft">
        <label for={uid} class="cursor-pointer">
          {props.label}
        </label>
        {props.hint && <InfoTip text={props.hint} />}
      </span>
      <input
        id={uid}
        type="number"
        value={props.value}
        min={props.min}
        max={props.max}
        step={props.step}
        onInput={(e) => props.onChange(Number(e.currentTarget.value))}
        class="h-8 w-24 rounded-control border border-line bg-paper px-2 text-right text-[13px] outline-none focus:border-leaf"
      />
    </div>
  );
}

/* Decimal blend controls (e.g. the search weights) are a draggable progress
   bar from 0 to 1 instead of a number box. The .slider class paints the
   filled portion from the --fill custom property, and the tiny readout keeps
   the exact value visible since a 0.05 step is hard to eyeball. */
function RangeField(props: {
  label: string;
  value: number;
  onChange: (n: number) => void;
  hint?: string;
  min?: number;
  max?: number;
  step?: number;
  /** Render the readout (e.g. "auto" at 0 instead of "0.00"). */
  format?: (n: number) => string;
  /** A short suffix after the readout (e.g. "of 16"). */
  suffix?: string;
}) {
  const uid = createUniqueId();
  const min = props.min ?? 0;
  const max = props.max ?? 1;
  const pct = () => ((props.value - min) / (max - min)) * 100;
  return (
    <div class="flex items-center justify-between gap-4">
      <span class="flex items-center gap-1.5 text-[13.5px] text-ink-soft">
        <label for={uid} class="cursor-pointer">
          {props.label}
        </label>
        {props.hint && <InfoTip text={props.hint} />}
      </span>
      <span class="flex shrink-0 items-center gap-2.5">
        <input
          id={uid}
          type="range"
          value={props.value}
          min={min}
          max={max}
          step={props.step}
          onInput={(e) => props.onChange(Number(e.currentTarget.value))}
          class="slider w-40"
          style={{ "--fill": `${pct()}%` } as JSX.CSSProperties}
        />
        <span class="data w-9 shrink-0 text-right text-muted tabular-nums">
          {props.format ? props.format(props.value) : props.value.toFixed(2)}
        </span>
        {props.suffix && (
          <span class="shrink-0 text-[12px] text-muted tabular-nums">{props.suffix}</span>
        )}
      </span>
    </div>
  );
}

/* A single path list rendered as a notebook ledger: one hairline-bordered box,
   flat rows divided by rules, and an add row beneath. Empty lists show honest
   guidance instead of a blank box. */
function PathList(props: {
  values: string[];
  onAdd: (v: string) => void;
  onRemove: (v: string) => void;
  title?: string;
  placeholder?: string;
  empty?: string;
}) {
  const [input, setInput] = createSignal("");

  const addInput = () => {
    if (input().trim()) {
      props.onAdd(input().trim());
      setInput("");
    }
  };

  const browse = async () => {
    const dir = await pickFolder(props.title);
    if (dir) props.onAdd(dir);
  };

  return (
    <div class="flex flex-col gap-2">
      {props.values.length === 0 ? (
        <p class="rounded-control border border-dashed border-line bg-paper/40 px-3 py-2.5 text-[13px] leading-5 text-muted">
          {props.empty ?? "Nothing here yet. Add a path below."}
        </p>
      ) : (
        <ul class="divide-y divide-line overflow-hidden rounded-control border border-line bg-paper">
          <For each={props.values}>
            {(v) => (
              <li class="group flex items-center gap-2 px-3 py-1.5">
                <span class="data min-w-0 flex-1 truncate font-mono text-[12.5px] text-muted" title={v}>
                  {v}
                </span>
                <button
                  class="shrink-0 text-faint opacity-0 transition-opacity duration-150 group-hover:opacity-100 focus-visible:opacity-100 hover:text-danger"
                  onClick={() => props.onRemove(v)}
                  aria-label={`Remove ${v}`}
                >
                  <CloseIcon size={14} />
                </button>
              </li>
            )}
          </For>
        </ul>
      )}
      <div class="flex gap-2">
        <input
          value={input()}
          onInput={(e) => setInput(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") addInput();
          }}
          placeholder={props.placeholder ?? "/absolute/path"}
          class="h-8 min-w-0 flex-1 rounded-control border border-line bg-paper px-3 text-[13px] outline-none focus:border-leaf"
          spellcheck={false}
        />
        <Button size="sm" variant="outline" onClick={() => void browse()} aria-label="Browse for folder">
          <FolderOpenIcon size={15} />
          Browse
        </Button>
        <Button size="sm" onClick={addInput}>
          Add
        </Button>
      </div>
    </div>
  );
}

/* Compact tag list for the "excluded folders" sub-setting. Paths render as
   removable chips rather than a full ledger, so it reads as a lighter sub-list
   nested under Obsidian vaults instead of a second list box. */
function ChipList(props: {
  values: string[];
  onAdd: (v: string) => void;
  onRemove: (v: string) => void;
  title?: string;
}) {
  const [input, setInput] = createSignal("");

  const addInput = () => {
    if (input().trim()) {
      props.onAdd(input().trim());
      setInput("");
    }
  };

  const browse = async () => {
    const dir = await pickFolder(props.title);
    if (dir) props.onAdd(dir);
  };

  return (
    <div class="flex flex-col gap-2">
      {props.values.length === 0 ? (
        <p class="text-[12.5px] leading-4 text-muted">None. Every folder inside a vault is indexed.</p>
      ) : (
        <ul class="flex flex-wrap gap-1.5">
          <For each={props.values}>
            {(v) => (
              <li class="inline-flex max-w-full items-center gap-1.5 rounded-full border border-line bg-paper px-2.5 py-1">
                <span class="min-w-0 truncate font-mono text-[12px] text-ink-soft" title={v}>
                  {v}
                </span>
                <button
                  class="shrink-0 text-faint transition-colors hover:text-danger"
                  onClick={() => props.onRemove(v)}
                  aria-label={`Remove ${v}`}
                >
                  <CloseIcon size={12} />
                </button>
              </li>
            )}
          </For>
        </ul>
      )}
      <div class="flex items-center gap-2">
        <input
          value={input()}
          onInput={(e) => setInput(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") addInput();
          }}
          placeholder="folder name, e.g. .trash"
          class="h-7 min-w-0 flex-1 rounded-control border border-line bg-paper px-2.5 text-[12.5px] outline-none focus:border-leaf"
          spellcheck={false}
        />
        <Button size="sm" variant="outline" onClick={() => void browse()}>
          Browse
        </Button>
        <Button size="sm" onClick={addInput}>
          Add
        </Button>
      </div>
    </div>
  );
}

/* One group inside a GroupList ledger: a name header (with path count and a
   remove button) and its own list of paths plus a compact add row. Rendered as
   its own component so each group's add-input keeps its own state. */
function GroupItem(props: {
  name: string;
  paths: string[];
  onAddPath: (name: string, v: string) => void;
  onRemovePath: (name: string, v: string) => void;
  onRemoveGroup: (name: string) => void;
  title?: string;
}) {
  const [input, setInput] = createSignal("");

  const addInput = () => {
    if (input().trim()) {
      props.onAddPath(props.name, input().trim());
      setInput("");
    }
  };

  const browse = async () => {
    const dir = await pickFolder(props.title);
    if (dir) props.onAddPath(props.name, dir);
  };

  return (
    <li class="px-3 py-3">
      <div class="mb-2 flex items-center gap-2">
        <span class="min-w-0 flex-1 truncate text-[13px] font-semibold text-ink">{props.name}</span>
        <span class="data shrink-0 rounded-full border border-line bg-surface px-2 py-0.5 font-mono text-[11px] text-muted">
          {props.paths.length} {props.paths.length === 1 ? "path" : "paths"}
        </span>
        <button
          class="shrink-0 text-faint transition-colors hover:text-danger"
          onClick={() => props.onRemoveGroup(props.name)}
          aria-label={`Remove ${props.name}`}
        >
          <CloseIcon size={14} />
        </button>
      </div>
      {props.paths.length === 0 ? (
        <p class="text-[12.5px] leading-4 text-muted">No paths yet.</p>
      ) : (
        <ul class="divide-y divide-line/60">
          <For each={props.paths}>
            {(v) => (
              <li class="group flex items-center gap-2 py-1">
                <span class="data min-w-0 flex-1 truncate font-mono text-[12.5px] text-muted" title={v}>
                  {v}
                </span>
                <button
                  class="shrink-0 text-faint opacity-0 transition-opacity duration-150 group-hover:opacity-100 focus-visible:opacity-100 hover:text-danger"
                  onClick={() => props.onRemovePath(props.name, v)}
                  aria-label={`Remove ${v}`}
                >
                  <CloseIcon size={13} />
                </button>
              </li>
            )}
          </For>
        </ul>
      )}
      <div class="mt-2 flex items-center gap-2">
        <input
          value={input()}
          onInput={(e) => setInput(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") addInput();
          }}
          placeholder="/absolute/path"
          class="h-7 min-w-0 flex-1 rounded-control border border-line bg-paper px-2.5 text-[12.5px] outline-none focus:border-leaf"
          spellcheck={false}
        />
        <Button size="sm" variant="outline" onClick={() => void browse()}>
          Browse
        </Button>
        <Button size="sm" onClick={addInput}>
          Add
        </Button>
      </div>
    </li>
  );
}

function GroupList(props: {
  groups: Record<string, string[]>;
  onAddPath: (name: string, v: string) => void;
  onRemovePath: (name: string, v: string) => void;
  onAddGroup: (name: string) => void;
  onRemoveGroup: (name: string) => void;
  title?: string;
  empty?: string;
}) {
  const [name, setName] = createSignal("");

  const addGroup = () => {
    if (name().trim()) {
      props.onAddGroup(name().trim());
      setName("");
    }
  };

  const entries = () => Object.entries(props.groups);

  return (
    <div class="flex flex-col gap-2">
      {entries().length === 0 ? (
        <p class="rounded-control border border-dashed border-line bg-paper/40 px-3 py-2.5 text-[13px] leading-5 text-muted">
          {props.empty ?? "No groups yet. Create one below, then add its folders."}
        </p>
      ) : (
        <ul class="divide-y divide-line overflow-hidden rounded-control border border-line bg-paper">
          <For each={entries()}>
            {([gname, paths]) => (
              <GroupItem
                name={gname}
                paths={paths}
                onAddPath={props.onAddPath}
                onRemovePath={props.onRemovePath}
                onRemoveGroup={props.onRemoveGroup}
                title={props.title}
              />
            )}
          </For>
        </ul>
      )}
      <div class="flex gap-2">
        <input
          value={name()}
          onInput={(e) => setName(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") addGroup();
          }}
          placeholder="collection name…"
          class="h-8 min-w-0 flex-1 rounded-control border border-line bg-paper px-3 text-[13px] outline-none focus:border-leaf"
        />
        <Button size="sm" onClick={addGroup}>
          New group
        </Button>
      </div>
    </div>
  );
}

/* Header row for one source kind inside the Sources section: an icon plate, the
   name, an explanation tip, and a mono count. It sits above the ledger so each
   source reads as an entry in the settings notebook rather than a separate card. */
function SourceHeading(props: {
  icon: JSX.Element;
  title: string;
  hint: string;
  count: number;
  unit: string;
}) {
  return (
    <div class="flex items-center gap-2.5">
      <span class="flex h-7 w-7 shrink-0 items-center justify-center rounded-control bg-surface text-leaf">
        {props.icon}
      </span>
      <h4 class="text-[13.5px] font-semibold tracking-[-0.01em] text-ink">{props.title}</h4>
      <InfoTip text={props.hint} />
      <span class="data ml-auto shrink-0 rounded-full border border-line bg-surface px-2 py-0.5 font-mono text-[11px] text-muted">
        {props.count} {props.count === 1 ? props.unit : `${props.unit}s`}
      </span>
    </div>
  );
}

/* A sub-group of the Sources section (Documents / Code): a small heading on a
   hairline rule, with a serif-italic note underneath. */
function SourceGroup(props: { title: string; note: string; children: JSX.Element }) {
  return (
    <div>
      <div class="mb-1.5 flex items-center gap-3">
        <h3 class="text-[13px] font-semibold tracking-[-0.01em] text-ink-soft">{props.title}</h3>
        <div class="h-px flex-1 bg-line" aria-hidden="true" />
      </div>
      <p class="note mb-4 text-[13px] leading-5 text-muted">{props.note}</p>
      {props.children}
    </div>
  );
}

/* Copy to the clipboard, with a legacy textarea fallback for webviews where
   the async Clipboard API is unavailable or blocked. */
const copyText = async (text: string): Promise<boolean> => {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
    /* fall through to the textarea fallback */
  }
  try {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.select();
    const ok = document.execCommand("copy");
    document.body.removeChild(ta);
    return ok;
  } catch {
    return false;
  }
};

/* A mono command or config snippet with a copy button that flips to a check
   for a beat after copying. Used by the Connect section's setup directions.
   multiline keeps the line breaks in the copied text visible (for a JSON
   block); single-line snippets truncate with ellipsis instead. */
function Snippet(props: { label: string; code: string; multiline?: boolean }) {
  const [copied, setCopied] = createSignal(false);
  const copy = async () => {
    if (await copyText(props.code)) {
      setCopied(true);
      setTimeout(() => setCopied(false), 1600);
    }
  };
  return (
    <div>
      <p class="data mb-1 text-[11.5px] font-medium uppercase tracking-[0.08em] text-muted">{props.label}</p>
      <div class="flex items-center gap-2 rounded-control border border-line bg-paper/60 px-3 py-2">
        <code
          class={`data min-w-0 flex-1 font-mono text-[12px] leading-5 text-ink-soft ${
            props.multiline ? "whitespace-pre-wrap break-all" : "truncate"
          }`}
          title={props.code}
        >
          {props.code}
        </code>
        <button
          class="shrink-0 rounded p-1 text-faint transition-colors hover:text-leaf-deep"
          onClick={() => void copy()}
          aria-label={`Copy ${props.label}`}
          title="Copy"
        >
          {copied() ? <CheckIcon size={14} /> : <CopyIcon size={14} />}
        </button>
      </div>
    </div>
  );
}

/* The read-only MCP tools vectile serves, shown in the Connect section. */
const MCP_TOOLS: { name: string; desc: string }[] = [
  {
    name: "vectile_search",
    desc: "Hybrid semantic + keyword search, filterable by collection, source type, path, and date.",
  },
  { name: "vectile_list_collections", desc: "List your collections with file and chunk counts." },
  {
    name: "vectile_collection_info",
    desc: "Details for one collection: counts, source types, and sample titles.",
  },
];

/* The three moments the sidebar mascot (Vexter) can appear for. The config key
   is the per-state show/suppress flag; the preview shows the exact animated
   webp the sidebar uses, with the shared static PNG under reduced motion. */
const DEFAULT_MASCOT: MascotConfig = {
  show_searching: true,
  show_indexing: true,
  show_nothing: true,
};

const MASCOT_STATES: {
  key: keyof MascotConfig;
  label: string;
  desc: string;
  anim: string;
  static: string;
}[] = [
  {
    key: "show_searching",
    label: "Searching",
    desc: "While a query runs",
    anim: MASCOT_ASSETS.searching,
    static: MASCOT_STATIC,
  },
  {
    key: "show_indexing",
    label: "Indexing",
    desc: "While a library rebuilds",
    anim: MASCOT_ASSETS.indexing,
    static: MASCOT_STATIC,
  },
  {
    key: "show_nothing",
    label: "No results",
    desc: "When a search comes up empty",
    anim: MASCOT_ASSETS.nothing,
    static: MASCOT_STATIC,
  },
];

/* ---- the view ---- */

const cloneCfg = (c: AppConfig): AppConfig => ({
  ...c,
  obsidian_vaults: [...c.obsidian_vaults],
  obsidian_exclude_folders: [...c.obsidian_exclude_folders],
  calibre_libraries: [...c.calibre_libraries],
  repositories: Object.fromEntries(Object.entries(c.repositories).map(([k, v]) => [k, [...v]])),
  projects: Object.fromEntries(Object.entries(c.projects).map(([k, v]) => [k, [...v]])),
  disabled_collections: [...c.disabled_collections],
  git_commit_subject_blacklist: [...c.git_commit_subject_blacklist],
  search_defaults: { ...c.search_defaults },
  gui: { ...c.gui, mascot: { ...(c.gui.mascot ?? DEFAULT_MASCOT) } },
  mcp: { ...c.mcp },
});

/** Clamp a freshly loaded config into the hard bounds so out-of-range values
    saved by an older version or a hand-edited file get corrected on open. */
const sanitizeConfig = (cfg: AppConfig): AppConfig => {
  cfg.embedding_batch_size = clamp(
    cfg.embedding_batch_size,
    STATIC_BOUNDS.embedding_batch_size.min,
    STATIC_BOUNDS.embedding_batch_size.max,
  );
  cfg.chunk_size_tokens = clamp(
    cfg.chunk_size_tokens,
    STATIC_BOUNDS.chunk_size_tokens.min,
    STATIC_BOUNDS.chunk_size_tokens.max,
  );
  cfg.chunk_overlap_tokens = clamp(
    cfg.chunk_overlap_tokens,
    STATIC_BOUNDS.chunk_overlap_tokens.min,
    Math.max(STATIC_BOUNDS.chunk_overlap_tokens.min, cfg.chunk_size_tokens - 1),
  );
  cfg.git_history_in_months = clamp(
    cfg.git_history_in_months,
    STATIC_BOUNDS.git_history_in_months.min,
    STATIC_BOUNDS.git_history_in_months.max,
  );
  cfg.search_defaults.top_k = clamp(cfg.search_defaults.top_k, STATIC_BOUNDS.top_k.min, STATIC_BOUNDS.top_k.max);
  cfg.search_defaults.rrf_k = clamp(cfg.search_defaults.rrf_k, STATIC_BOUNDS.rrf_k.min, STATIC_BOUNDS.rrf_k.max);
  cfg.search_defaults.vector_weight = clamp(
    cfg.search_defaults.vector_weight,
    STATIC_BOUNDS.vector_weight.min,
    STATIC_BOUNDS.vector_weight.max,
  );
  cfg.search_defaults.fts_weight = clamp(
    cfg.search_defaults.fts_weight,
    STATIC_BOUNDS.fts_weight.min,
    STATIC_BOUNDS.fts_weight.max,
  );
  cfg.gui.auto_reindex_interval_minutes = clamp(
    cfg.gui.auto_reindex_interval_minutes,
    STATIC_BOUNDS.auto_reindex_interval_minutes.min,
    STATIC_BOUNDS.auto_reindex_interval_minutes.max,
  );
  cfg.mcp = cfg.mcp ?? { enabled: false, port: 31123 };
  cfg.mcp.port = clamp(cfg.mcp.port, STATIC_BOUNDS.mcp_port.min, STATIC_BOUNDS.mcp_port.max);
  cfg.gui.mascot = cfg.gui.mascot ?? { ...DEFAULT_MASCOT };
  return cfg;
};

export function SettingsView() {
  const store = useAppStore();

  const draft = (): AppConfig | null => store.settingsDraft();

  createEffect(() => {
    const c = store.config();
    if (c && !store.settingsDraft()) store.replaceSettingsDraft(sanitizeConfig(cloneCfg(c)));
  });

  const setNumber = (
    k: "embedding_batch_size" | "chunk_size_tokens" | "chunk_overlap_tokens" | "git_history_in_months",
    n: number,
  ) =>
    store.setSettingsDraft((d) => {
      if (!d) return d;
      if (k === "chunk_size_tokens") {
        const size = clamp(n, STATIC_BOUNDS.chunk_size_tokens.min, STATIC_BOUNDS.chunk_size_tokens.max);
        const overlap = clamp(
          d.chunk_overlap_tokens,
          STATIC_BOUNDS.chunk_overlap_tokens.min,
          Math.max(STATIC_BOUNDS.chunk_overlap_tokens.min, size - 1),
        );
        return { ...d, chunk_size_tokens: size, chunk_overlap_tokens: overlap };
      }
      const b = boundsFor(k, d.chunk_size_tokens);
      return { ...d, [k]: clamp(n, b.min, b.max) };
    });

  const addPath = (k: "obsidian_vaults" | "obsidian_exclude_folders" | "calibre_libraries", v: string) =>
    store.setSettingsDraft((d) => (d ? { ...d, [k]: [...d[k], v] } : d));
  const removePath = (k: "obsidian_vaults" | "obsidian_exclude_folders" | "calibre_libraries", v: string) =>
    store.setSettingsDraft((d) => (d ? { ...d, [k]: d[k].filter((x) => x !== v) } : d));

  const addGroupPath = (mapKey: "projects" | "repositories", name: string, v: string) =>
    store.setSettingsDraft((d) =>
      d ? { ...d, [mapKey]: { ...d[mapKey], [name]: [...(d[mapKey][name] ?? []), v] } } : d,
    );
  const removeGroupPath = (mapKey: "projects" | "repositories", name: string, v: string) =>
    store.setSettingsDraft((d) =>
      d
        ? { ...d, [mapKey]: { ...d[mapKey], [name]: (d[mapKey][name] ?? []).filter((x) => x !== v) } }
        : d,
    );
  const addGroup = (mapKey: "projects" | "repositories", name: string) =>
    store.setSettingsDraft((d) => (d ? { ...d, [mapKey]: { ...d[mapKey], [name]: [] } } : d));
  const removeGroup = (mapKey: "projects" | "repositories", name: string) =>
    store.setSettingsDraft((d) => {
      if (!d) return d;
      const m = { ...d[mapKey] };
      delete m[name];
      return { ...d, [mapKey]: m };
    });

  const setSearch = (k: keyof SearchDefaults, n: number) =>
    store.setSettingsDraft((d) => {
      if (!d) return d;
      const b = STATIC_BOUNDS[k];
      return { ...d, search_defaults: { ...d.search_defaults, [k]: clamp(n, b.min, b.max) } };
    });
  const setGui = (p: Partial<GUIConfig>) =>
    store.setSettingsDraft((d) => {
      if (!d) return d;
      const gui = { ...d.gui, ...p };
      if (p.auto_reindex_interval_minutes !== undefined) {
        const b = STATIC_BOUNDS.auto_reindex_interval_minutes;
        gui.auto_reindex_interval_minutes = clamp(p.auto_reindex_interval_minutes, b.min, b.max);
      }
      return { ...d, gui };
    });

  const setMascot = (k: keyof MascotConfig, v: boolean) =>
    store.setSettingsDraft((d) => {
      if (!d) return d;
      return { ...d, gui: { ...d.gui, mascot: { ...d.gui.mascot, [k]: v } } };
    });
  const setMascotAll = (v: boolean) =>
    store.setSettingsDraft((d) => {
      if (!d) return d;
      return {
        ...d,
        gui: { ...d.gui, mascot: { show_searching: !v, show_indexing: !v, show_nothing: !v } },
      };
    });
  const mascotAllDisabled = () => {
    const m = draft()?.gui.mascot;
    return m ? !m.show_searching && !m.show_indexing && !m.show_nothing : false;
  };

  const setMCP = (p: Partial<MCPConfig>) =>
    store.setSettingsDraft((d) => {
      if (!d) return d;
      const mcp = { ...d.mcp, ...p };
      if (p.port !== undefined) {
        mcp.port = clamp(p.port, STATIC_BOUNDS.mcp_port.min, STATIC_BOUNDS.mcp_port.max);
      }
      return { ...d, mcp };
    });

  const running = () => store.mcpStatus()?.running ?? false;
  const mcpUrl = () => `http://127.0.0.1:${draft()!.mcp.port}/sse`;
  const claudeJson = () => `{\n  "mcpServers": {\n    "vectile": { "url": "${mcpUrl()}" }\n  }\n}`;
  const [urlCopied, setUrlCopied] = createSignal(false);
  const copyUrl = async () => {
    const u = store.mcpStatus()?.url ?? mcpUrl();
    if (await copyText(u)) {
      setUrlCopied(true);
      setTimeout(() => setUrlCopied(false), 1600);
    }
  };

  const save = async () => {
    await store.saveSettings();
  };

  const saveAndLeave = async () => {
    await store.saveSettings();
    store.confirmLeave();
  };

  /* ---- Model library (independent of the config draft) ---- */

  const activeModel = (): ModelInfo | null => store.models().find((m) => m.isActive) ?? null;

  const [selModel, setSelModel] = createSignal("");
  const [switchBusy, setSwitchBusy] = createSignal(false);
  createEffect(() => {
    const m = activeModel();
    if (m && !switchBusy() && confirmDim() === null) setSelModel(m.path);
  });

  const cpuCount = (): number => (store.cpuCount() > 0 ? store.cpuCount() : 64);

  const [modelCtx, setModelCtx] = createSignal(0);
  const [modelBatch, setModelBatch] = createSignal(32);
  const [modelThreads, setModelThreads] = createSignal(0);
  const [syncedActivePath, setSyncedActivePath] = createSignal<string | null>(null);
  createEffect(() => {
    const m = activeModel();
    if (m && m.path !== syncedActivePath()) {
      setSyncedActivePath(m.path);
      setModelCtx(m.contextWindow);
      setModelBatch(m.batchSize);
      setModelThreads(m.threads);
    }
  });

  const [confirmDim, setConfirmDim] = createSignal<{ path: string; name: string } | null>(null);
  const [confirmBusy, setConfirmBusy] = createSignal(false);

  const modelLabel = (m: ModelInfo) => (m.dimensions > 0 ? `${m.name} · ${m.dimensions}d` : m.name);

  const switchModel = async (path: string) => {
    if (path === activeModel()?.path) return;
    setSwitchBusy(true);
    setSelModel(path); // preview the pending choice in the dropdown
    try {
      const r = await store.setActiveModel(path);
      if (r.needsRebuild) setConfirmDim({ path, name: r.name });
      else setSelModel(activeModel()?.path ?? ""); // applied; resync from the store
    } catch (err) {
      setSelModel(activeModel()?.path ?? ""); // rejected; revert the dropdown
      store.pushToast(`Couldn't switch model: ${err}`, "danger");
    } finally {
      setSwitchBusy(false);
    }
  };

  const confirmDimSwitch = async () => {
    const c = confirmDim();
    if (!c) return;
    setConfirmBusy(true);
    try {
      await store.setActiveModel(c.path, true);
      setConfirmDim(null);
    } catch (err) {
      setSelModel(activeModel()?.path ?? ""); // failed; revert the dropdown
      store.pushToast(`Couldn't switch model: ${err}`, "danger");
    } finally {
      setConfirmBusy(false);
    }
  };

  const importModelFlow = async () => {
    const p = await pickModelFile();
    if (!p) return;
    try {
      const m = await importModel(p);
      await store.loadModels();
      store.pushToast(`Imported ${m.name}`, "success");
    } catch (err) {
      store.pushToast(`Import failed: ${err}`, "danger");
    }
  };

  const removeModel = async (m: ModelInfo) => {
    await store.deleteModel(m.path, m.name);
  };

  const saveModelSettings = async () => {
    const m = activeModel();
    if (!m) return;
    await store.updateModelSettings(m.id, modelCtx(), modelBatch(), modelThreads());
  };

  return (
    <div class="relative flex h-full flex-col">
      <ViewHeading title="Settings" note="Model, chunking, search, and sources. Everything stays on this machine." />

      <Show when={draft()} fallback={<p class="note text-muted">Loading settings…</p>}>
        <div class="scroll-quiet -mr-2 flex-1 overflow-y-auto pb-20 pr-2">
          <Section
            title="Model"
            note="The embedding engine runs in-process. Drop a .gguf into the models folder, or import one below."
          >
            <div class="flex flex-wrap items-center gap-3">
              <StatusPill state={store.modelState()} name={store.modelName()} />
            </div>
            <div class="data truncate text-muted" title={store.status()?.modelPath ?? ""}>
              {store.status()?.modelPath ?? "…"}
            </div>

            <div class="rounded-control border border-line bg-paper/60 p-3">
              <div class="mb-2 flex items-center justify-between gap-3">
                <p class="text-[13px] font-medium text-ink-soft">Get a model</p>
                <button
                  type="button"
                  class="text-[12px] font-medium text-leaf hover:underline"
                  onClick={() => openExternal("https://huggingface.co/models?library=gguf&sort=trending&search=embedding")}
                >
                  Browse Hugging Face
                </button>
              </div>
              <div class="space-y-2">
                <For each={store.recommended()}>
                  {(m) => (
                    <CatalogModelCard
                      model={m}
                      installedModels={store.models()}
                      downloadState={store.downloadState()}
                      onDownload={(k) => store.downloadModelByKey(k)}
                      onUninstall={(f) => store.uninstallCatalogFile(f)}
                      onCancel={() => store.cancelDownload()}
                    />
                  )}
                </For>
              </div>
            </div>

            <div class="flex items-center justify-between gap-4">
              <span class="text-[13.5px] text-ink-soft">Active model</span>
              <Select
                aria-label="Active model"
                value={selModel()}
                options={store.models().map((m) => ({ value: m.path, label: modelLabel(m) }))}
                onChange={(v) => void switchModel(v)}
              />
            </div>

            <div class="flex items-center justify-between gap-4">
              <span class="text-[13.5px] text-ink-soft">Add a model file</span>
              <Button size="sm" onClick={() => void importModelFlow()}>
                Import model…
              </Button>
            </div>

            <Show when={activeModel()}>
              {(m) => (
                <div class="rounded-control border border-line bg-paper p-3">
                  <p class="mb-2 flex items-center gap-1.5 text-[13px] font-medium text-ink-soft">
                    {m().name} settings
                    <InfoTip text="Each model carries its own settings. Context window 0 falls back to the model's native maximum (shown here when the .gguf reports one); threads 0 uses all cores." />
                  </p>
                  <div class="data mb-2 truncate text-muted">
                    Dimensions: {m().dimensions > 0 ? m().dimensions : "auto"}
                  </div>
                  <NumField
                    label="Context window (tokens)"
                    value={modelCtx()}
                    onChange={setModelCtx}
                    hint="How many tokens the model can read at once. 0 = the model's native maximum. Raise it for long chunks, lower it to save memory."
                    min={0}
                    max={8192}
                    step={256}
                  />
                  <NumField
                    label="Embedding batch size"
                    value={modelBatch()}
                    onChange={setModelBatch}
                    hint="How many chunks get fed to the model at once. A bigger number finishes indexing faster but uses more memory while it runs. If a large library makes the app stall, drop it to something like 16."
                    min={STATIC_BOUNDS.embedding_batch_size.min}
                    max={STATIC_BOUNDS.embedding_batch_size.max}
                    step={STATIC_BOUNDS.embedding_batch_size.step}
                  />
                  <RangeField
                    label="CPU threads"
                    value={modelThreads()}
                    onChange={setModelThreads}
                    hint={`0 = auto, which uses all ${cpuCount()} logical cores. Drag to reserve some for the rest of the system. Lower it if indexing starves other apps.`}
                    min={0}
                    max={cpuCount()}
                    step={1}
                    format={(n) => (n <= 0 ? "auto" : String(Math.round(n)))}
                    suffix={`of ${cpuCount()}`}
                  />
                  <div class="mt-2 flex justify-end">
                    <Button size="sm" onClick={() => void saveModelSettings()}>
                      Save model settings
                    </Button>
                  </div>
                </div>
              )}
            </Show>

            <Show when={store.models().length > 0}>
              <p class="mb-1 text-[13px] font-medium text-ink-soft">Installed models</p>
              <ul class="space-y-1">
                <For each={store.models()}>
                  {(m) => (
                    <li class="flex items-center gap-2 rounded-control border border-line bg-paper px-3 py-1.5">
                      <span class="data flex-1 truncate text-muted" title={m.path}>
                        {modelLabel(m)}
                      </span>
                      {m.isActive && (
                        <span class="shrink-0 rounded-control bg-mint px-1.5 py-0.5 text-[11px] font-medium text-leaf-deep">
                          active
                        </span>
                      )}
                      <button
                        class="shrink-0 text-faint transition-colors hover:text-danger disabled:opacity-40 disabled:pointer-events-none"
                        aria-label={`Remove ${m.name}`}
                        disabled={m.isActive}
                        onClick={() => void removeModel(m)}
                      >
                        <CloseIcon size={14} />
                      </button>
                    </li>
                  )}
                </For>
              </ul>
            </Show>

            <Show when={store.models().length === 0}>
              <p class="note text-muted">
                No models yet. Import a .gguf or drop one into the models folder.
              </p>
            </Show>

            <ConfirmDialog
              open={confirmDim() !== null}
              title="Switching changes the embedding dimension"
              body={
                <>
                  <p>
                    <span class="font-medium text-ink">{confirmDim()?.name}</span> uses a
                    different embedding dimension than the current model. Every indexed
                    collection will need to be re-indexed before meaning search works again,
                    and all existing embeddings will be cleared.
                  </p>
                  <p class="mt-2">Switch anyway?</p>
                </>
              }
              confirmLabel="Switch & re-index"
              busyLabel="Switching…"
              busy={confirmBusy()}
              onCancel={() => {
                setConfirmDim(null);
                setSelModel(activeModel()?.path ?? ""); // keep the previous model
              }}
              onConfirm={() => void confirmDimSwitch()}
            />
          </Section>

          <Section title="Chunking" note="Smaller chunks match more precisely; overlap keeps sentences intact.">
            <NumField
              label="Chunk size (words)"
              value={draft()!.chunk_size_tokens}
              onChange={(n) => setNumber("chunk_size_tokens", n)}
              hint="How many words each indexed slice holds. Search matches slices, not whole files, so this sets how finely results are cut. Smaller chunks match more precisely; bigger ones carry more context. 500 is a safe start."
              min={STATIC_BOUNDS.chunk_size_tokens.min}
              max={STATIC_BOUNDS.chunk_size_tokens.max}
              step={STATIC_BOUNDS.chunk_size_tokens.step}
            />
            <NumField
              label="Chunk overlap (words)"
              value={draft()!.chunk_overlap_tokens}
              onChange={(n) => setNumber("chunk_overlap_tokens", n)}
              hint="How many words repeat from one slice into the next, so sentences that straddle a cut stay searchable whole. Too little overlap and text slips through; too much and it gets stored twice. 50 is the usual start."
              min={STATIC_BOUNDS.chunk_overlap_tokens.min}
              max={Math.max(STATIC_BOUNDS.chunk_overlap_tokens.min, draft()!.chunk_size_tokens - 1)}
              step={STATIC_BOUNDS.chunk_overlap_tokens.step}
            />
          </Section>

          <Section title="Search" note="Hybrid ranking blends exact-term and meaning results.">
            <NumField
              label="Top results"
              value={draft()!.search_defaults.top_k}
              onChange={(n) => setSearch("top_k", n)}
              hint="How many matches a search returns by default. Raise it for a longer list, lower it for a shorter one. Set a different number per search under Filters."
              min={STATIC_BOUNDS.top_k.min}
              max={STATIC_BOUNDS.top_k.max}
              step={STATIC_BOUNDS.top_k.step}
            />
            <NumField
              label="RRF constant (k)"
              value={draft()!.search_defaults.rrf_k}
              onChange={(n) => setSearch("rrf_k", n)}
              hint="A smoothing value in the math that merges the two search lists. Bigger k flattens the gap between high- and low-ranked matches, so entries further down still get a fair shot. 60 is the usual value."
              min={STATIC_BOUNDS.rrf_k.min}
              max={STATIC_BOUNDS.rrf_k.max}
              step={STATIC_BOUNDS.rrf_k.step}
            />
            <RangeField
              label="Vector weight"
              value={draft()!.search_defaults.vector_weight}
              onChange={(n) => setSearch("vector_weight", n)}
              hint="How much the meaning-based ranking counts when the two search lists are blended. It works against the full-text weight like a seesaw: raise it and results lean toward semantic matches, even when the words don't line up exactly."
              min={STATIC_BOUNDS.vector_weight.min}
              max={STATIC_BOUNDS.vector_weight.max}
              step={STATIC_BOUNDS.vector_weight.step}
            />
            <RangeField
              label="Full-text weight"
              value={draft()!.search_defaults.fts_weight}
              onChange={(n) => setSearch("fts_weight", n)}
              hint="How much exact-word matches count in the final blend. Raise it when you're hunting a precise phrase or a name and want literal hits to win. Lower it and meaning takes over from wording."
              min={STATIC_BOUNDS.fts_weight.min}
              max={STATIC_BOUNDS.fts_weight.max}
              step={STATIC_BOUNDS.fts_weight.step}
            />
          </Section>

          <Section title="Sources" note="Folders are walked recursively; nested git repos are discovered automatically.">
            <div class="space-y-7">
              <SourceGroup
                title="Documents"
                note="Notes and books you read, searchable by meaning and keyword."
              >
                <div class="grid items-start gap-x-8 gap-y-6 md:grid-cols-2">
                  <div class="space-y-3">
                    <SourceHeading
                      icon={<FileIcon size={15} />}
                      title="Obsidian vaults"
                      hint="Point at an Obsidian vault and every markdown note in it gets indexed, subfolders included. Use the exclude list below to keep noisy folders out."
                      count={draft()!.obsidian_vaults.length}
                      unit="path"
                    />
                    <div id="setup-add-folder">
                      <PathList
                        values={draft()!.obsidian_vaults}
                        onAdd={(v) => addPath("obsidian_vaults", v)}
                        onRemove={(v) => removePath("obsidian_vaults", v)}
                        title="Choose an Obsidian vault"
                        placeholder="path to a vault…"
                        empty="No vaults yet. Add one and its notes become searchable."
                      />
                    </div>
                    <div class="space-y-2.5 rounded-control border border-line bg-paper/60 p-3">
                      <SourceHeading
                        icon={<SlashIcon size={14} />}
                        title="Excluded folders"
                        hint="Folders listed here are skipped when vaults are indexed. Handy for hiding attachments, templates, .trash, or anything else you don't want in search results."
                        count={draft()!.obsidian_exclude_folders.length}
                        unit="folder"
                      />
                      <ChipList
                        values={draft()!.obsidian_exclude_folders}
                        onAdd={(v) => addPath("obsidian_exclude_folders", v)}
                        onRemove={(v) => removePath("obsidian_exclude_folders", v)}
                        title="Choose a folder to exclude"
                      />
                    </div>
                  </div>
                  <div class="space-y-3">
                    <SourceHeading
                      icon={<LibraryIcon size={15} />}
                      title="Calibre libraries"
                      hint="Point at a Calibre library. The app reads the metadata and indexes the text of the formats it understands, so your books are searchable without opening them."
                      count={draft()!.calibre_libraries.length}
                      unit="path"
                    />
                    <PathList
                      values={draft()!.calibre_libraries}
                      onAdd={(v) => addPath("calibre_libraries", v)}
                      onRemove={(v) => removePath("calibre_libraries", v)}
                      title="Choose a Calibre library"
                      placeholder="path to a library…"
                      empty="No libraries yet. Add a Calibre library to search its books."
                    />
                  </div>
                </div>
              </SourceGroup>

              <SourceGroup
                title="Code"
                note="Folders you work in, grouped into searchable collections."
              >
                <div class="grid items-start gap-x-8 gap-y-6 md:grid-cols-2">
                  <div class="space-y-3">
                    <SourceHeading
                      icon={<FolderOpenIcon size={15} />}
                      title="Project folders"
                      hint="A group of folders indexed together as one collection. Each group becomes its own searchable set, so you can keep client work separate from personal files."
                      count={Object.keys(draft()!.projects).length}
                      unit="group"
                    />
                    <GroupList
                      groups={draft()!.projects}
                      onAddPath={(n, v) => addGroupPath("projects", n, v)}
                      onRemovePath={(n, v) => removeGroupPath("projects", n, v)}
                      onAddGroup={(n) => addGroup("projects", n)}
                      onRemoveGroup={(n) => removeGroup("projects", n)}
                      title="Choose a project folder"
                      empty="No project groups yet. Create one, then add its folders."
                    />
                  </div>
                  <div class="space-y-3">
                    <SourceHeading
                      icon={<CodeIcon size={15} />}
                      title="Code repositories"
                      hint="Git repositories to index as code. Indexes the current file tree and the commit history (how far back is set below), nested repos included."
                      count={Object.keys(draft()!.repositories).length}
                      unit="group"
                    />
                    <GroupList
                      groups={draft()!.repositories}
                      onAddPath={(n, v) => addGroupPath("repositories", n, v)}
                      onRemovePath={(n, v) => removeGroupPath("repositories", n, v)}
                      onAddGroup={(n) => addGroup("repositories", n)}
                      onRemoveGroup={(n) => removeGroup("repositories", n)}
                      title="Choose a code repository"
                      empty="No repository groups yet. Create one, then add its repos."
                    />
                  </div>
                </div>
              </SourceGroup>
            </div>
          </Section>

          <Section title="Indexing">
            <NumField
              label="Commit history (months)"
              value={draft()!.git_history_in_months}
              onChange={(n) => setNumber("git_history_in_months", n)}
              hint="How many months of git history get indexed for a repository. Each commit becomes a searchable document, so this controls how far back you can dig through your changelog. 6 months is the default."
              min={STATIC_BOUNDS.git_history_in_months.min}
              max={STATIC_BOUNDS.git_history_in_months.max}
              step={STATIC_BOUNDS.git_history_in_months.step}
            />
            <Toggle
              checked={draft()!.gui.auto_reindex}
              onChange={(v) => setGui({ auto_reindex: v })}
              label="Auto-reindex"
              description="Re-index all enabled collections on a timer."
              hint="Re-index all your collections on a timer, so files you add or edit show up in search without running anything manually. Off by default: a full pass uses your CPU and model for a while. Turn it on if you add files often."
            />
            <Show when={draft()!.gui.auto_reindex}>
              <NumField
                label="Interval (minutes)"
                value={draft()!.gui.auto_reindex_interval_minutes}
                onChange={(n) => setGui({ auto_reindex_interval_minutes: n })}
                hint="How often the auto-reindex timer fires. 60 means once an hour. Only matters when Auto-reindex is switched on."
                min={STATIC_BOUNDS.auto_reindex_interval_minutes.min}
                max={STATIC_BOUNDS.auto_reindex_interval_minutes.max}
                step={STATIC_BOUNDS.auto_reindex_interval_minutes.step}
              />
            </Show>
            <Toggle
              checked={draft()!.gui.start_on_login}
              onChange={(v) => setGui({ start_on_login: v })}
              label="Start on login"
              description="Launch vectile when you sign in."
              hint="Launch vectile when you sign in, so it's already open and indexing before you need it."
            />
          </Section>

          <Section
            title="Vexter"
            note="The pixel dinosaur in the sidebar. It pokes up while your library works."
          >
            <Toggle
              checked={mascotAllDisabled()}
              onChange={(v) => setMascotAll(v)}
              label="Disable Vexter"
              description="Hide the mascot for every moment at once."
              hint="Vexter is the small pixel dinosaur in the sidebar. When this is on, it never appears, whether you're searching, indexing, or turning up nothing."
            />
            <div class="divide-y divide-line/60 overflow-hidden rounded-control border border-line bg-paper">
              <For each={MASCOT_STATES}>
                {(s) => (
                  <div class={`flex items-center gap-4 px-3 py-2.5 ${draft()!.gui.mascot[s.key] ? "" : "opacity-60"}`}>
                    <div class="mascot-preview shrink-0">
                      <img class="mascot-preview__anim" src={s.anim} alt="" draggable={false} />
                      <img class="mascot-preview__static" src={s.static} alt="" draggable={false} />
                    </div>
                    <div class="min-w-0 flex-1">
                      <p class="text-[13.5px] font-medium leading-tight text-ink">{s.label}</p>
                      <p class="text-[12.5px] leading-5 text-muted">{s.desc}</p>
                    </div>
                    <Switch
                      checked={draft()!.gui.mascot[s.key]}
                      onChange={(v) => setMascot(s.key, v)}
                      label={`Show Vexter: ${s.label.toLowerCase()}`}
                    />
                  </div>
                )}
              </For>
            </div>
          </Section>

          <Section
            title="Connect"
            note="Let AI assistants on this machine search your library. Read-only: they can search, never change."
          >
            {/* Status plate: live from the backend, not the draft */}
            <div class="rounded-[9px] border border-line bg-paper-warm px-3.5 py-3">
              <div class="flex items-center gap-2">
                <span class="relative flex h-2 w-2 shrink-0">
                  <span class={`h-2 w-2 rounded-full ${running() ? "bg-leaf" : "bg-faint"}`} />
                </span>
                <span class={`text-[12px] font-semibold leading-none ${running() ? "text-leaf-deep" : "text-muted"}`}>
                  {running() ? "running" : "stopped"}
                </span>
                <Show when={running()}>
                  <button
                    class="ml-auto flex shrink-0 items-center gap-1 rounded-control px-1.5 py-1 text-[11.5px] text-muted transition-colors hover:bg-surface hover:text-leaf-deep"
                    onClick={() => void copyUrl()}
                    title="Copy URL"
                  >
                    {urlCopied() ? <CheckIcon size={13} /> : <CopyIcon size={13} />}
                    {urlCopied() ? "copied" : "copy URL"}
                  </button>
                </Show>
              </div>
              <Show
                when={running()}
                fallback={
                  <p class="note mt-2 text-[12.5px] leading-4 text-muted">
                    No server running. Enable it below, then save settings.
                  </p>
                }
              >
                <p class="data mt-2 truncate font-mono text-[12px] text-ink-soft" title={store.mcpStatus()?.url}>
                  {store.mcpStatus()?.url}
                </p>
              </Show>
              <div class="mt-2.5 border-t border-line" aria-hidden="true" />
              <p class="note mt-2 text-[11.5px] leading-4 text-muted">
                binds to 127.0.0.1 · nothing leaves this machine
              </p>
            </div>

            <Toggle
              checked={draft()!.mcp.enabled}
              onChange={(v) => setMCP({ enabled: v })}
              label="Share your library with AI tools"
              description="Serve search tools over MCP on 127.0.0.1."
              hint="Starts a local MCP server that AI assistants on this machine can connect to. Applies when you save settings. The server answers only on your machine."
            />
            <Show when={draft()!.mcp.enabled}>
              <NumField
                label="Port"
                value={draft()!.mcp.port}
                onChange={(n) => setMCP({ port: n })}
                hint="The port the MCP server listens on. Clients connect to http://127.0.0.1:<port>/sse. Applies when you save."
                min={STATIC_BOUNDS.mcp_port.min}
                max={STATIC_BOUNDS.mcp_port.max}
                step={STATIC_BOUNDS.mcp_port.step}
              />
            </Show>

            <div>
              <h4 class="flex items-center gap-1.5 text-[13px] font-semibold tracking-[-0.01em] text-ink">
                <PlugIcon size={14} class="text-leaf" />
                What your AI can do
              </h4>
              <p class="note mb-2 mt-0.5 text-[12.5px] leading-4 text-muted">
                Three read-only tools, scoped to your library.
              </p>
              <ul class="divide-y divide-line/60 overflow-hidden rounded-control border border-line bg-paper">
                <For each={MCP_TOOLS}>
                  {(t) => (
                    <li class="flex items-start gap-3 px-3 py-2">
                      <span class="data mt-px shrink-0 font-mono text-[11.5px] text-leaf-deep">{t.name}</span>
                      <span class="text-[12.5px] leading-5 text-ink-soft">{t.desc}</span>
                    </li>
                  )}
                </For>
              </ul>
            </div>

            <div>
              <h4 class="text-[13px] font-semibold tracking-[-0.01em] text-ink">How to connect</h4>
              <p class="note mb-2 mt-0.5 text-[12.5px] leading-4 text-muted">
                Point an MCP client at the URL below.
              </p>
              <div class="space-y-3">
                <Snippet label="Claude Desktop" code={claudeJson()} multiline />
                <Snippet label="Claude Code" code={`claude mcp add vectile --transport sse ${mcpUrl()}`} />
                <Snippet label="Any MCP SSE client" code={mcpUrl()} />
              </div>
            </div>
          </Section>
        </div>
      </Show>

      {/* Sticky save bar: pinned to the bottom of the view so saving doesn't
          mean scrolling back to the top. Only appears while the draft is dirty. */}
      <Show when={store.settingsDirty()}>
        <div class="absolute inset-x-0 bottom-0 flex items-center gap-3 border-t border-line bg-paper/90 px-6 py-3">
          <span
            class="inline-flex items-center gap-1.5 rounded-full border border-leaf/30 bg-mint px-2 py-0.5 text-[11.5px] font-medium text-leaf-deep"
            role="status"
          >
            <span class="h-1.5 w-1.5 rounded-full bg-leaf" aria-hidden="true" />
            unsaved
          </span>
          <span class="ml-auto flex items-center gap-2">
            <Button variant="ghost" onClick={() => store.confirmLeave({ discard: true })}>
              Discard
            </Button>
            <Button onClick={() => void save()}>Save settings</Button>
          </span>
        </div>
      </Show>

      {/* Leaving Settings with unsaved edits: the store holds the navigation and
          this dialog decides whether the draft is saved or dropped. */}
      <Show when={store.pendingLeave() !== null}>
        <div
          class="fixed inset-0 z-50 flex items-center justify-center bg-ink/20 p-4"
          onClick={() => store.cancelLeave()}
        >
          <div class="sheet w-[24rem] p-5 shadow-pop" onClick={(e) => e.stopPropagation()}>
            <h3 class="title text-[15px] tracking-[-0.01em] text-ink">Save your changes before leaving?</h3>
            <p class="read mt-2 text-[13.5px] leading-5 text-muted">
              You have unsaved changes. If you leave now, they'll be lost.
            </p>
            <div class="mt-5 flex flex-col gap-2">
              <Button autofocus onClick={() => void saveAndLeave()}>
                Save settings
              </Button>
              <Button variant="outline" onClick={() => store.confirmLeave({ discard: true })}>
                Leave without saving
              </Button>
              <Button variant="ghost" onClick={() => store.cancelLeave()}>
                Keep editing
              </Button>
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
}
