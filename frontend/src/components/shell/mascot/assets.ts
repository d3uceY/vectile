import type { SpriteClip } from "../../../lib/sprite";
import type { ActiveMascotState } from "./types";

/* The mascot is drawn as horizontal sprite sheets. Each clip owns its frame
   timing and its play order: `play` runs once, then `loop` repeats forever, so
   a long animation can settle into a short repeat without a second file. Live
   example: { play: [1, 20], loop: [11, 20] } plays the intro, then cycles the
   tail of the sheet.

   The three sheets were exported at different scales (16, 3 and 4 sheet pixels
   per art pixel), so `pixelSize` keeps one art pixel the same size on screen in
   every state, and `content` crops each frame to the drawn area so the mascot
   sits on the same floor whichever sheet is playing. */

const MASCOT_DIR = "/mascot";

export const MASCOT_CLIPS: Record<ActiveMascotState, SpriteClip> = {
  idle: {
    sheet: {
      src: `${MASCOT_DIR}/idle.png`,
      frames: 10,
      frameWidth: 960,
      frameHeight: 960,
      pixelSize: 16,
      content: { x: 144, y: 336, width: 608, height: 560 },
    },
    frameMs: 110,
  },
  indexing: {
    sheet: {
      src: `${MASCOT_DIR}/indexing.png`,
      frames: 29,
      frameWidth: 192,
      frameHeight: 192,
      pixelSize: 3,
      content: { x: 27, y: 63, width: 165, height: 108 },
    },
    frameMs: 150,
    /* Play the whole sheet once, then settle into the working loop. */
    play: [1, 29],
    loop: [9, 29],
  },
  searching: {
    sheet: {
      src: `${MASCOT_DIR}/searching.png`,
      frames: 39,
      frameWidth: 240,
      frameHeight: 240,
      pixelSize: 4,
      content: { x: 8, y: 52, width: 224, height: 120 },
    },
    frameMs: 120,
    /* Play the whole sheet once, then keep looping the tail while the search
       is still running. */
    play: [1, 39],
    loop: [8, 39],
  },
};

/** On-screen CSS pixels per art pixel. One value for every state, so the
    mascot keeps a single size while it switches sheets. */
export const MASCOT_ART_PX = 2;

/** How far each sheet sits above the stage floor, in art pixels, so every sheet
    stands on the same branch line. Idle and indexing leave about 13 art pixels
    of curled tail below the branch, which puts their branch on the floor line
    already; the searching art ends at the branch, so it needs lifting to meet
    them. */
export const MASCOT_LIFT_ART: Partial<Record<ActiveMascotState, number>> = {
  searching: 13,
};
