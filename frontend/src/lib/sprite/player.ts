import { createEffect, createSignal, onCleanup, type Accessor } from "solid-js";
import { framePlan } from "./timeline";
import type { FrameStep, SpriteClip } from "./types";

export interface SpritePlayer {
  /** The frame number to draw right now. */
  frame: Accessor<number>;
  /** True once a clip with `loop: false` has played its last frame. Stays
      false for looping clips and resets whenever the clip changes. */
  done: Accessor<boolean>;
}

/** Drives a clip's frame numbers, one `setTimeout` at a time so every frame can
    hold for its own time. Restarts whenever the clip or the paused flag
    changes; paused holds the first frame (reduced motion). */
export function createSpritePlayer(
  clip: Accessor<SpriteClip>,
  paused?: Accessor<boolean>,
): SpritePlayer {
  const [frame, setFrame] = createSignal(framePlan(clip()).intro[0].frame);
  const [done, setDone] = createSignal(false);

  createEffect(() => {
    const plan = framePlan(clip());
    const still = paused?.() ?? false;
    setDone(false);

    if (still) {
      setFrame(plan.intro[0].frame);
      return;
    }

    let step = 0;
    let timer: ReturnType<typeof setTimeout> | undefined;

    const tick = () => {
      const { intro, loop } = plan;
      const current: FrameStep | undefined =
        step < intro.length
          ? intro[step]
          : loop
            ? loop[(step - intro.length) % loop.length]
            : undefined;

      if (!current) {
        setDone(true);
        return;
      }

      setFrame(current.frame);
      step++;
      timer = setTimeout(tick, current.frameMs);
    };

    tick();
    onCleanup(() => clearTimeout(timer));
  });

  return { frame, done };
}
