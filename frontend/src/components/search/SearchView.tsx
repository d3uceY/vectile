import { createSignal, For, Show, onCleanup, type JSX } from "solid-js";
import { useAppStore } from "../../lib/store";
import { daysSince } from "../../lib/time";
import type { SearchFilters } from "../../lib/types";
import { exampleQueries, termsOf } from "../../lib/mock";
import { SearchIcon, CloseIcon, ReindexIcon, DownloadIcon } from "../ui/icons";
import { EmptyState, Button, Kbd, Select, Skeleton } from "../ui/primitives";
import { ResultCard } from "./ResultCard";

const typeOptions = [
  { value: "", label: "Any type" },
  { value: "markdown", label: "Markdown" },
  { value: "pdf", label: "PDF" },
  { value: "epub", label: "EPUB" },
  { value: "plaintext", label: "Plain text" }, 
  { value: "xlsx", label: "Excel" },
  { value: "pptx", label: "PowerPoint" },
  { value: "ipynb", label: "Notebook" },
  { value: "xml", label: "XML" },
  { value: "sql", label: "SQL" },
  { value: "shell", label: "Shell" },
  { value: "code", label: "Code" },
];

const SEARCH_DEBOUNCE_MS = 350;

export function SearchView() {
  const store = useAppStore();
  const [value, setValue] = createSignal<string>(store.query());
  const [showAdvanced, setShowAdvanced] = createSignal(false);
  const [debounce, setDebounce] = createSignal<number | null>(null);
  const [filterDebounce, setFilterDebounce] = createSignal<number | null>(null);

  onCleanup(() => {
    window.clearTimeout(debounce() ?? undefined);
    window.clearTimeout(filterDebounce() ?? undefined);
  });

  const collectionOptions = () => [
    { value: "", label: "All collections" },
    ...store.collections().map((c) => ({ value: c.name, label: c.name })),
  ];

  const onInput = (text: string) => {
    setValue(text);
    store.noteTyping();
    window.clearTimeout(debounce() ?? undefined);
    setDebounce(
      window.setTimeout(() => store.runSearch(text, store.filters()), SEARCH_DEBOUNCE_MS),
    );
  };

  const applyFilter = (patch: Partial<SearchFilters>, debounced = false) => {
    const next = { ...store.filters(), ...patch };
    window.clearTimeout(debounce() ?? undefined);
    if (!debounced) {
      window.clearTimeout(filterDebounce() ?? undefined);
      store.runSearch(value(), next);
      return;
    }
    window.clearTimeout(filterDebounce() ?? undefined);
    setFilterDebounce(
      window.setTimeout(() => store.runSearch(value(), next), SEARCH_DEBOUNCE_MS),
    );
  };

  const clearAll = () => {
    window.clearTimeout(debounce() ?? undefined);
    window.clearTimeout(filterDebounce() ?? undefined);
    setValue("");
    store.clearSearch();
  };

  const pickExample = (q: string) => {
    window.clearTimeout(debounce() ?? undefined);
    window.clearTimeout(filterDebounce() ?? undefined);
    setValue(q);
    store.runSearch(q, store.filters());
  };

  const anyFilter = () =>
    store.filters().collection ||
    store.filters().sourceType ||
    store.filters().path ||
    store.filters().sender ||
    store.filters().dateFrom ||
    store.filters().dateTo;

  const freshnessDays = () => {
    const last = store.status()?.lastIndexed;
    return last ? daysSince(last) : null;
  };
  const showFreshness = () =>
    !store.noModel() &&
    freshnessDays() !== null &&
    freshnessDays()! >= 1 &&
    store.config()?.gui.auto_reindex === false &&
    !store.indexing();

  const resultOptions = () => {
    const opts = new Set([8, 12, 24, 48]);
    opts.add(store.filters().topK);
    return [...opts].sort((a, b) => a - b);
  };

  return (
    <div class="relative flex h-full flex-col">
      {/* The search bar */}
      <div class="relative z-10">
        <div
          class={`flex items-center gap-3 rounded-[12px] border border-line-control bg-surface px-4 transition-[border-color,box-shadow] duration-150 ease-snappy focus-within:border-leaf-deep focus-within:ring-1 focus-within:ring-leaf/40 ${
            value() ? "shadow-card" : "shadow-xs"
          }`}
        >
          <SearchIcon size={18} class="shrink-0 text-muted" />
          <input
            id="search-input"
            ref={(el) => store.registerSearchInput(el)}
            class="h-11 w-full bg-transparent text-[14px] font-medium text-ink outline-none placeholder:text-muted disabled:cursor-not-allowed"
            placeholder={
              store.noModel()
                ? "Add a model to turn search on"
                : "Search your notes, books, and code"
            }
            value={value()}
            disabled={store.noModel()}
            onInput={(e) => onInput(e.currentTarget.value)}
            onKeyDown={(e) => {
              if (e.key === "Escape" && value()) clearAll();
            }}
            aria-label="Search"
            spellcheck={false}
            autocomplete="off"
          />
          <Show when={value()}>
            <button
              class="flex h-8 w-8 shrink-0 items-center justify-center rounded-[4px] text-faint transition-colors duration-150 hover:bg-surface-2 hover:text-ink"
              onClick={clearAll}
              aria-label="Clear search"
            >
              <CloseIcon size={15} />
            </button>
          </Show>
          <Kbd class="shrink-0">{navigator.platform.toLowerCase().includes("mac") ? "⌘K" : "Ctrl K"}</Kbd>
        </div>

        {/* Filter bar */}
        <div class="mt-3 flex flex-wrap items-center gap-2">
          <Select
            label=""
            options={collectionOptions()}
            value={store.filters().collection ?? ""}
            onChange={(v) => applyFilter({ collection: v })}
            aria-label="Filter by collection"
          />
          <Select
            options={typeOptions}
            value={store.filters().sourceType ?? ""}
            onChange={(v) => applyFilter({ sourceType: v as SearchFilters["sourceType"] })}
            aria-label="Filter by source type"
          />
          <button
            class={`inline-flex h-9 items-center gap-1.5 rounded-control border px-3 text-[13px] font-medium transition-colors duration-150 ${
              showAdvanced()
                ? "border-ink bg-surface-2 text-ink"
                : "border-line-control bg-surface text-ink-soft hover:border-faint"
            }`}
            onClick={() => setShowAdvanced((v) => !v)}
            aria-expanded={showAdvanced()}
          >
            Filters
            <Show when={anyFilter()}>
              <span class="h-1.5 w-1.5 rounded-full bg-leaf-deep" />
            </Show>
          </button>
        </div>

        {/* Advanced filters */}
        <Show when={showAdvanced()}>
          <div class="mt-3 grid grid-cols-2 gap-3 rounded-card border border-line-strong bg-surface p-4 md:grid-cols-4">
            <FilterField label="Path contains">
              <input
                class="h-9 w-full rounded-control border border-line-control bg-paper px-3 text-[13px] font-medium outline-none placeholder:text-muted focus:border-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-leaf-deep"
                placeholder="e.g. rustyquill"
                value={store.filters().path ?? ""}
                onInput={(e) => applyFilter({ path: e.currentTarget.value }, true)}
              />
            </FilterField>
            <FilterField label="Sender / author">
              <input
                class="h-9 w-full rounded-control border border-line-control bg-paper px-3 text-[13px] font-medium outline-none placeholder:text-muted focus:border-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-leaf-deep"
                placeholder="e.g. orders@…"
                value={store.filters().sender ?? ""}
                onInput={(e) => applyFilter({ sender: e.currentTarget.value }, true)}
              />
            </FilterField>
            <FilterField label="From">
              <input
                type="date"
                class="h-9 w-full rounded-control border border-line-control bg-paper px-2 text-[13px] font-medium outline-none focus:border-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-leaf-deep"
                value={store.filters().dateFrom ?? ""}
                onChange={(e) => applyFilter({ dateFrom: e.currentTarget.value })}
              />
            </FilterField>
            <FilterField label="Results">
              <Select
                class="w-full"
                options={resultOptions().map((n) => ({ value: String(n), label: String(n) }))}
                value={String(store.filters().topK)}
                onChange={(v) => applyFilter({ topK: Number(v) })}
                aria-label="Results"
              />
            </FilterField>
          </div>
        </Show>

        <Show when={showFreshness()}>
          <FreshnessBar
            days={freshnessDays()!}
            disabled={!store.canIndex()}
            onReindex={() => void store.startIndexAll()}
          />
        </Show>
      </div>

      {/* Results area: the view's own scroll region */}
      <div class="scroll-quiet relative mt-6 min-h-0 flex-1 overflow-y-auto">
        <Show
          when={store.noModel()}
          fallback={
            <Show
              when={store.searchState() === "idle"}
              fallback={
                <Show when={store.searchState() === "searching"} fallback={<ResultList />}>
                  <SearchingState />
                </Show>
              }
            >
              <IdleState onPick={pickExample} />
            </Show>
          }
        >
          <NoModelState />
        </Show>
      </div>
    </div>
  );
}

