import { For } from "solid-js";
import { Switch, Toggle } from "../../ui/primitives";
import { SpriteAnimation } from "../../ui/SpriteAnimation";
import { MASCOT_CLIPS, MASCOT_LIFT_ART } from "../../shell/mascot/assets";
import type { ActiveMascotState } from "../../shell/mascot/types";
import type { SpriteClip } from "../../../lib/sprite";
import type { MascotConfig } from "../../../lib/types";
import { Section } from "../fields";
import { VexterNavIcon } from "../nav";
import { useSettings } from "../context";

const MASCOT_STATES: {
  key: keyof MascotConfig;
  state: ActiveMascotState;
  label: string;
  desc: string;
  clip: SpriteClip;
}[] = [
  {
    key: "show_idle",
    state: "idle",
    label: "Idle",
    desc: "While nothing is running",
    clip: MASCOT_CLIPS.idle,
  },
  {
    key: "show_searching",
    state: "searching",
    label: "Searching",
    desc: "While the Search page is open",
    clip: MASCOT_CLIPS.searching,
  },
  {
    key: "show_indexing",
    state: "indexing",
    label: "Indexing",
    desc: "While a library rebuilds",
    clip: MASCOT_CLIPS.indexing,
  },
];

export function VexterSection() {
  const { draft, setMascot, setMascotAll, mascotAllDisabled } = useSettings();

  return (
    <Section
      icon={<VexterNavIcon size={16} />}
      title="Vexter"
      note="The pixel mascot in the sidebar. It rests at idle and changes while your library works, and each of those moments has its own switch."
    >
      <div class="space-y-6">
        <Toggle
          checked={mascotAllDisabled()}
          onChange={(v) => setMascotAll(v)}
          label="Disable Vexter"
          description="Hide the mascot for every moment at once."
          hint="Hides the sidebar mascot in every state."
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
                  <SpriteAnimation clip={s.clip} artPx={1} lift={MASCOT_LIFT_ART[s.state] ?? 0} />
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
