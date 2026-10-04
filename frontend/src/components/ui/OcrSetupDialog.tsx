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
        class="fixed inset-0 z-50 flex items-center justify-center bg-ink/30 p-4"
        role="dialog"
        aria-modal="true"
        aria-label="Read scanned PDFs"
        onClick={props.onDismiss}
      >
        <div
          class="od-scale-in w-104 rounded-[12px] border border-line bg-paper p-[22px] shadow-overlay"
          onClick={(e) => e.stopPropagation()}
        >
          <p class="font-mono text-[11px] uppercase tracking-[0.1em] text-muted">Optional</p>
          <h2 class="mt-1 text-[20px] font-semibold leading-tight tracking-[-0.015em] text-ink">Read scanned PDFs</h2>
          <p class="mt-2 text-[13px] font-medium leading-[1.7] text-ink-soft">
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
            <p class="text-[12px] font-medium leading-4 text-muted">
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
