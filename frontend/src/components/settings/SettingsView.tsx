import { createEffect, createSignal, createUniqueId, For, Show } from "solid-js";
import { useAppStore } from "../../lib/store";
import { Button, ViewHeading } from "../ui/primitives";
import { cloneCfg, sanitizeConfig } from "./config";
import { SettingsProvider } from "./context";
import { NAV_GROUPS } from "./nav";
import type { SectionKey } from "./types";
import { CacheSection } from "./sections/CacheSection";
import { ChunkingSection } from "./sections/ChunkingSection";
import { ConnectSection } from "./sections/ConnectSection";
import { IndexingSection } from "./sections/IndexingSection";
import { ModelSection } from "./sections/ModelSection";
import { OcrSection } from "./sections/OcrSection";
import { SearchSection } from "./sections/SearchSection";
import { SourcesSection } from "./sections/SourcesSection";
import { VexterSection } from "./sections/VexterSection";

export function SettingsView() {
  const store = useAppStore();

  const leaveTitleId = createUniqueId();

  const draft = () => store.settingsDraft();
  const [section, setSection] = createSignal<SectionKey>(
    (store.takePendingSettingsSection() as SectionKey) ?? "model",
  );
  createEffect(() => {
    const pending = store.takePendingSettingsSection();
    if (pending) setSection(pending as SectionKey);
  });

  createEffect(() => {
    const c = store.config();
    if (c && !store.settingsDraft()) store.replaceSettingsDraft(sanitizeConfig(cloneCfg(c)));
  });

  const save = async () => {
    await store.saveSettings();
  };

  const saveAndLeave = async () => {
    await store.saveSettings();
    store.confirmLeave();
  };

  return (
    <div class="relative flex h-full flex-col">
      <div class="px-5 pt-6 md:px-8">
        <ViewHeading title="Settings" note="Model, chunking, search, and sources. Everything stays on this machine." />
      </div>

      <Show when={draft()} fallback={<p class="px-6 text-[13px] font-medium text-muted">Loading settings…</p>}>
        <SettingsProvider>
          <div class="flex min-h-0 flex-1 flex-col">
            <div class="scroll-quiet flex w-full shrink-0 items-center gap-1.5 overflow-x-auto px-4 pb-3 md:hidden">
              <For each={NAV_GROUPS.flatMap((g) => g.items)}>
                {(it) => {
                  const active = () => section() === it.key;
                  let el: HTMLButtonElement | undefined;
                  createEffect(() => {
                    if (active()) el?.scrollIntoView({ block: "nearest", inline: "nearest" });
                  });
                  return (
                    <button
                      ref={el}
                      type="button"
                      onClick={() => setSection(it.key)}
                      aria-current={active() ? "page" : undefined}
                      class={`group flex shrink-0 items-center gap-1.5 rounded-full border px-3 py-1 text-[12px] font-semibold transition-colors duration-100 ${
                        active() ? "border-leaf-deep bg-mint text-leaf-deep" : "border-line-control bg-paper text-ink-soft"
                      }`}
                    >
                      <span
                        class={`flex shrink-0 items-center justify-center ${
                          active() ? "" : "text-muted"
                        }`}
                      >
                        <it.icon size={15} active={active()} />
                      </span>
                      {it.label}
                    </button>
                  );
                }}
              </For>
            </div>

            <div class="flex min-h-0 min-w-0 flex-1">
            {/* Left rail: grouped sections, active on a mint pill */}
            <nav
              class="scroll-quiet hidden w-52 shrink-0 overflow-y-auto border-r border-line py-3 pr-3 md:block"
              aria-label="Settings sections"
            >
              <For each={NAV_GROUPS}>
                {(g) => (
                  <div class="mb-5">
                    <p class="mb-1.5 px-2.5 text-[12px] font-semibold text-muted">
                      {g.label}
                    </p>
                    <For each={g.items}>
                      {(it) => {
                        const active = () => section() === it.key;
                        return (
                          <button
                            type="button"
                            onClick={() => setSection(it.key)}
                            aria-current={active() ? "page" : undefined}
                            class={`group flex w-full items-center gap-2.5 rounded-[8px] px-2.5 py-2 text-[13px] font-semibold transition-colors duration-100 ease-snappy ${
                              active() ? "bg-mint text-leaf-deep" : "text-muted hover:bg-surface-2 hover:text-ink"
                            }`}
                          >
                            <span
                              class={`flex h-[18px] w-[18px] shrink-0 items-center justify-center ${
                                active() ? "text-leaf-deep" : "text-muted"
                              }`}
                            >
                              <it.icon size={18} active={active()} />
                            </span>
                            <span class="min-w-0 truncate text-left">{it.label}</span>
                          </button>
                        );
                      }}
                    </For>
                  </div>
                )}
              </For>
            </nav>

              <div
                class={`scroll-quiet min-w-0 flex-1 overflow-y-auto bg-paper ${
                  store.settingsDirty() ? "pb-20" : "pb-4"
                }`}
              >
              <div class="px-4 py-5 md:px-6 md:py-6">
                <Show when={section() === "model"}>
                  <ModelSection />
                </Show>
                <Show when={section() === "ocr"}>
                  <OcrSection />
                </Show>
                <Show when={section() === "chunking"}>
                  <ChunkingSection />
                </Show>
                <Show when={section() === "search"}>
                  <SearchSection />
                </Show>
                <Show when={section() === "cache"}>
                  <CacheSection />
                </Show>
                <Show when={section() === "sources"}>
                  <SourcesSection />
                </Show>
                <Show when={section() === "indexing"}>
                  <IndexingSection />
                </Show>
                <Show when={section() === "vexter"}>
                  <VexterSection />
                </Show>
                <Show when={section() === "connect"}>
                  <ConnectSection />
                </Show>
              </div>
              </div>
            </div>
          </div>
        </SettingsProvider>
      </Show>

      {/* Sticky save bar: pinned to the bottom of the view so saving doesn't
          mean scrolling back to the top. Only appears while the draft is dirty. */}
      <Show when={store.settingsDirty()}>
        <div class="absolute inset-x-0 bottom-0 flex items-center gap-3 border-t border-line bg-paper px-6 py-3.5">
          <span
            class="inline-flex items-center gap-1.5 rounded-full border border-amber-warm bg-amber-soft px-2 py-0.5 text-[11px] font-semibold uppercase tracking-[0.04em] text-amber-deep"
            role="status"
          >
            <span class="h-1.5 w-1.5 rounded-full bg-amber" aria-hidden="true" />
            unsaved
          </span>
          <span class="ml-auto flex items-center gap-2">
            <Button variant="ghost" onClick={() => store.confirmLeave({ discard: true })}>
              Discard
            </Button>
            <Button onClick={() => void save()}>Save settings</Button>
          </span>
        </div>
      </Show>

      <Show when={store.pendingLeave() !== null}>
        <div
          class="fixed inset-0 z-50 flex items-center justify-center bg-ink/30 p-4"
          role="dialog"
          aria-modal="true"
          aria-labelledby={leaveTitleId}
          onClick={() => store.cancelLeave()}
        >
          <div
            class="od-scale-in w-[26rem] rounded-[12px] border border-line bg-paper p-[22px] shadow-overlay"
            onClick={(e) => e.stopPropagation()}
          >
            <h3 id={leaveTitleId} class="text-[17px] font-semibold tracking-[-0.01em] text-ink">
              Save your changes before leaving?
            </h3>
            <p class="mt-2 text-[13px] font-medium leading-[1.7] text-muted">
              You have unsaved changes. If you leave now, they'll be lost.
            </p>
            <div class="mt-5 flex flex-col gap-2">
              <Button autofocus onClick={() => void saveAndLeave()}>
                Save settings
              </Button>
              <Button variant="outline" onClick={() => store.confirmLeave({ discard: true })}>
                Leave without saving
              </Button>
              <Button variant="ghost" onClick={() => store.cancelLeave()}>
                Keep editing
              </Button>
            </div>
          </div>
        </div>
      </Show>
    </div>
  );
}
