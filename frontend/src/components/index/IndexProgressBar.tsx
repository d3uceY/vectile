import { Show } from "solid-js";
import { ProgressBar } from "../ui/ProgressBar";

/**
 * Inline indexing loader shown on the collection/dir row being indexed.
 * Driven by real per-file events (current/total) but shaped to feel like a
 * "pushing" loader: the fill caps at 96% and a leaf-green shine sweeps across
 * it (see ProgressBar).
 */
export function IndexProgressBar(props: {
  collection: string;
  current: number;
  total: number;
  file?: string;
  onCancel?: () => void;
}) {
  const pct = () => {
    if (props.total <= 0) return 0;
    const raw = (props.current / props.total) * 100;
    return Math.min(96, Math.round(raw * 10) / 10); 
  };

  const preparing = () => props.total <= 0;

  return (
    <ProgressBar label={`Indexing ${props.collection}`} percent={pct()} preparing={preparing()} onCancel={props.onCancel}>
      <span class="truncate font-mono text-[12px] text-muted">{props.file ?? ""}</span>
      <Show when={!preparing()}>
        <span class="shrink-0 font-mono text-[12px] text-leaf-deep">
          {props.current}/{props.total}
        </span>
      </Show>
    </ProgressBar>
  );
}
