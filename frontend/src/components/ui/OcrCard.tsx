import { Show } from "solid-js";
import { fmtBytes } from "../../lib/format";
import type { OCRState } from "../../lib/types";
import { openExternal } from "../../lib/update";
import { DownloadProgressBar } from "./DownloadProgressBar";
import { Button } from "./primitives";

/**
 * The OCR plugin card, shared by the Settings section and the first-run setup
 * dialog so the install flow has one implementation.
 *
 * It states what the plugin is for in plain words, shows exactly where the
 * file comes from, and shows where it landed once it is installed.
 */
export function OcrCard(props: {
  state: OCRState | null;
  onInstall: () => void;
  onCancel: () => void;
  /** Omitted where removing makes no sense, such as the first-run dialog. */
  onRemove?: () => void;
}) {
  const s = () => props.state;

  const status = () => {
    const st = s();
    if (!st) return { word: "Checking", dot: "bg-faint" };
    if (st.installing) return { word: "Downloading", dot: "bg-amber" };
    if (st.installed) return { word: "Installed", dot: "bg-leaf" };
    if (!st.supported) return { word: "Not available on your system", dot: "bg-faint" };
    return { word: "Not installed", dot: "bg-amber" };
  };

  const busy = () => s()?.installing ?? false;
  const ready = () => Boolean(s()?.supported && !s()?.installed && !busy());

  return (
    <div class="rounded-control border border-line bg-paper-warm p-4">
      <div class="flex flex-wrap items-center gap-x-2.5 gap-y-1">
        <span class={`h-2 w-2 shrink-0 rounded-full ${status().dot}`} aria-hidden="true" />
        <span class="text-[13px] font-medium text-ink">{status().word}</span>
        <Show when={s()?.supported}>
          <span class="data text-[11px] text-faint">
            tesseract {s()!.version} · {s()!.platform} · {fmtBytes(s()!.sizeBytes)}
          </span>
        </Show>
      </div>

      <p class="mt-2 text-[13px] leading-5 text-muted">
        Reads PDFs that are photos of pages. PDFs with real text do not need it, and it never runs
        for a page that already came back with text.
      </p>

      <Show when={s()?.error}>
        <p class="mt-2 text-[12.5px] leading-5 text-danger">{s()!.error}</p>
      </Show>

      <Show when={busy()}>
        <div class="mt-3">
          <DownloadProgressBar progress={s()!} label="Downloading OCR" onCancel={props.onCancel} />
        </div>
      </Show>

      <Show when={!busy() && s()?.installed}>
        <p class="data mt-3 truncate text-[11px] text-faint" title={s()!.dir}>
          {s()!.dir}
        </p>
      </Show>

      <Show when={ready()}>
        <div class="mt-3 rounded-control border border-line bg-surface px-3 py-2">
          <p class="text-[10.5px] uppercase tracking-wide text-faint">Downloaded from</p>
          <p class="data mt-0.5 truncate text-[11.5px] text-ink-soft" title={s()!.downloadUrl}>
            {s()!.downloadUrl}
          </p>
        </div>
      </Show>

      <div class="mt-3 flex flex-wrap items-center gap-2">
        <Show when={ready()}>
          <Button size="sm" onClick={props.onInstall}>
            Install
          </Button>
        </Show>
        <Show when={!busy() && s()?.installed && props.onRemove}>
          <Button size="sm" variant="danger" onClick={() => props.onRemove?.()}>
            Remove
          </Button>
        </Show>
        <Show when={s()?.releaseUrl}>
          <Button size="sm" variant="ghost" onClick={() => openExternal(s()!.releaseUrl)}>
            Open release page
          </Button>
        </Show>
      </div>
    </div>
  );
}
