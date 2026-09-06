import { createEffect, createSignal, createUniqueId, For, onCleanup, Show, type JSX } from "solid-js";
import { Portal } from "solid-js/web";
import type { ModelState } from "../../lib/types";
import { ChevronDown, CloseIcon, InfoIcon } from "./icons";

/* ---------------- Button ---------------- */

type ButtonProps = JSX.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "outline" | "ghost" | "quiet" | "danger";
  size?: "sm" | "md";
};

export function Button(props: ButtonProps) {
  const variant = props.variant ?? "primary";
  const size = props.size ?? "md";
  const cls = `inline-flex items-center justify-center gap-2 rounded-control font-medium transition-all duration-150 ease-snappy select-none active:scale-[0.98] disabled:opacity-45 disabled:pointer-events-none ${
    size === "sm" ? "h-8 px-3 text-[13px]" : "h-9.5 px-4 text-sm"
  } ${
    variant === "primary"
      ? "bg-leaf-deep text-white hover:bg-leaf shadow-[0_1px_2px_rgb(21_112_62/0.3)]"
      : variant === "outline"
        ? "border border-line-strong bg-paper text-ink-soft hover:border-indigo/50 hover:text-indigo-deep"
        : variant === "ghost"
          ? "text-ink-soft hover:bg-indigo-soft hover:text-indigo-deep"
          : variant === "danger"
            ? "border border-danger/40 bg-paper text-danger hover:border-danger hover:bg-danger-soft"
            : "text-faint hover:text-ink"
  } ${props.class ?? ""}`;
  return (
    <button
      {...props}
      class={cls}
      type={props.type ?? "button"}
      aria-pressed={props["aria-pressed"]}
    >
      {props.children}
    </button>
  );
}

/* ---------------- Kbd ---------------- */

export function Kbd(props: { children: JSX.Element; class?: string }) {
  return (
    <kbd
      class={`inline-flex h-5 min-w-5 items-center justify-center rounded-[5px] border border-line-strong bg-surface px-1 font-mono text-[11px] leading-none text-muted ${props.class ?? ""}`}
    >
      {props.children}
    </kbd>
  );
}

/* ---------------- Chip ---------------- */

