import { createEffect, createSignal, For, onCleanup, type JSX } from "solid-js";
import { MonitorIcon, MoonIcon, SunIcon } from "../ui/icons";
import {
  applyTheme,
  getThemePref,
  setThemePref,
  type ThemePref,
} from "../../lib/theme";

const OPTIONS: {
  id: ThemePref;
  label: string;
  title: string;
  icon: (p: { size?: number; class?: string }) => JSX.Element;
}[] = [
  { id: "system", label: "Auto", title: "Auto theme (follows the system)", icon: MonitorIcon },
  { id: "light", label: "Light", title: "Light theme", icon: SunIcon },
  { id: "dark", label: "Dark", title: "Dark theme", icon: MoonIcon },
];

const next = (pref: ThemePref): ThemePref =>
  pref === "system" ? "light" : pref === "light" ? "dark" : "system";

/** Theme switcher for the sidebar footer. `compact` collapses it to one
    cycling button for the 64px icon rail; otherwise it is a 3-way segmented
    control. Follows the OS while the preference is "system". */
export function ThemeToggle(props: { compact?: boolean }) {
  const [pref, setPref] = createSignal<ThemePref>(getThemePref());

  // Apply on mount and whenever the preference changes. While it is "system",
  // also re-apply when the OS flips.
  createEffect(() => {
    const current = pref();
    applyTheme(current);
    if (current !== "system") return;
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => applyTheme("system");
    mq.addEventListener("change", onChange);
    onCleanup(() => mq.removeEventListener("change", onChange));
  });

  const choose = (id: ThemePref) => {
    setPref(id);
    setThemePref(id);
  };

  const current = () => OPTIONS.find((o) => o.id === pref()) ?? OPTIONS[0];

  if (props.compact) {
    return (
      <button
        type="button"
        onClick={() => choose(next(pref()))}
        class="flex h-8 w-8 items-center justify-center rounded-full border border-line bg-surface-2 text-muted transition-colors duration-100 hover:text-ink"
        aria-label={`Theme: ${current().title}`}
        title={current().title}
      >
        {current().icon({ size: 15 })}
      </button>
    );
  }

  return (
    <div
      role="radiogroup"
      aria-label="Theme"
      class="flex items-center gap-0.5 rounded-full border border-line bg-surface-2 p-0.5"
    >
      <For each={OPTIONS}>
        {(o) => {
          const active = () => pref() === o.id;
          return (
            <button
              type="button"
              role="radio"
              aria-checked={active()}
              aria-label={o.title}
              title={o.title}
              onClick={() => choose(o.id)}
              class={`flex h-7 min-w-0 flex-1 items-center justify-center gap-1 whitespace-nowrap rounded-full px-1 text-[11px] font-semibold transition-colors duration-100 ease-snappy ${
                active() ? "bg-mint text-leaf-deep" : "text-muted hover:text-ink"
              }`}
            >
              <span class="shrink-0">{o.icon({ size: 14 })}</span>
              <span>{o.label}</span>
            </button>
          );
        }}
      </For>
    </div>
  );
}
