import { Show } from "solid-js";
import { DOWNLOAD_URL, openExternal } from "../../lib/update";
import { Button } from "./primitives";

/** Update prompt shown once per launch when a NEWER STABLE release exists
    (beta/rc/etc. never triggers it; see isNewer in lib/update). Matches the
    notebook world: a paper sheet, serif title, a mint "latest" plate, and one
    clear primary action that opens the README's Download section. */
export function UpdateDialog(props: {
  open: boolean;
  latest: string;
  current: string;
  onDismiss: () => void;
}) {
  return (
    <Show when={props.open}>
      <div
        class="fixed inset-0 z-50 flex items-center justify-center bg-ink/20 p-4"
        role="dialog"
        aria-modal="true"
        aria-label="Update available"
        onClick={props.onDismiss}
      >
        <div class="sheet w-[23rem] p-6 shadow-pop" onClick={(e) => e.stopPropagation()}>
          <p class="data flex items-center gap-2 text-[11px] font-semibold uppercase tracking-[0.14em] text-amber-deep">
            <span class="h-1.5 w-1.5 rounded-full bg-amber" aria-hidden="true" />
            update
          </p>
          <h3 class="title mt-2 text-[17px] tracking-[-0.01em] text-ink">A new version is here</h3>
          <p class="read mt-2.5 text-[13.5px] leading-5 text-muted">
            <span class="font-medium text-ink">{props.latest}</span> is available. You're on{" "}
            <span class="font-medium text-ink">{props.current}</span>.
          </p>

          <div class="mt-4 grid grid-cols-2 gap-px overflow-hidden rounded-[9px] border border-line bg-line">
            <div class="bg-paper-warm px-3.5 py-2.5">
              <p class="data text-[10.5px] uppercase tracking-[0.12em] text-muted">installed</p>
              <p class="data mt-1 text-[13px] text-muted">{props.current}</p>
            </div>
            <div class="bg-indigo-soft px-3.5 py-2.5">
              <p class="data text-[10.5px] uppercase tracking-[0.12em] text-indigo-deep">latest</p>
              <p class="data mt-1 text-[13px] font-semibold text-indigo-deep">{props.latest}</p>
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
