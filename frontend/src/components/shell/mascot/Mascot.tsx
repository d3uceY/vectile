import { Show } from "solid-js";
import { SpriteAnimation } from "../../ui/SpriteAnimation";
import { useMascotState } from "./state";
import { MASCOT_ART_PX, MASCOT_CLIPS, MASCOT_LIFT_ART } from "./assets";
import "./mascot.css";

/* ------------------------------------------------------------------
   Sidebar mascot: the pixel mascot resting at the bottom of the footer
   colophon. It plays the idle sheet, switches to the indexing or searching
   sheet while that interaction runs, then settles back to idle. Purely
   decorative.

   The component composes the module pieces:
     - useMascotState() : store -> which sheet is playing (null when every
                          Vexter switch is off)
     - assets.ts        : state -> sprite clip
     - mascot.css       : the stage it stands on
   A sheet change swaps the clip in place: the player restarts the new clip's
   intro, and nothing pops, rises or fades.
   ------------------------------------------------------------------ */

export function Mascot() {
  const active = useMascotState();

  return (
    <Show when={active()}>
      {(state) => (
        <div class="mascot-stage" aria-hidden="true">
          <SpriteAnimation
            clip={MASCOT_CLIPS[state()]}
            artPx={MASCOT_ART_PX}
            lift={(MASCOT_LIFT_ART[state()] ?? 0) * MASCOT_ART_PX}
          />
        </div>
      )}
    </Show>
  );
}
