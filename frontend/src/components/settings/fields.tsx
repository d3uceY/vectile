import { createUniqueId, type JSX } from "solid-js";
import { InfoTip } from "../ui/primitives";

export function Section(props: {
  icon: JSX.Element;
  title: string;
  note?: string;
  children: JSX.Element;
}) {
  return (
    <section class="rounded-card border border-line-strong bg-paper shadow-card">
      <header class="flex items-start gap-3 px-5 pb-4 pt-5">
        <span class="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-control bg-indigo-soft text-indigo-deep">
          {props.icon}
        </span>
        <div class="min-w-0">
          <h2 class="title text-[18px] leading-tight tracking-[-0.01em] text-ink">{props.title}</h2>
          {props.note && <p class="note mt-1 text-[13px] leading-5 text-muted">{props.note}</p>}
        </div>
      </header>
      <div class="border-t border-line px-5 pb-5 pt-4">{props.children}</div>
    </section>
  );
}

export function SubHeading(props: { children: JSX.Element; action?: JSX.Element }) {
  return (
    <div class="mb-3 flex items-center justify-between gap-3">
      <p class="text-[13px] font-semibold tracking-[-0.01em] text-ink-soft">{props.children}</p>
      {props.action}
    </div>
  );
}

export function FieldList(props: { children: JSX.Element }) {
  return <div class="space-y-0 divide-y divide-line/70">{props.children}</div>;
}

export function NumField(props: {
  label: string;
  value: number;
  onChange: (n: number) => void;
  hint?: string;
  min?: number;
  max?: number;
  step?: number;
}) {
  const uid = createUniqueId();
  return (
    <div class="flex items-center justify-between gap-4 py-3.5">
      <span class="flex items-center gap-1.5 text-[13.5px] text-ink-soft">
        <label for={uid} class="cursor-pointer">
          {props.label}
        </label>
        {props.hint && <InfoTip text={props.hint} />}
      </span>
      <input
        id={uid}
        type="number"
        value={props.value}
        min={props.min}
        max={props.max}
        step={props.step}
        onInput={(e) => props.onChange(Number(e.currentTarget.value))}
        class="h-8 w-24 rounded-control border border-line-control bg-surface px-2 text-right text-[13px] outline-none transition-colors focus:border-leaf"
      />
    </div>
  );
}

export function RangeField(props: {
  label: string;
  value: number;
  onChange: (n: number) => void;
  hint?: string;
  min?: number;
  max?: number;
  step?: number;
  /** Render the readout (e.g. "auto" at 0 instead of "0.00"). */
  format?: (n: number) => string;
  /** A short suffix after the readout (e.g. "of 16"). */
  suffix?: string;
}) {
  const uid = createUniqueId();
  const min = props.min ?? 0;
  const max = props.max ?? 1;
  const pct = () => ((props.value - min) / (max - min)) * 100;
  return (
    <div class="flex items-center justify-between gap-4 py-3.5">
      <span class="flex items-center gap-1.5 text-[13.5px] text-ink-soft">
        <label for={uid} class="cursor-pointer">
          {props.label}
        </label>
        {props.hint && <InfoTip text={props.hint} />}
      </span>
      <span class="flex shrink-0 items-center gap-2.5">
        <input
          id={uid}
          type="range"
          value={props.value}
          min={min}
          max={max}
          step={props.step}
          onInput={(e) => props.onChange(Number(e.currentTarget.value))}
          class="slider w-40"
          style={{ "--fill": `${pct()}%` } as JSX.CSSProperties}
        />
        <span class="data w-9 shrink-0 text-right text-muted tabular-nums">
          {props.format ? props.format(props.value) : props.value.toFixed(2)}
        </span>
        {props.suffix && (
          <span class="shrink-0 text-[12px] text-muted tabular-nums">{props.suffix}</span>
        )}
      </span>
    </div>
  );
}
