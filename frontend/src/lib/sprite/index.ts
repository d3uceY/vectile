/** Sprite sheet engine.

    Pure data and timing here in `types.ts` / `timeline.ts`, frame geometry in
    `geometry.ts`, and the only Solid-aware pieces in `player.ts` (playback) and
    `motion.ts` (reduced motion). Nothing in this module knows about the mascot:
    a clip is a sheet plus play order plus per-frame timings. */
export * from "./types";
export * from "./timeline";
export * from "./geometry";
export * from "./player";
export * from "./motion";
export * from "./preload";