export function Chip(props: {
  children: JSX.Element;
  tone?: "neutral" | "mint" | "leaf" | "code" | "amber" | "indigo";
  class?: string;
}) {
  const tone = props.tone ?? "neutral";
  const cls =
    tone === "mint"
      ? "bg-surface-2 text-muted"
      : tone === "leaf"
        ? "bg-leaf text-white"
        : tone === "code"
          ? "bg-paper text-muted font-mono"
          : tone === "amber"
            ? "bg-amber-soft text-amber-deep"
            : tone === "indigo"
              ? "bg-indigo-soft text-indigo-deep"
              : "bg-paper text-muted";
  return (
    <span
      class={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11.5px] font-medium leading-4 ${cls} ${props.class ?? ""}`}
    >
      {props.children}
    </span>
  );
}

/* ---------------- Select (custom, accessible combobox) ---------------- */

type SelectProps = {
  label?: string;
  options: { value: string; label: string }[];
  value: string;
  onChange?: (value: string) => void;
  "aria-label"?: string;
  class?: string;
};

/* A custom listbox styled to match the app's form controls. The trigger keeps
   focus (combobox pattern) and the menu is rendered in a fixed-position portal
   so it escapes any overflow-hidden / scrollable ancestor that would clip it.
   Keyboard: type-ahead is omitted, but Arrow/Home/End/Enter/Space/Escape/Tab
   all work, and the active option is reported via aria-activedescendant. */
export function Select(props: SelectProps) {
  const listId = createUniqueId();
  const [open, setOpen] = createSignal(false);
  const [activeIdx, setActiveIdx] = createSignal(-1);
  const [rect, setRect] = createSignal<DOMRect | null>(null);
  let trigger: HTMLButtonElement | undefined;
  let list: HTMLUListElement | undefined;

  const selected = () => props.options.find((o) => o.value === props.value);
  const selectedLabel = () => selected()?.label ?? props.value ?? "";

  const openMenu = () => {
    if (trigger) setRect(trigger.getBoundingClientRect());
    setActiveIdx(Math.max(0, props.options.findIndex((o) => o.value === props.value)));
    setOpen(true);
  };

  const closeMenu = (focusTrigger = true) => {
    setOpen(false);
    setActiveIdx(-1);
    if (focusTrigger) trigger?.focus();
  };

  const pick = (value: string) => {
    props.onChange?.(value);
    closeMenu();
  };

  const onTriggerKey = (e: KeyboardEvent) => {
    const last = props.options.length - 1;
    if (!open()) {
      if (e.key === "Enter" || e.key === " " || e.key === "ArrowDown" || e.key === "ArrowUp") {
        e.preventDefault();
        openMenu();
        if (e.key === "ArrowUp") setActiveIdx(last);
      }
      return;
    }
    if (e.key === "Escape") {
      e.preventDefault();
      closeMenu();
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIdx((i) => Math.min(last, i + 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIdx((i) => Math.max(0, i - 1));
    } else if (e.key === "Home") {
      e.preventDefault();
      setActiveIdx(0);
    } else if (e.key === "End") {
      e.preventDefault();
      setActiveIdx(last);
    } else if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      const o = props.options[activeIdx()];
      if (o) pick(o.value);
    } else if (e.key === "Tab") {
      closeMenu(false);
    }
  };

  const onDocMouseDown = (e: MouseEvent) => {
    if (!open()) return;
    if (trigger?.contains(e.target as Node)) return;
    if (list?.contains(e.target as Node)) return;
    closeMenu(false);
  };

  createEffect(() => {
    if (open()) document.addEventListener("mousedown", onDocMouseDown);
    else document.removeEventListener("mousedown", onDocMouseDown);
  });
  onCleanup(() => document.removeEventListener("mousedown", onDocMouseDown));

  createEffect(() => {
    if (!open()) return;
    const el = list?.children[activeIdx()] as HTMLElement | undefined;
    el?.scrollIntoView({ block: "nearest" });
  });

  return (
    <span class={`inline-flex items-center gap-2 text-[13px] text-muted ${props.class ?? ""}`}>
      {props.label && <span class="whitespace-nowrap">{props.label}</span>}
      <span class="relative inline-flex min-w-0 flex-1">
        <button
          ref={trigger}
          type="button"
          class="inline-flex h-8 min-w-0 flex-1 items-center rounded-control border border-line bg-paper pl-3 pr-8 text-[13px] text-ink transition-colors hover:border-line-strong focus:border-indigo"
          aria-label={props["aria-label"]}
          aria-haspopup="listbox"
          aria-expanded={open()}
          aria-activedescendant={open() && activeIdx() >= 0 ? `${listId}-option-${activeIdx()}` : undefined}
          onClick={openMenu}
          onKeyDown={onTriggerKey}
        >
          <span class="min-w-0 truncate">{selectedLabel()}</span>
        </button>
        <ChevronDown size={14} class="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-faint" />
      </span>
      <Show when={open()}>
        <Portal>
          <ul
            ref={list}
            role="listbox"
            class="scroll-quiet fixed z-50 max-h-56 overflow-y-auto rounded-control border border-line bg-paper py-1 shadow-pop"
            style={
              {
                top: `${(rect()?.bottom ?? 0) + 4}px`,
                left: `${rect()?.left ?? 0}px`,
                minWidth: `${rect()?.width ?? 0}px`,
              } as JSX.CSSProperties
            }
          >
            <For each={props.options}>
              {(o, i) => (
                <li
                  id={`${listId}-option-${i()}`}
                  role="option"
                  aria-selected={o.value === props.value}
                  class={`flex cursor-pointer items-center gap-2 px-3 py-1.5 text-[13px] ${
                    i() === activeIdx()
                      ? "bg-indigo-mist text-ink"
                      : o.value === props.value
                        ? "text-ink"
                        : "text-ink-soft"
                  }`}
                  onMouseEnter={() => setActiveIdx(i())}
                  onMouseDown={(e) => {
                    e.preventDefault();
                    pick(o.value);
                  }}
                >
                  <span class="min-w-0 truncate">{o.label}</span>
                </li>
              )}
            </For>
          </ul>
        </Portal>
      </Show>
    </span>
  );
}

/* ---------------- Toggle ---------------- */

export function Switch(props: {
  checked: boolean;
  onChange: (v: boolean) => void;
  label: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={props.checked}
      aria-label={props.label}
      onClick={() => props.onChange(!props.checked)}
      class={`relative h-6 w-10 shrink-0 rounded-full border transition-colors duration-150 ease-snappy focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-indigo/60 focus-visible:ring-offset-2 focus-visible:ring-offset-paper ${
        props.checked ? "border-leaf bg-leaf" : "border-line-strong bg-surface"
      }`}
    >
      <span
        class={`absolute left-0.5 top-0.5 h-4.5 w-4.5 rounded-full bg-white transition-transform duration-150 ease-snappy ${
          props.checked ? "translate-x-4" : ""
        }`}
      />
    </button>
  );
}

/* ---------------- Toggle (labeled row) ---------------- */

export function Toggle(props: {
  checked: boolean;
  onChange: (v: boolean) => void;
  label: string;
  description?: string;
  hint?: string;
}) {
  return (
    <div class="flex items-center justify-between gap-4 py-2">
      <span>
        <span class="flex items-center gap-1.5">
          <span class="block text-sm text-ink">{props.label}</span>
          {props.hint && <InfoTip text={props.hint} />}
        </span>
        {props.description && <span class="block text-[13px] text-muted">{props.description}</span>}
      </span>
      <Switch checked={props.checked} onChange={props.onChange} label={props.label} />
    </div>
  );
}

/* ---------------- InfoTip (hover explanation) ---------------- */
export function InfoTip(props: { text: string; class?: string }) {
  let ref: HTMLButtonElement | undefined;
  const [pos, setPos] = createSignal<{ x: number; y: number } | null>(null);
  let timer: ReturnType<typeof setTimeout> | undefined;

  const open = () => {
    const r = ref?.getBoundingClientRect();
    if (!r) return;
    const pad = 8;
    const est = 170; // rough bubble height; flips above when tight on space
    const below = r.bottom + pad;
    const y = below + est <= window.innerHeight ? below : Math.max(pad, r.top - pad - est);
    const x = Math.min(Math.max(pad, r.left), window.innerWidth - 316);
    setPos({ x, y });
    window.addEventListener("scroll", hide, true);
  };
  const show = () => {
    if (pos()) return;
    clearTimeout(timer);
    timer = setTimeout(open, 150);
  };
  const hide = () => {
    clearTimeout(timer);
    setPos(null);
    window.removeEventListener("scroll", hide, true);
  };
  onCleanup(() => {
    clearTimeout(timer);
    window.removeEventListener("scroll", hide, true);
  });

  return (
    <>
      <button
        ref={ref}
        type="button"
        class={`inline-flex shrink-0 items-center justify-center rounded-full text-faint transition-colors hover:text-indigo focus-visible:text-indigo ${props.class ?? ""}`}
        aria-label="What this setting does"
        onMouseEnter={show}
        onMouseLeave={hide}
        onFocus={show}
        onBlur={hide}
      >
        <InfoIcon size={15} />
      </button>
      <Show when={pos()}>
        <Portal mount={document.body}>
          <div
            role="tooltip"
            class="pointer-events-none fixed z-100 w-75 rounded-control border border-line bg-paper p-3 text-[13px] leading-5 text-ink shadow-pop"
            style={{ left: `${pos()!.x}px`, top: `${pos()!.y}px` }}
          >
            {props.text}
          </div>
        </Portal>
      </Show>
    </>
  );
}

/* ---------------- StatusPill (in-process model engine) ---------------- */

/** Shared model-state → dot/text mapping, used by StatusPill and the sidebar plate. */
export const modelStateMeta: Record<ModelState, { dot: string; text: string }> = {
  loaded: { dot: "bg-leaf", text: "text-leaf-deep" },
  idle: { dot: "bg-faint", text: "text-muted" },
  failed: { dot: "bg-danger", text: "text-danger" },
};

const modelLabel: Record<ModelState, string> = {
  loaded: "model loaded",
  idle: "model idle",
  failed: "model failed",
};

export function StatusPill(props: {
  state: ModelState;
  name?: string;
  compact?: boolean;
}) {
  const m = () => modelStateMeta[props.state];
  const tip = props.name ? `${modelLabel[props.state]} · ${props.name}` : modelLabel[props.state];
  const dot = (
    <span class="relative flex h-2 w-2">
      <span class={`h-2 w-2 rounded-full ${m().dot} ${props.state === "loaded" ? "pulse-dot" : ""}`} />
    </span>
  );
  if (props.compact) {
    return (
      <span class="inline-flex" title={tip}>
        {dot}
      </span>
    );
  }
  return (
    <span
      class={`inline-flex items-center gap-2 rounded-full border border-line bg-paper px-3 py-1 ${m().text}`}
      title={tip}
    >
      {dot}
      <span class="data">{modelLabel[props.state]}</span>
      {props.name && <span class="data text-muted">· {props.name}</span>}
    </span>
  );
}

/* ---------------- EmptyState ---------------- */

export function EmptyState(props: {
  icon?: JSX.Element;
  title: string;
  note?: string;
  children?: JSX.Element;
}) {
  return (
    <div class="flex flex-col items-center justify-center px-6 py-16 text-center">
      {props.icon && (
          <div class="mb-4 flex h-12 w-12 items-center justify-center rounded-card border border-line bg-surface text-indigo">
          {props.icon}
        </div>
      )}
      <h3 class="title text-lg tracking-[-0.01em] text-ink">{props.title}</h3>
      {props.note && <p class="note mt-2 max-w-[34ch] text-[15.5px] leading-6 text-muted">{props.note}</p>}
      {props.children && <div class="mt-5">{props.children}</div>}
    </div>
  );
}

/* ---------------- Skeleton ---------------- */

export function Skeleton(props: { class?: string }) {
  return (
    <div
      class={`animate-pulse rounded-md bg-surface ${props.class ?? "h-4 w-full"}`}
      aria-hidden="true"
    />
  );
}

/* ---------------- View heading ---------------- */

export function ViewHeading(props: { title: string; note?: string; children?: JSX.Element }) {
  return (
    <div class="mb-6 flex flex-wrap items-end justify-between gap-x-6 gap-y-3">
      <div>
        <h1 class="title text-[28px] leading-tight tracking-[-0.025em] text-ink">
          {props.title}
        </h1>
        {props.note && <p class="note mt-1.5 text-[15.5px] text-muted">{props.note}</p>}
      </div>
      {props.children && <div class="flex items-center gap-2">{props.children}</div>}
    </div>
  );
}

/* ---------------- Toast ---------------- */

export function ToastStack(props: {
  toasts: { id: number; message: string; tone: "neutral" | "success" | "danger" }[];
  onDismiss: (id: number) => void;
}) {
  return (
    <div class="pointer-events-none fixed bottom-4 right-4 z-50 flex w-80 flex-col gap-2">
      <For each={props.toasts}>
        {(t) => (
          <div
            class={`pointer-events-auto flex items-start gap-3 rounded-card border bg-surface px-4 py-3 shadow-pop ${
              t.tone === "danger" ? "border-danger/40" : "border-line"
            }`}
          >
            <span
              class={`mt-1 h-2 w-2 shrink-0 rounded-full ${
                t.tone === "success" ? "bg-leaf" : t.tone === "danger" ? "bg-danger" : "bg-faint"
              }`}
            />
            <p class="flex-1 text-[13px] leading-5 text-ink-soft">{t.message}</p>
            <button
              class="text-faint hover:text-ink"
              onClick={() => props.onDismiss(t.id)}
              aria-label="Dismiss"
            >
              <CloseIcon size={14} />
            </button>
          </div>
        )}
      </For>
    </div>
  );
}

/* ---------------- ConfirmDialog ---------------- */

export function ConfirmDialog(props: {
  open: boolean;
  title: string;
  body: JSX.Element;
  confirmLabel?: string;
  cancelLabel?: string;
  busyLabel?: string;
  busy?: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  return (
    <Show when={props.open}>
      <div
        class="fixed inset-0 z-50 flex items-center justify-center bg-ink/20 p-4"
        onClick={props.busy ? undefined : props.onCancel}
      >
        <div class="sheet w-[22rem] p-5 shadow-pop" onClick={(e) => e.stopPropagation()}>
          <h3 class="title text-[15px] tracking-[-0.01em] text-ink">{props.title}</h3>
          <div class="read mt-2 text-[13.5px] leading-5 text-muted">{props.body}</div>
          <div class="mt-4 flex justify-end gap-2">
            <Button size="sm" variant="ghost" onClick={props.onCancel} disabled={props.busy}>
              {props.cancelLabel ?? "Keep"}
            </Button>
            <button
              type="button"
              onClick={props.onConfirm}
              disabled={props.busy}
              class="inline-flex h-8 select-none items-center justify-center gap-2 rounded-control bg-danger px-3 text-[13px] font-medium text-white transition-all duration-150 ease-snappy active:scale-[0.98] disabled:opacity-45 disabled:pointer-events-none"
            >
              {props.busy ? (props.busyLabel ?? "Deleting…") : (props.confirmLabel ?? "Delete")}
            </button>
          </div>
        </div>
      </div>
    </Show>
  );
}
