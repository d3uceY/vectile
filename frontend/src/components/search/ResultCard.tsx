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
            <mark class="rounded-sm bg-highlighter px-px text-ink">{p.text}</mark>
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
      class={`rounded-card border bg-surface px-4 py-4 shadow-xs transition-[border-color,box-shadow,transform] duration-150 ease-snappy hover:-translate-y-0.5 hover:border-line-strong hover:shadow-card ${
        open() ? "border-line-strong shadow-card" : "border-line"
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
        <h3 class="min-w-0 flex-1 text-[14px] font-semibold leading-[1.4] tracking-[-0.01em] text-ink">
          <Highlighted text={r().title} terms={props.terms} />
        </h3>
        <span class="mt-0.5 shrink-0 font-mono text-[12px] font-semibold text-leaf-deep">{score()}</span>
      </button>

      <p class="read mt-1.5 line-clamp-3 text-[13px] leading-[1.7] text-muted">
        <Highlighted text={r().content} terms={props.terms} />
      </p>

      <Show when={open()}>
        <div class="mt-3 rounded-[8px] border border-line bg-paper p-4">
          <p class="read max-h-48 overflow-y-auto whitespace-pre-wrap text-[13px] leading-[1.7] text-ink-soft">
            {r().content}
          </p>
          <div class="mt-3 flex flex-wrap items-center gap-x-3 gap-y-2">
            <Show when={meta().length > 0}>
              <p class="font-mono text-[12px] text-muted">{meta().join(" · ")}</p>
            </Show>
            <span class="ml-auto flex items-center gap-1.5">
              <button
                type="button"
                class="inline-flex h-8 items-center gap-1.5 rounded-control border border-line-control bg-paper px-2.5 text-[12px] font-semibold text-ink-soft transition-colors duration-100 ease-snappy hover:border-line-strong hover:bg-surface-2 hover:text-ink"
                onClick={() => void onOpen()}
              >
                <FolderOpenIcon size={14} class="text-leaf-deep" />
                Open file
              </button>
              <button
                type="button"
                class="flex h-8 w-8 items-center justify-center rounded-control border border-line-control bg-paper text-muted transition-colors duration-100 ease-snappy hover:border-line-strong hover:bg-surface-2 hover:text-ink"
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
        <Chip tone="mint" class="min-w-0">
          <span class="truncate">{r().collection}</span>
        </Chip>
        <span class="shrink-0 font-mono text-[12px] text-muted">{r().sourceType}</span>
        <span class="mx-0.5 h-3 w-px shrink-0 bg-line-strong" aria-hidden="true" />
        <span class="truncate font-mono text-[12px] text-muted">{r().sourcePath}</span>
        <button
          type="button"
          class="ml-auto flex shrink-0 items-center gap-1 rounded-full px-2.5 py-1 text-[12px] font-semibold text-muted transition-colors duration-100 ease-snappy hover:bg-surface-2 hover:text-ink"
          onClick={() => setOpen((v) => !v)}
          aria-expanded={open()}
        >
          <span>{open() ? "Close" : "Read full passage"}</span>
          <ChevronDown
            size={13}
            class={`transition-transform duration-100 ease-snappy ${open() ? "rotate-180" : ""}`}
          />
        </button>
      </div>
    </article>
  );
}