function FilterField(props: { label: string; children: JSX.Element }) {
  return (
    <label class="flex flex-col gap-1.5">
      <span class="text-[12px] font-medium text-muted">{props.label}</span>
      {props.children}
    </label>
  );
}

function NoModelState() {
  const store = useAppStore();
  return (
    <div class="flex h-full items-center justify-center">
      <EmptyState
        icon={<DownloadIcon size={20} />}
        title="No model to search with"
        note="Search ranks by meaning, which needs a local embedding model. Add one and the box turns back on — nothing leaves this machine."
      >
        <Button onClick={() => store.openSettings("model")}>Add a model</Button>
      </EmptyState>
    </div>
  );
}

function IdleState(props: { onPick: (q: string) => void }) {
  return (
    <div class="relative flex h-full flex-col items-center justify-center">
      <div class="relative flex flex-col items-center text-center">
        <h2 class="od-fade-slide-up text-[32px] font-semibold leading-[1.2] tracking-[-0.02em] text-ink">
          Find what you half-remember
        </h2>
        <p class="od-fade-slide-up mt-2.5 max-w-[60ch] text-[14px] font-medium leading-6 text-muted">
          Search notes, books, and code by meaning, not just exact words
        </p>
        <div class="mt-8 flex flex-col items-center gap-3">
          <span class="font-mono text-[11px] text-muted">Try a search</span>
          <div class="od-stagger flex max-w-xl flex-wrap items-center justify-center gap-x-5 gap-y-2.5">
            <For each={exampleQueries}>
              {(q) => (
                <button
                  class="text-[13px] font-medium text-ink-soft underline decoration-line-strong underline-offset-4 transition-colors duration-150 ease-snappy hover:text-ink hover:decoration-ink active:translate-y-px"
                  onClick={() => props.onPick(q)}
                >
                  {q}
                </button>
              )}
            </For>
          </div>
        </div>
      </div>
    </div>
  );
}

