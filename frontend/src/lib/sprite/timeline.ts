import type {
  FramePlan,
  FrameRange,
  FrameSpan,
  FrameStep,
  SpriteClip,
} from "./types";

/** Time per frame when a clip sets neither `frameMs` nor a matching rule. */
export const SPRITE_FRAME_MS = 90;

/** Resolve a clip into steps: the frames that play once, then the frames that
    repeat. Each step carries its own hold time, so a dozen per-frame rules cost
    nothing at play time. Ranges are clamped to the sheet and never empty. */
export function framePlan(clip: SpriteClip): FramePlan {
  const total = Math.max(1, Math.floor(clip.sheet.frames));
  const play = clampRange(clip.play ?? [1, total], total);
  const intro = steps(clip, play, total);

  if (clip.loop === false) return { intro, loop: null };

  const loop = clampRange(clip.loop ?? play, total);
  return { intro, loop: steps(clip, loop, total) };
}

/** How long one frame is held: the clip's `frameMs`, then each timing rule in
    order, where the last rule covering the frame wins. */
export function frameMsFor(clip: SpriteClip, frame: number, total?: number): number {
  const sheetFrames = Math.max(1, Math.floor(total ?? clip.sheet.frames));
  let ms = clip.frameMs ?? SPRITE_FRAME_MS;

  for (const rule of clip.timing ?? []) {
    const [first, last] = clampRange(spanRange(rule.frames), sheetFrames);
    if (frame >= first && frame <= last) ms = rule.frameMs;
  }

  return ms;
}

/** Normalize any span to a first/last pair. */
export function spanRange(span: FrameSpan): FrameRange {
  return typeof span === "number" ? [span, span] : span;
}

function steps(clip: SpriteClip, range: FrameRange, total: number): FrameStep[] {
  return framesIn(range).map((frame) => ({ frame, frameMs: frameMsFor(clip, frame, total) }));
}

function framesIn([first, last]: FrameRange): number[] {
  return Array.from({ length: last - first + 1 }, (_, i) => first + i);
}

/** Clamp a range to the sheet and order it, so `[8, 3]` reads as 3..8. */
function clampRange([first, last]: FrameRange, total: number): FrameRange {
  const a = clampFrame(first, total);
  const b = clampFrame(last, total);
  return a <= b ? [a, b] : [b, a];
}

function clampFrame(frame: number, total: number): number {
  return Math.min(total, Math.max(1, Math.round(frame) || 1));
}
