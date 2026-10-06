import { For, type JSX } from "solid-js";
import { useAppStore } from "../../lib/store";
import type { ModelState, ViewId } from "../../lib/types";
import { modelStateMeta } from "../ui/primitives";
import {
  BrowseNavIcon,
  IndexNavIcon,
  LibraryNavIcon,
  SearchNavIcon,
  SettingsNavIcon,
} from "../ui/nav-icons";
import { Mascot } from "./mascot";
import { ThemeToggle } from "./ThemeToggle";

const NAV: {
  id: ViewId;
  label: string;
  icon: (p: { size?: number; active?: boolean }) => JSX.Element;
}[] = [
  { id: "search", label: "Search", icon: SearchNavIcon },
  { id: "library", label: "Library", icon: LibraryNavIcon },
  { id: "browse", label: "Browse", icon: BrowseNavIcon },
  { id: "index", label: "Index", icon: IndexNavIcon },
  { id: "settings", label: "Settings", icon: SettingsNavIcon },
];

function ModelPlate(props: { state: ModelState; name?: string }) {
  const m = () => modelStateMeta[props.state];
  const tip = () => (props.name ? `${props.state} · ${props.name}` : props.state);
  return (
    <span class="flex min-w-0 items-center gap-2" title={tip()}>
      <span class="relative flex h-2 w-2 shrink-0">
        <span class={`h-2 w-2 rounded-full ${m().dot} ${props.state === "loaded" ? "pulse-dot" : ""}`} />
      </span>
      <span class={`shrink-0 text-[12px] font-semibold ${m().text}`}>{props.state}</span>
      <span class="min-w-0 truncate text-[12px] font-medium text-muted">{props.name ?? "…"}</span>
    </span>
  );
}

export function Sidebar() {
  const store = useAppStore();
  return (
    <aside class="relative flex w-16 shrink-0 flex-col border-r border-line bg-sidebar md:w-56">
      {/* Title plate */}
      <div class="flex flex-col items-center gap-2.5 pb-3 pt-6 md:block md:px-4">
        <div class="flex items-center justify-center gap-2.5 md:justify-start">
          <span class="flex h-8 w-8 shrink-0 items-center justify-center">
            <img src="/vectile-logo.png" alt="vectile" class="h-8 w-8" />
          </span>
          <span class="hidden text-[20px] font-semibold leading-none tracking-[-0.02em] text-ink md:inline">
            vectile
          </span>
        </div>
      </div>

      {/* Destination rail */}
      <nav class="flex-1 px-2 pt-2" aria-label="Primary">
        <ul class="space-y-0.5">
          <For each={NAV}>
            {(item) => {
              const active = () => store.view() === item.id;
              return (
                <li>
                  <button
                    class={`group flex w-full items-center justify-center gap-2.5 rounded-[12px] py-2.5 text-[13px] font-semibold transition-colors duration-100 ease-snappy md:justify-start md:pl-4 md:pr-2.5 ${
                      active() ? "bg-mint text-leaf-deep" : "text-muted hover:bg-surface-2 hover:text-ink"
                    }`}
                    aria-current={active() ? "page" : undefined}
                    aria-label={item.label}
                    title={item.label}
                    onClick={() => store.setView(item.id)}
                  >
                    <span
                      class={`flex h-[18px] w-[18px] shrink-0 items-center justify-center ${
                        active() ? "text-leaf-deep" : "text-faint group-hover:text-ink-soft"
                      }`}
                    >
                      <item.icon size={18} active={active()} />
                    </span>
                    <span class="hidden md:inline">{item.label}</span>
                  </button>
                </li>
              );
            }}
          </For>
        </ul>
      </nav>

      {/* Engine colophon */}
      <footer class="relative shrink-0 border-t border-line px-4 pb-4 pt-3 md:px-5">
        <div class="pointer-events-none absolute inset-x-0 bottom-full hidden justify-center md:flex">
          <Mascot />
        </div>
        {/* Icon rail (< md): theme cycle + just the state dot, no room for text */}
        <div class="flex flex-col items-center gap-2 md:hidden">
          <ThemeToggle compact />
          <span class="relative flex h-2 w-2 shrink-0">
            <span
              class={`h-2 w-2 rounded-full ${modelStateMeta[store.modelState()].dot} ${
                store.modelState() === "loaded" ? "pulse-dot" : ""
              }`}
            />
          </span>
        </div>
        <div class="hidden md:flex md:flex-col md:gap-2.5">
          <ThemeToggle />
          <ModelPlate state={store.modelState()} name={store.modelName()} />
        </div>
      </footer>
    </aside>
  );
}
