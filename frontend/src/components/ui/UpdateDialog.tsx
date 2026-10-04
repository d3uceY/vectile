import { Show } from "solid-js";
import { DOWNLOAD_URL, openExternal } from "../../lib/update";
import { Button } from "./primitives";

/** Update prompt shown once per launch when a NEWER STABLE release exists
    (beta/rc/etc. never triggers it; see isNewer in lib/update). Matches the
    notebook world: a paper sheet, serif title, a mint "latest" plate, and one
    clear primary action that opens the latest release page. */
export function UpdateDialog(props: {
  open: boolean;
  latest: string;
  current: string;
  onDismiss: () => void;
}) {
  return (
    <Show when={props.open}>
      <div
        class="fixed inset-0 z-50 flex items-center justify-center bg-ink/30 p-4"
        role="dialog"
        aria-modal="true"
        aria-label="Update available"
        onClick={props.onDismiss}
      >
        <div
          class="od-scale-in w-[23rem] rounded-[12px] border border-line bg-paper p-[22px] shadow-overlay"
          onClick={(e) => e.stopPropagation()}
        >
          <p class="flex items-center gap-2 font-mono text-[11px] font-semibold uppercase tracking-[0.12em] text-amber-deep">
            <span class="h-1.5 w-1.5 rounded-full bg-amber" aria-hidden="true" />
            update
          </p>
          <h3 class="mt-2 text-[17px] font-semibold tracking-[-0.01em] text-ink">A new version is here</h3>
          <p class="mt-2.5 text-[13px] font-medium leading-[1.7] text-muted">
            <span class="font-semibold text-ink">{props.latest}</span> is available. You're on{" "}
            <span class="font-semibold text-ink">{props.current}</span>.
          </p>

          <div class="mt-4 grid grid-cols-2 gap-px overflow-hidden rounded-[8px] border border-line bg-line">
            <div class="bg-surface-2 px-3.5 py-2.5">
              <p class="font-mono text-[10.5px] uppercase tracking-[0.1em] text-muted">installed</p>
              <p class="mt-1 font-mono text-[12px] text-muted">{props.current}</p>
            </div>
            <div class="bg-mint px-3.5 py-2.5">
              <p class="font-mono text-[10.5px] uppercase tracking-[0.1em] text-leaf-deep">latest</p>
              <p class="mt-1 font-mono text-[12px] font-semibold text-leaf-deep">{props.latest}</p>
            </div>
          </div>

          <div class="mt-5 flex items-center justify-end gap-2">
            <Button size="sm" variant="ghost" onClick={props.onDismiss}>
              Not now
            </Button>
            <Button
              size="sm"
              autofocus
              onClick={() => {
                openExternal(DOWNLOAD_URL);
                props.onDismiss();
              }}
            >
              Download update
            </Button>
          </div>
        </div>
      </div>
    </Show>
  );
}
