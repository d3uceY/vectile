import { Show, type JSX } from "solid-js";

/**
 * Shared determinate bar used by the indexing and model-download loaders.
 * The fill caps at 96% until the backend wraps up (rename/register), and a
 * "Preparing" state shows when the total isn't known yet. Content before the
 * Cancel button is supplied by the caller.
 */
export function ProgressBar(props: {
  label: string;
  percent: number;
  preparing: boolean;
  children?: JSX.Element;
  onCancel?: () => void;
  cancelling?: boolean;
}) {
  return (
    <div>
      <div
        class="index-progress__track"
        role="progressbar"
        aria-label={props.label}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={props.preparing ? undefined : Math.round(props.percent)}
      >
        <div
          class="index-progress__fill"
          classList={{ "index-progress__fill--preparing": props.preparing }}
          style={{ width: props.preparing ? "36%" : `${props.percent}%` }}
        >
          <span class="index-progress__shine" aria-hidden="true" />
        </div>
      </div>
      <div class="mt-1.5 flex items-center gap-2">
        {props.children}
        <Show when={props.onCancel}>
          <button
            type="button"
            onClick={props.onCancel}
            disabled={props.cancelling}
            class="ml-auto inline-flex h-7 shrink-0 items-center rounded-[4px] px-2.5 text-[12px] font-medium text-danger transition-colors duration-150 hover:bg-danger-soft disabled:cursor-default disabled:text-muted disabled:hover:bg-transparent"
          >
            {props.cancelling ? "Cancelling…" : "Cancel"}
          </button>
        </Show>
      </div>
    </div>
  );
}
