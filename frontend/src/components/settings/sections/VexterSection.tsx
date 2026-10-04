import { For } from "solid-js";
import { Switch, Toggle } from "../../ui/primitives";
import { MASCOT_ASSETS, MASCOT_STATIC } from "../../shell/mascot/assets";
import { Section } from "../fields";
import { useSettings } from "../context";

const MASCOT_STATES: {
  key: "show_searching" | "show_indexing" | "show_nothing";
  label: string;
  desc: string;
  anim: string;
  static: string;
}[] = [
  {
    key: "show_searching",
    label: "Searching",
    desc: "While a query runs",
    anim: MASCOT_ASSETS.searching,
    static: MASCOT_STATIC,
  },
  {
    key: "show_indexing",
    label: "Indexing",
    desc: "While a library rebuilds",
    anim: MASCOT_ASSETS.indexing,
    static: MASCOT_STATIC,
  },
  {
    key: "show_nothing",
    label: "No results",
    desc: "When a search comes up empty",
    anim: MASCOT_ASSETS.nothing,
    static: MASCOT_STATIC,
  },
];

export function VexterSection() {
  const { draft, setMascot, setMascotAll, mascotAllDisabled } = useSettings();

  return (
    <Section
      icon={
        <span class="flex h-4.5 w-4.5 items-center justify-center overflow-hidden">
          <img
            src={MASCOT_STATIC}
            alt=""
            class="h-full w-full object-contain"
            style="image-rendering: pixelated"
          />
        </span>
      }
      title="Vexter"
      note="The pixel dinosaur in the sidebar. It pokes up while your library works."
    >
      <div class="space-y-6">
        <Toggle
          checked={mascotAllDisabled()}
          onChange={(v) => setMascotAll(v)}
          label="Disable Vexter"
          description="Hide the mascot for every moment at once."
          hint="Vexter is the small pixel dinosaur in the sidebar. When this is on, it never appears, whether you're searching, indexing, or turning up nothing."
        />
        <div class="divide-y divide-line overflow-hidden rounded-[8px] border border-line bg-surface pb-1.5">
          <For each={MASCOT_STATES}>
            {(s) => (
              <div
                class={`flex items-center gap-4 px-3 py-2.5 ${
                  draft()!.gui.mascot[s.key] ? "" : "opacity-60"
                }`}
              >
                <div class="mascot-preview shrink-0">
                  <img class="mascot-preview__anim" src={s.anim} alt="" draggable={false} />
                  <img class="mascot-preview__static" src={s.static} alt="" draggable={false} />
                </div>
                <div class="min-w-0 flex-1">
                  <p class="text-[13px] font-semibold leading-tight text-ink">{s.label}</p>
                  <p class="text-[12px] font-medium leading-5 text-muted">{s.desc}</p>
                </div>
                <Switch
                  checked={draft()!.gui.mascot[s.key]}
                  onChange={(v) => setMascot(s.key, v)}
                  label={`Show Vexter: ${s.label.toLowerCase()}`}
                />
              </div>
            )}
          </For>
        </div>
      </div>
    </Section>
  );
}
