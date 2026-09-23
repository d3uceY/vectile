import { Show } from "solid-js";
import type { OCRState } from "../../lib/types";
import { OcrCard } from "./OcrCard";
import { Button } from "./primitives";

/**
 * The one-time OCR offer, shown after the first-run model dialog. The plugin is
 * optional, so this says so and stays out of the way: dismissing it writes a
 * flag and it never returns.
 */
export function OcrSetupDialog(props: {
  open: boolean;
  state: OCRState | null;
  onInstall: () => void;
  onCancel: () => void;
  onDismiss: () => void;
}) {
  return (
    <Show when={props.open}>
      <div
        class="fixed inset-0 z-50 flex items-center justify-center bg-ink/20 p-4"
        role="dialog"
        aria-modal="true"
        aria-label="Read scanned PDFs"
        onClick={props.onDismiss}
      >
        <div class="sheet w-104 p-6 shadow-pop" onClick={(e) => e.stopPropagation()}>
          <p class="data text-[11px] uppercase tracking-wide text-faint">Optional</p>
          <h2 class="title mt-1 text-[20px] leading-tight text-ink">Read scanned PDFs</h2>
          <p class="mt-2 text-[13.5px] leading-6 text-ink-soft">
            Some PDFs are photos of pages instead of text. Nothing in them can be searched until it
            is read as text, which is what this does.
          </p>

          <div class="mt-4">
            <OcrCard
              state={props.state}
              onInstall={props.onInstall}
              onCancel={props.onCancel}
            />
          </div>

          <div class="mt-5 flex items-center justify-between gap-3">
            <p class="text-[12px] leading-4 text-muted">
              You can install this later from Settings.
            </p>
            <Button variant="ghost" onClick={props.onDismiss}>
              Not now
            </Button>
          </div>
        </div>
      </div>
    </Show>
  );
}
