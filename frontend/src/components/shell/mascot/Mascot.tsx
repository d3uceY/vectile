import { Show, createEffect, createSignal, onMount } from "solid-js";
import { SpriteAnimation } from "../../ui/SpriteAnimation";
import { loadSheet, useReducedMotion, type SpriteClip } from "../../../lib/sprite";
import { useMascotState } from "./state";
import { MASCOT_ART_PX, MASCOT_CLIPS, MASCOT_EASTER, mascotLift } from "./assets";
import type { ActiveMascotState } from "./types";
import "./mascot.css";

/* ------------------------------------------------------------------
   Sidebar mascot: the pixel chameleon resting at the bottom of the footer
   colophon. It plays the idle sheet, switches to the indexing or searching
   sheet while that interaction runs, then settles back to idle. Purely
   decorative.

   The component composes the module pieces:
     - useMascotState() : store -> which sheet is playing (null when every
                          Vexter switch is off)
     - assets.ts        : state -> sprite clip, plus the click easter egg
     - mascot.css       : the stage it stands on and the click target
   A sheet change swaps the clip in place: the player restarts the new clip's
   intro, and nothing pops, rises or fades.

   Clicking it plays one of the easter-egg sheets, picked at random, once and
   over whatever is showing, then hands back.
   ------------------------------------------------------------------ */

export function Mascot() {
  const active = useMascotState();
  const still = useReducedMotion();

  /** The easter egg, or null when idle/deep work owns the stage. A fresh object
      per click restarts the clip if the mascot is tapped again mid-play. */
  const [easter, setEaster] = createSignal<SpriteClip | null>(null);

  const clip = (state: ActiveMascotState) => easter() ?? MASCOT_CLIPS[state];

  // Nothing is rendered when every switch is off, so do not keep the egg warm.
  createEffect(() => {
    if (!active()) setEaster(null);
  });

  // Decode every strip up front so a swap never paints a half-loaded sheet.
  onMount(() => {
    for (const c of Object.values(MASCOT_CLIPS)) void loadSheet(c.sheet.src);
    for (const c of MASCOT_EASTER) void loadSheet(c.sheet.src);
  });

  /** One of the idle eggs at random; the spread copies it so a second click
      mid-play restarts the clip instead of reusing the running one. */
  const play = async () => {
    if (still()) return;
    const egg = MASCOT_EASTER[Math.floor(Math.random() * MASCOT_EASTER.length)];
    await loadSheet(egg.sheet.src);
    setEaster({ ...egg });
  };

  return (
    <Show when={active()}>
      {(state) => (
        <div class="mascot-stage" aria-hidden="true">
          <button type="button" class="mascot-hit" tabIndex={-1} onClick={play}>
            <SpriteAnimation
              clip={clip(state())}
              artPx={MASCOT_ART_PX}
              lift={mascotLift(clip(state()))}
              onDone={() => setEaster(null)}
            />
          </button>
        </div>
      )}
    </Show>
  );
}
