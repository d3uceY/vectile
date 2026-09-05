import { createSignal, For, Show, onCleanup, type JSX } from "solid-js";
import { useAppStore } from "../../lib/store";
import { daysSince } from "../../lib/time";
import type { SearchFilters } from "../../lib/types";
import { exampleQueries, termsOf } from "../../lib/mock";
import { SearchIcon, CloseIcon, BoltIcon } from "../ui/icons";
import { EmptyState, Button, Kbd, Select, Skeleton } from "../ui/primitives";
import { GridPattern } from "../ui/patterns";
import { ResultCard } from "./ResultCard";

const typeOptions = [
  { value: "", label: "Any type" },
  { value: "markdown", label: "Markdown" },
  { value: "email", label: "Email" },
  { value: "pdf", label: "PDF" },
  { value: "epub", label: "EPUB" },
  { value: "plaintext", label: "Plain text" }, // must match backend source_type
  { value: "code", label: "Code" },
  { value: "rss", label: "RSS" },
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
          class={`sheet flex items-center gap-3 px-4 transition-shadow duration-150 ease-snappy ${
            value() ? "shadow-card" : ""
          }`}
        >
          <SearchIcon size={19} class="shrink-0 text-leaf" />
          <input
            id="search-input"
            ref={(el) => store.registerSearchInput(el)}
            class="h-13 w-full bg-transparent text-[15.5px] text-ink outline-none placeholder:text-faint"
            placeholder="Search your notes, books, email, and code"
            value={value()}
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
              class="shrink-0 rounded-full p-1 text-faint transition-colors hover:bg-surface hover:text-ink"
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
            class={`inline-flex h-8 items-center gap-1.5 rounded-control border px-3 text-[13px] transition-colors ${
              showAdvanced()
                ? "border-leaf/50 bg-mint-strong text-leaf-deep"
                : "border-line bg-paper text-ink-soft hover:border-line-strong"
            }`}
            onClick={() => setShowAdvanced((v) => !v)}
            aria-expanded={showAdvanced()}
          >
            Filters
            <Show when={anyFilter()}>
              <span class="h-1.5 w-1.5 rounded-full bg-leaf" />
            </Show>
          </button>
        </div>

        {/* Advanced filters */}
        <Show when={showAdvanced()}>
          <div class="mt-3 grid grid-cols-2 gap-3 rounded-card border border-line bg-paper/80 p-4 md:grid-cols-4">
            <FilterField label="Path contains">
              <input
                class="h-8 w-full rounded-control border border-line bg-paper px-3 text-[13px] outline-none placeholder:text-faint focus:border-leaf"
                placeholder="e.g. rustyquill"
                value={store.filters().path ?? ""}
                onInput={(e) => applyFilter({ path: e.currentTarget.value }, true)}
              />
            </FilterField>
            <FilterField label="Sender / author">
              <input
                class="h-8 w-full rounded-control border border-line bg-paper px-3 text-[13px] outline-none placeholder:text-faint focus:border-leaf"
                placeholder="e.g. orders@…"
                value={store.filters().sender ?? ""}
                onInput={(e) => applyFilter({ sender: e.currentTarget.value }, true)}
              />
            </FilterField>
            <FilterField label="From">
              <input
                type="date"
                class="h-8 w-full rounded-control border border-line bg-paper px-2 text-[13px] outline-none focus:border-leaf"
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
          <FreshnessBar days={freshnessDays()!} onReindex={() => void store.startIndexAll()} />
        </Show>
      </div>

      {/* Results area: the view's own scroll region */}
      <div class="scroll-quiet relative mt-6 min-h-0 flex-1 overflow-y-auto">
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
      </div>
    </div>
  );
}

function FilterField(props: { label: string; children: JSX.Element }) {
  return (
    <label class="flex flex-col gap-1.5">
      <span class="text-[11.5px] font-medium uppercase tracking-[0.08em] text-muted">
        {props.label}
      </span>
      {props.children}
    </label>
  );
}

function IdleState(props: { onPick: (q: string) => void }) {
  return (
    <div class="relative flex h-full flex-col items-center justify-center">
      <div class="pointer-events-none absolute inset-0 text-leaf/[0.06]">
        <GridPattern width={32} height={32} />
      </div>
      <div class="relative flex flex-col items-center text-center">
        <div class="mb-4 flex h-12 w-12 items-center justify-center rounded-card border border-line bg-surface text-leaf shadow-card">
          <BoltIcon size={20} />
        </div>
        <h2 class="title text-[22px] tracking-[-0.02em] text-ink">
          Ask your library
        </h2>
        <p class="note mt-2 max-w-[36ch] text-[15.5px] leading-6 text-muted">
          Searches meaning and exact words together.
        </p>
        <div class="mt-6 flex max-w-md flex-wrap items-center justify-center gap-2">
          <For each={exampleQueries}>
            {(q) => (
              <button
                class="rounded-full border border-line bg-paper px-3.5 py-1.5 text-[13px] text-ink-soft transition-all duration-150 ease-snappy hover:border-leaf/50 hover:text-leaf-deep active:scale-[0.98]"
                onClick={() => props.onPick(q)}
              >
                {q}
              </button>
            )}
          </For>
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
          <div class="sheet p-5">
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
          note="Try fewer words or drop a filter."
        />
      }
    >
      <div class="enter-stagger space-y-3">
        <div class="flex items-center justify-between gap-3">
          <p class="data text-muted">
            {results().length} result{results().length === 1 ? "" : "s"} · hybrid ranked
          </p>
          <div
            class="flex items-center rounded-full border border-line bg-paper p-0.5"
            role="group"
            aria-label="Result score display"
          >
            <button
              type="button"
              class={`h-6 rounded-full px-2.5 text-[11.5px] font-medium transition-colors duration-150 ease-snappy ${
                store.scoreDisplay() === "rank" ? "bg-mint-strong text-leaf-deep" : "text-muted hover:text-ink"
              }`}
              onClick={() => store.setScoreDisplay("rank")}
              aria-pressed={store.scoreDisplay() === "rank"}
            >
              # rank
            </button>
            <button
              type="button"
              class={`h-6 rounded-full px-2.5 text-[11.5px] font-medium transition-colors duration-150 ease-snappy ${
                store.scoreDisplay() === "percent" ? "bg-mint-strong text-leaf-deep" : "text-muted hover:text-ink"
              }`}
              onClick={() => store.setScoreDisplay("percent")}
              aria-pressed={store.scoreDisplay() === "percent"}
            >
              % score
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

function FreshnessBar(props: { days: number; onReindex: () => void }) {
  return (
    <div class="mt-3 flex items-center gap-3 rounded-control border border-line bg-surface/40 py-2 pl-3 pr-2">
      <span class="note text-[13px] leading-5 text-muted">
        Last indexed {props.days} day{props.days === 1 ? "" : "s"} ago
      </span>
      <Button size="sm" variant="outline" class="ml-auto" onClick={props.onReindex}>
        Re-index
      </Button>
    </div>
  );
}
