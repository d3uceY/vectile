import { createSignal, For, Show } from "solid-js";
import * as api from "../../lib/api";
import { useAppStore } from "../../lib/store";
import type { SearchResult } from "../../lib/types";
import { ChevronDown, FolderIcon, FolderOpenIcon } from "../ui/icons";
import { Chip } from "../ui/primitives";

const escapeRe = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

/** Wrap query terms in a highlighter-yellow mark. */
function Highlighted(props: { text: string; terms: string[] }) {
  const parts = () => {
    if (!props.terms.length) return [{ text: props.text, hit: false }];
    const re = new RegExp(`(${props.terms.map(escapeRe).join("|")})`, "ig");
    return props.text
      .split(re)
      .map((p, i) => ({ text: p, hit: i % 2 === 1 }))
      .filter((p) => p.text.length > 0);
  };
  return (
    <>
      <For each={parts()}>
        {(p) =>
          p.hit ? (
            <mark class="rounded-sm bg-highlighter px-px text-paper">{p.text}</mark>
          ) : (
            <>{p.text}</>
          )
        }
      </For>
    </>
  );
}

function metaLine(r: SearchResult): string[] {
  const out: string[] = [];
  const m = r.metadata ?? {};
  if (typeof m.sender === "string") out.push(`from ${m.sender}`);
  if (typeof m.author === "string") out.push(m.author as string);
  if (Array.isArray(m.authors)) out.push((m.authors as string[]).join(", "));
  if (typeof m.page === "number") out.push(`p. ${m.page}`);
  if (Array.isArray(m.tags)) out.push((m.tags as string[]).map((t) => `#${t}`).join(" "));
  return out;
}

export function ResultCard(props: { result: SearchResult; terms: string[]; rank: number }) {
  const store = useAppStore();
  const [open, setOpen] = createSignal(false);
  const r = () => props.result;
  const meta = () => metaLine(r());

  const score = () =>
    store.scoreDisplay() === "rank" ? `#${props.rank}` : `${Math.round(r().score * 100)}%`;

  const onOpen = async () => {
    try {
      await api.openFile(r().sourcePath);
    } catch (err) {
      store.pushToast(`Cannot open ${r().sourcePath}: ${err}`, "danger");
    }
  };
  const onReveal = async () => {
    try {
      await api.revealInFolder(r().sourcePath);
    } catch (err) {
      store.pushToast(`Cannot reveal: ${err}`, "danger");
    }
  };

  return (
    <article
      class={`sheet px-5 py-4 transition-all duration-150 ease-snappy hover:border-leaf/40 hover:shadow-card ${
        open() ? "border-leaf/40 shadow-card" : ""
      }`}
    >
      {/* Title row is the expand toggle; open/reveal live in the expanded panel
          so no interactive element is nested inside another. */}
      <button
        type="button"
        class="flex w-full items-start justify-between gap-4 text-left"
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open()}
        aria-label={`${r().title}, ${open() ? "close" : "read"} the full passage`}
      >
        <h3 class="title min-w-0 flex-1 text-[15px] leading-6 tracking-[-0.005em] text-ink">
          <Highlighted text={r().title} terms={props.terms} />
        </h3>
        <span class="data mt-1 shrink-0 text-leaf">{score()}</span>
      </button>

      <p class="read mt-1.5 line-clamp-3 text-[14.5px] leading-[1.6] text-muted">
        <Highlighted text={r().content} terms={props.terms} />
      </p>

      <Show when={open()}>
        <div class="mt-3 rounded-[10px] border border-line bg-paper/70 p-4">
          <p class="read max-h-48 overflow-y-auto whitespace-pre-wrap text-[14.5px] leading-[1.65] text-ink-soft">
            {r().content}
          </p>
          <div class="mt-3 flex flex-wrap items-center gap-x-3 gap-y-2">
            <Show when={meta().length > 0}>
              <p class="data text-muted">{meta().join(" · ")}</p>
            </Show>
            <span class="ml-auto flex items-center gap-1.5">
              <button
                type="button"
                class="inline-flex h-8 items-center gap-1.5 rounded-control border border-line-strong bg-paper px-2.5 text-[12.5px] font-medium text-ink-soft transition-colors duration-150 ease-snappy hover:border-leaf/50 hover:text-ink"
                onClick={() => void onOpen()}
              >
                <FolderOpenIcon size={14} class="text-leaf" />
                Open file
              </button>
              <button
                type="button"
                class="flex h-8 w-8 items-center justify-center rounded-control border border-line-strong bg-paper text-muted transition-colors duration-150 ease-snappy hover:border-leaf/50 hover:text-ink"
                onClick={() => void onReveal()}
                aria-label="Reveal in folder"
                title="Reveal in folder"
              >
                <FolderIcon size={14} />
              </button>
            </span>
          </div>
        </div>
      </Show>

      <div class="mt-3 flex items-center gap-2 overflow-hidden">
        <Chip tone="mint">{r().collection}</Chip>
        <span class="data shrink-0 text-muted">{r().sourceType}</span>
        <span class="mx-0.5 h-3 w-px shrink-0 bg-line-strong" aria-hidden="true" />
        <span class="data truncate text-muted">{r().sourcePath}</span>
        <button
          type="button"
          class="ml-auto flex shrink-0 items-center gap-1 rounded-control px-2 py-1 text-[12px] font-medium text-muted transition-colors duration-150 ease-snappy hover:bg-surface hover:text-ink"
          onClick={() => setOpen((v) => !v)}
          aria-expanded={open()}
        >
          <span>{open() ? "Close" : "Read full passage"}</span>
          <ChevronDown
            size={13}
            class={`transition-transform duration-150 ease-snappy ${open() ? "rotate-180" : ""}`}
          />
        </button>
      </div>
    </article>
  );
}
