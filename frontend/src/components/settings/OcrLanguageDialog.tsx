import { createMemo, createSignal, For, Show } from "solid-js";
import { useAppStore } from "../../lib/store";
import type { OCRLanguageProgress } from "../../lib/types";
import { DownloadProgressBar } from "../ui/DownloadProgressBar";
import { CloseIcon, DownloadIcon, SearchIcon } from "../ui/icons";
import { Button, Chip } from "../ui/primitives";

/** The language list, as a searchable modal. A Select would hold the same
    rows but cannot filter: with ~120 languages, typing part of a name is how
    anyone actually finds one. */
export function OcrLanguageDialog(props: {
  open: boolean;
  /** Codes whose data is on disk already. */
  installed: string[];
  /** The one download in flight, if any. */
  download: OCRLanguageProgress | null;
  /** Adds the language and switches it on. */
  onPick: (code: string) => void;
  onCancelDownload: () => void;
  onDismiss: () => void;
}) {
  const store = useAppStore();
  const [query, setQuery] = createSignal("");

  const filtered = createMemo(() => {
    const needle = query().trim().toLowerCase();
    if (!needle) return store.ocrLanguages();
    return store
      .ocrLanguages()
      .filter((l) => l.name.toLowerCase().includes(needle) || l.code.includes(needle));
  });

  const isInstalled = (code: string) => props.installed.includes(code);
  const busy = () => Boolean(props.download);
  const downloading = (code: string) => props.download?.code === code;

  return (
    <Show when={props.open}>
      <div
        class="fixed inset-0 z-50 flex items-center justify-center bg-ink/30 p-4"
        role="dialog"
        aria-modal="true"
        aria-label="Add an OCR language"
        onClick={props.onDismiss}
      >
        <div
          class="od-scale-in flex max-h-[74vh] w-104 flex-col rounded-[12px] border border-line bg-paper p-[22px] shadow-overlay"
          onClick={(e) => e.stopPropagation()}
        >
          <h3 class="text-[17px] font-semibold tracking-[-0.01em] text-ink">Add a language</h3>
          <p class="mt-2 text-[13px] font-medium leading-[1.7] text-muted">
            Tesseract reads the languages you add here. The data downloads once and stays
            on this machine.
          </p>

          <div class="mt-3 flex items-center gap-2 rounded-[8px] border border-line-control bg-surface px-3">
            <SearchIcon size={15} class="shrink-0 text-muted" />
            <input
              class="h-9 w-full bg-transparent text-[13px] font-medium text-ink outline-none placeholder:text-faint"
              placeholder="Search languages"
              value={query()}
              onInput={(e) => setQuery(e.currentTarget.value)}
              aria-label="Search languages"
              spellcheck={false}
              autocomplete="off"
              ref={(el) => queueMicrotask(() => el.focus())}
            />
          </div>

          <div class="scroll-quiet -mr-1 mt-2 min-h-0 flex-1 overflow-y-auto pr-1">
            <ul class="divide-y divide-line">
              <For each={filtered()}>
                {(l) => (
                  <li>
                    <button
                      type="button"
                      class="flex w-full items-center gap-3 rounded-md px-1.5 py-2 text-left transition-colors duration-100 ease-snappy enabled:hover:bg-surface-2 disabled:opacity-45"
                      disabled={busy()}
                      onClick={() => props.onPick(l.code)}
                    >
                      <span class="min-w-0 flex-1">
                        <span class="block truncate text-[13px] font-semibold text-ink">
                          {l.name}
                        </span>
                        <span class="block font-mono text-[11px] text-muted">{l.code}</span>
                      </span>
                      <Show when={downloading(l.code)}>
                        <span class="shrink-0 font-mono text-[11px] text-leaf-deep">downloading</span>
                      </Show>
                      <Show when={!downloading(l.code) && isInstalled(l.code)}>
                        <Chip tone="mint">installed</Chip>
                      </Show>
                      <Show when={!downloading(l.code) && !isInstalled(l.code)}>
                        <DownloadIcon size={14} class="shrink-0 text-faint" />
                      </Show>
                    </button>
                  </li>
                )}
              </For>
            </ul>
            <Show when={filtered().length === 0}>
              <p class="py-4 text-center text-[13px] font-medium text-muted">
                No language matches that.
              </p>
            </Show>
          </div>

          <Show when={props.download}>
            {(dl) => (
              <div class="mt-3 rounded-[8px] border border-line bg-surface px-3 py-2">
                <DownloadProgressBar
                  progress={dl()}
                  label={`Downloading ${store.ocrLanguageName(dl().code)}`}
                  onCancel={props.onCancelDownload}
                />
              </div>
            )}
          </Show>

          <div class="mt-4 flex justify-end">
            <Button size="sm" variant="outline" onClick={props.onDismiss}>
              <CloseIcon size={13} />
              Close
            </Button>
          </div>
        </div>
      </div>
    </Show>
  );
}
