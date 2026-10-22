import { createEffect, createSignal, onCleanup, type Accessor } from "solid-js";

/* ------------------------------------------------------------------
   Sprite sheet playback.

   A sheet is one horizontal strip of frames. A clip says how long each
   frame is held and which frames play: an optional intro range that runs
   once, then an optional loop range that repeats forever. Ranges are
   1-based and inclusive, the way the art is exported, so "play 1 to 20
   once, then keep looping 11 to 20" is { play: [1, 20], loop: [11, 20] }.
   ------------------------------------------------------------------ */

/** First and last frame of a range, 1-based and inclusive. */
export type FrameRange = readonly [number, number];

export interface SpriteRect {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface SpriteSheet {
  /** Image URL. */
  src: string;
  /** Frames in the strip. */
  frames: number;
  /** Frame size in sheet pixels. */
  frameWidth: number;
  frameHeight: number;
  /** Sheet pixels per art pixel. Sheets exported at different scales keep
      one on-screen art-pixel size when this is set. Defaults to 1. */
  pixelSize?: number;
  /** The drawn part of a frame, in sheet pixels. Defaults to the whole frame. */
  content?: SpriteRect;
}

export interface SpriteClip {
  sheet: SpriteSheet;
  /** How long each frame is held, in ms. Defaults to SPRITE_FRAME_MS. */
  frameMs?: number;
  /** Frames played once, in order, before the loop starts. Defaults to the
      whole strip. */
  play?: FrameRange;
  /** Frames repeated forever once the intro ends. Defaults to `play`. */
  loop?: FrameRange;
}

/** Default time between frames, in ms. */
export const SPRITE_FRAME_MS = 90;

/** The frames to play once, then the frames to repeat, as 1-based numbers.
    Ranges are clamped to the sheet and never empty. */
export function spriteTimeline(clip: SpriteClip): { intro: number[]; loop: number[] } {
  const total = Math.max(1, Math.floor(clip.sheet.frames));
  const play = clampRange(clip.play ?? [1, total], total);
  const loop = clampRange(clip.loop ?? play, total);
  return { intro: range(play), loop: range(loop) };
}

/** The drawn part of a frame, in sheet pixels. */
export function spriteContent(sheet: SpriteSheet): SpriteRect {
  return sheet.content ?? { x: 0, y: 0, width: sheet.frameWidth, height: sheet.frameHeight };
}

/** CSS pixels per sheet pixel for a given on-screen art-pixel size. */
export function spriteScale(sheet: SpriteSheet, artPx: number): number {
  return artPx / (sheet.pixelSize ?? 1);
}

/** Frame numbers for a clip, restarting whenever the clip or the paused flag
    changes. Paused holds the first intro frame (used for reduced motion). */
export function createSpritePlayer(
  clip: Accessor<SpriteClip>,
  paused?: Accessor<boolean>,
): Accessor<number> {
  const [frame, setFrame] = createSignal(spriteTimeline(clip()).intro[0]);

  createEffect(() => {
    const timeline = spriteTimeline(clip());
    const frameMs = clip().frameMs ?? SPRITE_FRAME_MS;

    if (paused?.()) {
      setFrame(timeline.intro[0]);
      return;
    }

    let step = 0;
    let timer: ReturnType<typeof setTimeout> | undefined;

    const tick = () => {
      const intro = timeline.intro.length;
      setFrame(
        step < intro ? timeline.intro[step] : timeline.loop[(step - intro) % timeline.loop.length],
      );
      step++;
      timer = setTimeout(tick, frameMs);
    };

    tick();
    onCleanup(() => clearTimeout(timer));
  });

  return frame;
}

/** True while the user asks for reduced motion, so a player can freeze. */
export function useReducedMotion(): Accessor<boolean> {
  const query =
    typeof window === "undefined"
      ? undefined
      : window.matchMedia("(prefers-reduced-motion: reduce)");
  const [reduced, setReduced] = createSignal(query?.matches ?? false);

  if (query) {
    const onChange = (event: MediaQueryListEvent) => setReduced(event.matches);
    query.addEventListener("change", onChange);
    onCleanup(() => query.removeEventListener("change", onChange));
  }

  return reduced;
}

function clampRange(range: FrameRange, total: number): [number, number] {
  const first = clampFrame(range[0], total);
  const last = clampFrame(range[1], total);
  return first <= last ? [first, last] : [last, first];
}

function clampFrame(frame: number, total: number): number {
  return Math.min(total, Math.max(1, Math.round(frame) || 1));
}

function range([first, last]: [number, number]): number[] {
  return Array.from({ length: last - first + 1 }, (_, i) => first + i);
}