function SearchingState() {
  return (
    <div class="space-y-3">
      <For each={[0, 1, 2, 3]}>
        {(i) => (
          <div class="rounded-card border border-line bg-surface p-5 shadow-xs">
            <Skeleton class="mb-3 h-4 w-1/3" />
            <Skeleton class="mb-2 h-3 w-full" />
            <Skeleton class="mb-2 h-3 w-5/6" />
            <Skeleton class="h-3 w-2/3" />
          </div>
        )}
      </For>
    </div>
  );
}

function ResultList() {
  const store = useAppStore();
  const results = () => store.results();
  return (
    <Show
      when={results().length > 0}
      fallback={
        <EmptyState
          icon={<SearchIcon size={20} />}
          title="Nothing matched"
          note="Try fewer words or drop a filter"
        />
      }
    >
      <div class="enter-stagger space-y-3">
        <div class="flex items-center justify-between gap-3">
          <p class="flex items-center gap-2 text-[12px] font-medium text-muted">
            {results().length} result{results().length === 1 ? "" : "s"}
            <Show when={store.searchCached()}>
              <span
                class="font-mono text-[11px]"
                title="Embedding reused from cache. Ranking is still fresh."
              >
                · Cached
              </span>
            </Show>
          </p>
          <div
            class="flex items-center rounded-[4px] border border-line bg-surface-2 p-0.5"
            role="group"
            aria-label="Result score display"
          >
            <button
              type="button"
              class={`h-7 rounded-[3px] px-2.5 text-[11px] font-medium transition-colors duration-150 ease-snappy ${
                store.scoreDisplay() === "rank" ? "bg-paper text-ink shadow-xs ring-1 ring-line-strong" : "text-muted hover:text-ink"
              }`}
              onClick={() => store.setScoreDisplay("rank")}
              aria-pressed={store.scoreDisplay() === "rank"}
            >
              Rank
            </button>
            <button
              type="button"
              class={`h-7 rounded-[3px] px-2.5 text-[11px] font-medium transition-colors duration-150 ease-snappy ${
                store.scoreDisplay() === "percent" ? "bg-paper text-ink shadow-xs ring-1 ring-line-strong" : "text-muted hover:text-ink"
              }`}
              onClick={() => store.setScoreDisplay("percent")}
              aria-pressed={store.scoreDisplay() === "percent"}
            >
              Score
            </button>
          </div>
        </div>
        <For each={results()}>
          {(r, i) => <ResultCard result={r} rank={i() + 1} terms={termsOf(store.query())} />}
        </For>
      </div>
    </Show>
  );
}

function FreshnessBar(props: { days: number; disabled?: boolean; onReindex: () => void }) {
  return (
    <div class="mt-3 flex items-center gap-3 rounded-card border border-line bg-surface py-2 pl-3 pr-2">
      <span class="text-[13px] font-medium leading-5 text-muted">
        Last indexed {props.days} day{props.days === 1 ? "" : "s"} ago
      </span>
      <Button
        size="sm"
        variant="outline"
        class="ml-auto"
        disabled={props.disabled}
        onClick={props.onReindex}
      >
        <ReindexIcon size={14} />
        Re-index
      </Button>
    </div>
  );
}
