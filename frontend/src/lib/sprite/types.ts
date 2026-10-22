/* ------------------------------------------------------------------
   Sprite sheet engine: types.

   A sheet is one horizontal strip of frames drawn at one art-pixel scale.
   A clip says which frames play in what order and how long each is held.
   Frame numbers are 1-based and inclusive, the way the art is exported.
   ------------------------------------------------------------------ */

/** First and last frame of a range, 1-based and inclusive. */
export type FrameRange = readonly [number, number];

/** One frame, or a run of frames, 1-based. */
export type FrameSpan = FrameRange | number;

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
  /** Sheet-pixel x of the point that must stay on the same screen spot when
      clips swap, e.g. the centre of the character. Defaults to the centre of
      `content`, which is only right while every crop is the same width. A clip
      whose art reaches further to one side sets this so the wider crop grows
      away from the character instead of pushing it across. */
  anchor?: number;
}

/** A timing rule: how long the frames in `frames` are held. Rules apply in
    order after the clip's `frameMs`, so a later, narrower rule wins. */
export interface FrameTiming {
  /** The frame or run of frames it applies to. */
  frames: FrameSpan;
  /** How long each of them is held, in ms. */
  frameMs: number;
}

export interface SpriteClip {
  sheet: SpriteSheet;
  /** Time per frame when no rule matches. Defaults to SPRITE_FRAME_MS. */
  frameMs?: number;
  /** Per-frame overrides, applied in order. */
  timing?: FrameTiming[];
  /** Frames played once, in order, before the loop starts. Defaults to the
      whole strip. */
  play?: FrameRange;
  /** Frames repeated forever once the intro ends. `false` plays `play` once
      and stops on its last frame. Defaults to `play`. */
  loop?: FrameRange | false;
}

/** One frame of a resolved plan, with the time it is held. */
export interface FrameStep {
  frame: number;
  frameMs: number;
}

/** A clip resolved into playable steps. `loop` is null when the clip runs once. */
export interface FramePlan {
  intro: FrameStep[];
  loop: FrameStep[] | null;
}
