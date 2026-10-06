import { createEffect, createSignal, createUniqueId, For, onCleanup, Show, type JSX } from "solid-js";
import { Portal } from "solid-js/web";
import type { ModelState } from "../../lib/types";
import { CheckIcon, ChevronDown, CloseIcon, InfoIcon } from "./icons";

/* ---------------- Button ---------------- */

type ButtonProps = JSX.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "outline" | "ghost" | "quiet" | "danger";
  size?: "sm" | "md";
};

export function Button(props: ButtonProps) {
  const variant = props.variant ?? "primary";
  const size = props.size ?? "md";
  const cls = `inline-flex items-center justify-center gap-2 rounded-control font-semibold leading-none whitespace-nowrap transition-[background-color,border-color,color,box-shadow,transform] duration-100 ease-snappy select-none active:translate-y-px disabled:opacity-45 disabled:pointer-events-none ${
    size === "sm" ? "h-8 px-3 text-[13px]" : "h-9 px-4 text-[14px]"
  } ${
    variant === "primary"
      ? "rounded-full bg-accent text-accent-ink hover:bg-leaf shadow-[0_1px_2px_rgb(21_112_62/0.25)]"
      : variant === "outline"
        ? "border border-line-control bg-paper text-ink-soft hover:border-line-strong hover:bg-surface-2 hover:text-ink"
        : variant === "ghost"
          ? "text-ink-soft hover:bg-surface-2 hover:text-ink"
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
      class={`inline-flex h-5 min-w-5 items-center justify-center rounded-[4px] border border-line-strong bg-surface px-1 font-mono text-[11px] leading-none text-muted ${props.class ?? ""}`}
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
      ? "border border-line bg-surface-2 text-ink-soft"
      : tone === "leaf"
        ? "border border-transparent bg-accent text-accent-ink"
        : tone === "code"
          ? "border border-line bg-surface-2 font-mono text-muted"
          : tone === "amber"
            ? "border border-amber-warm bg-amber-soft text-amber-deep"
            : tone === "indigo"
              ? "border border-transparent bg-mint text-leaf-deep"
              : "border border-line bg-surface-2 text-muted";
  return (
    <span
      class={`inline-flex max-w-full items-center gap-1 whitespace-nowrap rounded-full px-[7px] py-[1px] text-[12px] font-semibold leading-[1.5] ${cls} ${props.class ?? ""}`}
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
  /** Muted text shown when the value matches no option (no data yet). */
  placeholder?: string;
  disabled?: boolean;
  "aria-label"?: string;
  class?: string;
};

const MENU_MAX_H = 256;
const MENU_GAP = 4;
const MENU_PAD = 8;

export function Select(props: SelectProps) {
  const listId = createUniqueId();
  const [open, setOpen] = createSignal(false);
  const [activeIdx, setActiveIdx] = createSignal(-1);
  const [menuStyle, setMenuStyle] = createSignal<JSX.CSSProperties>({});
  let trigger: HTMLButtonElement | undefined;
  let list: HTMLUListElement | undefined;
  let typed = "";
  let typedAt = 0;

  const disabled = () => props.disabled || props.options.length === 0;
  const selected = () => props.options.find((o) => o.value === props.value);
  const selectedLabel = () => selected()?.label ?? (props.value || props.placeholder || "");

  const place = () => {
    if (!trigger) return;
    const r = trigger.getBoundingClientRect();
    const rows = Math.min(props.options.length, 8);
    const estimate = Math.min(MENU_MAX_H, rows * 34 + 12);
    const below = window.innerHeight - r.bottom - MENU_GAP - MENU_PAD;
    const flip = below < estimate && r.top - MENU_GAP - MENU_PAD > below;
    const style: JSX.CSSProperties = {
      "min-width": `${r.width}px`,
      "max-width": `${Math.max(160, window.innerWidth - MENU_PAD * 2)}px`,
      left: `${Math.max(MENU_PAD, r.left)}px`,
      ...(flip
        ? { bottom: `${window.innerHeight - r.top + MENU_GAP}px`, top: "auto" }
        : { top: `${r.bottom + MENU_GAP}px`, bottom: "auto" }),
    };
    if (r.left + Math.max(r.width, 200) > window.innerWidth - MENU_PAD) {
      style.left = "auto";
      style.right = `${MENU_PAD}px`;
    }
    setMenuStyle(style);
  };

  const openMenu = () => {
    if (disabled()) return;
    place();
    setActiveIdx(Math.max(0, props.options.findIndex((o) => o.value === props.value)));
    setOpen(true);
  };

  const closeMenu = (focusTrigger = true) => {
    setOpen(false);
    setActiveIdx(-1);
    typed = "";
    if (focusTrigger) trigger?.focus();
  };

  const pick = (value: string) => {
    props.onChange?.(value);
    closeMenu();
  };

  /** Jump to the next option starting with the typed letters (600ms buffer). */
  const typeAhead = (key: string) => {
    const now = Date.now();
    typed = now - typedAt > 600 ? key : typed + key;
    typedAt = now;
    const q = typed.toLowerCase();
    const n = props.options.length;
    const start = open() ? activeIdx() : -1;
    for (let step = 1; step <= n; step++) {
      const i = (start + step) % n;
      if (props.options[i].label.toLowerCase().startsWith(q)) {
        if (!open()) openMenu();
        setActiveIdx(i);
        return true;
      }
    }
    return false;
  };

  const onTriggerKey = (e: KeyboardEvent) => {
    const last = props.options.length - 1;
    if (e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
      if (typeAhead(e.key)) e.preventDefault();
      return;
    }
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
  const onAnyScroll = (e: Event) => {
    if (list && e.target instanceof Node && list.contains(e.target)) return;
    closeMenu(false);
  };
  const onViewportChange = () => closeMenu(false);

  createEffect(() => {
    if (!open()) return;
    document.addEventListener("mousedown", onDocMouseDown);
    window.addEventListener("scroll", onAnyScroll, true);
    window.addEventListener("resize", onViewportChange);
    onCleanup(() => {
      document.removeEventListener("mousedown", onDocMouseDown);
      window.removeEventListener("scroll", onAnyScroll, true);
      window.removeEventListener("resize", onViewportChange);
    });
  });

  createEffect(() => {
    if (!open()) return;
    const el = list?.children[activeIdx()] as HTMLElement | undefined;
    el?.scrollIntoView({ block: "nearest" });
  });

  return (
    <span class={`inline-flex min-w-0 items-center gap-2 text-[13px] text-muted ${props.class ?? ""}`}>
      {props.label && <span class="whitespace-nowrap">{props.label}</span>}
      <span class="relative inline-flex min-w-0 flex-1">
        <button
          ref={trigger}
          type="button"
          class={`inline-flex h-9 min-w-0 flex-1 items-center rounded-control border bg-surface pl-2.5 pr-8 text-left text-[13px] transition-colors duration-100 ease-snappy disabled:pointer-events-none disabled:opacity-45 ${
            open() ? "border-ink" : "border-line-control hover:border-line-strong"
          } ${selected() ? "text-ink" : "text-muted"}`}
          aria-label={props["aria-label"]}
          aria-haspopup="listbox"
          aria-controls={open() ? listId : undefined}
          aria-expanded={open()}
          aria-activedescendant={open() && activeIdx() >= 0 ? `${listId}-option-${activeIdx()}` : undefined}
          disabled={disabled()}
          title={selectedLabel()}
          onClick={openMenu}
          onKeyDown={onTriggerKey}
        >
          <span class="min-w-0 truncate">{selectedLabel()}</span>
        </button>
        <ChevronDown
          size={14}
          class={`pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-faint transition-transform duration-150 ease-snappy ${
            open() ? "rotate-180" : ""
          }`}
        />
      </span>
      <Show when={open()}>
        <Portal>
          <ul
            ref={list}
            id={listId}
            role="listbox"
            class="scroll-quiet fixed z-50 max-h-[min(280px,48vh)] overflow-y-auto rounded-control border border-line-strong bg-paper p-1 shadow-pop"
            style={menuStyle()}
          >
            <For each={props.options}>
              {(o, i) => (
                <li
                  id={`${listId}-option-${i()}`}
                  role="option"
                  aria-selected={o.value === props.value}
                  title={o.label}
                  class={`flex min-h-[30px] cursor-pointer items-center gap-2 rounded-[4px] px-2 py-1.5 text-[13px] ${
                    i() === activeIdx() ? "bg-surface-2" : ""
                  } ${o.value === props.value ? "font-semibold text-ink" : "text-ink-soft"}`}
                  onMouseEnter={() => setActiveIdx(i())}
                  onMouseDown={(e) => {
                    e.preventDefault();
                    pick(o.value);
                  }}
                >
                  <span class="min-w-0 flex-1 truncate">{o.label}</span>
                  <Show when={o.value === props.value}>
                    <CheckIcon size={13} class="shrink-0 text-leaf-deep" />
                  </Show>
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
      class={`relative h-6 w-10 shrink-0 rounded-full border transition-colors duration-100 ease-snappy focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-leaf/50 focus-visible:ring-offset-2 focus-visible:ring-offset-paper ${
        props.checked ? "border-accent bg-accent" : "border-line-control bg-surface-2"
      }`}
    >
      <span
        class={`absolute left-0.5 top-0.5 h-4.5 w-4.5 rounded-full transition-transform duration-150 ease-snappy ${
          props.checked ? "translate-x-4 bg-accent-ink" : "bg-leaf"
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
          <span class="block text-[13px] font-semibold text-ink">{props.label}</span>
          {props.hint && <InfoTip text={props.hint} />}
        </span>
        {props.description && <span class="block text-[12px] font-medium text-muted">{props.description}</span>}
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
        class={`inline-flex shrink-0 items-center justify-center rounded-full text-faint transition-colors duration-100 hover:text-leaf-deep focus-visible:text-leaf-deep ${props.class ?? ""}`}
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
            class="pointer-events-none fixed z-100 w-[260px] rounded-control border border-line-strong bg-paper px-[9px] py-[5px] text-[12px] font-medium leading-[1.4] text-ink shadow-pop"
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
export const modelStateMeta: Record<ModelState, { dot: string; text: string; pill: string }> = {
  loaded: { dot: "bg-leaf", text: "text-leaf-deep", pill: "border-leaf/40 bg-mint" },
  idle: { dot: "bg-faint", text: "text-muted", pill: "border-line bg-surface-2" },
  failed: { dot: "bg-danger-strong", text: "text-danger", pill: "border-danger/40 bg-danger-soft" },
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
      class={`inline-flex items-center gap-1.5 rounded-full border px-[7px] py-0.5 text-[12px] font-bold uppercase tracking-[0.02em] ${m().pill} ${m().text}`}
      title={tip}
    >
      {dot}
      <span>{modelLabel[props.state]}</span>
      {props.name && (
        <span class="font-medium normal-case tracking-normal text-muted">· {props.name}</span>
      )}
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
    <div class="flex flex-col items-center justify-center rounded-card border border-dashed border-line-strong px-6 py-12 text-center">
      {props.icon && (
        <div class="mb-4 flex h-11 w-11 items-center justify-center rounded-[8px] border border-line bg-surface text-leaf-deep">
          {props.icon}
        </div>
      )}
      <h3 class="text-[15px] font-semibold tracking-[-0.01em] text-ink">{props.title}</h3>
      {props.note && (
        <p class="mt-2 max-w-[42ch] text-[13px] font-medium leading-[1.7] text-muted">{props.note}</p>
      )}
      {props.children && <div class="mt-5">{props.children}</div>}
    </div>
  );
}

/* ---------------- Skeleton ---------------- */

export function Skeleton(props: { class?: string }) {
  return (
    <div
      class={`animate-pulse rounded-md bg-line ${props.class ?? "h-4 w-full"}`}
      aria-hidden="true"
    />
  );
}

/* ---------------- View heading ---------------- */

export function ViewHeading(props: { title: string; note?: string; children?: JSX.Element }) {
  return (
    <div class="mb-6 flex flex-wrap items-end justify-between gap-x-6 gap-y-3">
      <div>
        <h1 class="text-[24px] font-semibold leading-tight tracking-[-0.02em] text-ink">
          {props.title}
        </h1>
        {props.note && <p class="mt-1.5 text-[13px] font-medium text-muted">{props.note}</p>}
      </div>
      {props.children && <div class="flex w-full items-center justify-between gap-2">{props.children}</div>}
    </div>
  );
}

/* ---------------- Toast ---------------- */

export function ToastStack(props: {
  toasts: {
    id: number;
    message: string;
    tone: "neutral" | "success" | "danger";
    action?: { label: string; run: () => void };
  }[];
  onDismiss: (id: number) => void;
}) {
  return (
    <div class="pointer-events-none fixed bottom-6 left-1/2 z-50 flex w-full max-w-[480px] -translate-x-1/2 flex-col items-center gap-2 px-4">
      <For each={props.toasts}>
        {(t) => (
          <div
            class={`od-fade-slide-up pointer-events-auto flex items-center gap-3 rounded-full border px-4 py-2.5 text-paper shadow-overlay ${
              t.tone === "danger"
                ? "border-danger/60 bg-ink"
                : t.tone === "success"
                  ? "border-leaf/50 bg-ink"
                  : "border-line-strong bg-ink"
            }`}
          >
            <span
              class={`h-2 w-2 shrink-0 rounded-full ${
                t.tone === "success" ? "bg-leaf-soft" : t.tone === "danger" ? "bg-danger-strong" : "bg-ghost"
              }`}
            />
            <p class="text-[13px] font-medium leading-5">{t.message}</p>
            <Show when={t.action}>
              <button
                class="shrink-0 text-[12px] font-semibold underline"
                onClick={() => {
                  t.action?.run();
                  props.onDismiss(t.id);
                }}
              >
                {t.action?.label}
              </button>
            </Show>
            <button
              class="shrink-0 text-paper/60 transition-colors duration-100 hover:text-paper"
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
  confirmIcon?: JSX.Element;
  cancelLabel?: string;
  busyLabel?: string;
  busy?: boolean;
  confirmTone?: "danger" | "primary";
  confirmDisabled?: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const titleId = createUniqueId();
  const blocked = () => props.busy || props.confirmDisabled;

  // Escape cancels: the one key every dialog owes the keyboard.
  createEffect(() => {
    if (!props.open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !props.busy) props.onCancel();
    };
    document.addEventListener("keydown", onKey);
    onCleanup(() => document.removeEventListener("keydown", onKey));
  });

  return (
    <Show when={props.open}>
      <div
        class="od-fade-in fixed inset-0 z-50 flex items-center justify-center bg-ink/30 p-4"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        onClick={props.busy ? undefined : props.onCancel}
      >
        <div
          class="od-scale-in w-[26.25rem] rounded-[12px] border border-line bg-paper p-[22px] shadow-overlay"
          onClick={(e) => e.stopPropagation()}
        >
          <h3 id={titleId} class="text-[17px] font-semibold tracking-[-0.01em] text-ink">
            {props.title}
          </h3>
          <div class="mt-2 text-[13px] font-medium leading-[1.7] text-muted">{props.body}</div>
          <div class="mt-5 flex justify-end gap-2">
            <Button size="sm" variant="ghost" class="rounded-full" onClick={props.onCancel} disabled={props.busy}>
              {props.cancelLabel ?? "Keep"}
            </Button>
            <Show
              when={props.confirmTone === "primary"}
              fallback={
                <button
                  type="button"
                  onClick={props.onConfirm}
                  disabled={blocked()}
                  class="inline-flex h-8 select-none items-center justify-center gap-2 rounded-full bg-danger-strong px-4 text-[13px] font-semibold leading-none text-white transition-[background-color,transform] duration-100 ease-snappy active:translate-y-px disabled:opacity-45 disabled:pointer-events-none"
                >
                  {props.busy ? (props.busyLabel ?? "Deleting…") : (props.confirmLabel ?? "Delete")}
                </button>
              }
            >
              <Button size="sm" onClick={props.onConfirm} disabled={blocked()}>
                {props.confirmIcon}
                {props.confirmLabel ?? "Save"}
              </Button>
            </Show>
          </div>
        </div>
      </div>
    </Show>
  );
}
