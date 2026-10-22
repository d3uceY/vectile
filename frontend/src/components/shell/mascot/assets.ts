import type { SpriteClip } from "../../../lib/sprite";
import type { ActiveMascotState } from "./types";

/* The mascot is drawn as horizontal sprite sheets. A clip owns its frame
   timings and its play order: `play` runs once, then `loop` repeats forever, so
   a long animation can settle into a short repeat without a second file.
   Timings can be set once (`frameMs`) and then corrected per frame with
   `timing`, where later rules win, e.g. 60ms on 1-3 and 100ms everywhere else.

   The sheets were exported at different scales (16, 3 and 4 sheet pixels per art
   pixel), so `pixelSize` keeps one art pixel the same size on screen in every
   state, and `content` crops each frame to the drawn area so the mascot sits on
   the same branch line whichever sheet is playing. The idle, searching, cool and
   pacman sheets are all drawn on the same 60x60 art canvas, so `anchor` points
   them at art x 28 (448 at 16x, 112 at 4x) and the branch never slides sideways
   when the sheet swaps. Indexing is its own 64x64 canvas, so it stays centred
   on its crop. */

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
      anchor: 448,
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
      /* Same 60x60 art canvas as idle, so the same art point (28) keeps the
         branch in place when the sheet swaps. */
      anchor: 112,
    },
    frameMs: 120,
    /* Play the whole sheet once, then keep looping the tail while the search
       is still running. */
    play: [1, 39],
    loop: [8, 39],
  },
};

/** Idle's easter eggs. Not states and not Settings switches: clicking the
    mascot plays one of them at random, once, then the sheet that was already on
    screen comes back. Each entry shares the idle art scale and bottom edge, so
    the egg never shifts the mascot on its branch. */
export const MASCOT_EASTER: SpriteClip[] = [
  /* The sunglasses beat. Frames 1-3 and 14-15 land on 60ms, the rest on 100ms.
     Same 960 frame and same branch as idle, so it anchors on the body's centre
     line (448) even though the crop is 56 sheet pixels wider. */
  {
    sheet: {
      src: `${MASCOT_DIR}/idle-cool.png`,
      frames: 16,
      frameWidth: 960,
      frameHeight: 960,
      pixelSize: 16,
      content: { x: 144, y: 336, width: 664, height: 560 },
      anchor: 448,
    },
    frameMs: 100,
    timing: [
      { frames: [1, 3], frameMs: 60 },
      { frames: [14, 15], frameMs: 60 },
    ],
    loop: false,
  },
  /* The pellet run: the tongue snaps out and the pellets vanish. 18240x960,
     19 frames of 960. Frame 4 is the 50ms snap; the rest hold for 200ms.
     The pellets reach 86 sheet pixels further right than idle's crop, so the
     frame is anchored on the body's centre line (448, idle's crop centre) to
     keep the chameleon on its spot when the egg takes over. */
  {
    sheet: {
      src: `${MASCOT_DIR}/idle-pacman.png`,
      frames: 19,
      frameWidth: 960,
      frameHeight: 960,
      pixelSize: 16,
      content: { x: 144, y: 336, width: 780, height: 560 },
      anchor: 448,
    },
    frameMs: 200,
    timing: [{ frames: 4, frameMs: 50 }],
    loop: false,
  },
];

/** On-screen CSS pixels per art pixel. One value for every state, so the
    mascot keeps a single size while it switches sheets. */
export const MASCOT_ART_PX = 2;

/** Sheets whose branch sits above their content bottom, in art pixels, keyed by
    clip so a one-off clip never inherits a state's lift. Idle and indexing leave
    about 13 art pixels of curled tail below the branch; the searching art ends
    at the branch, so it alone needs lifting to meet them. */
const LIFT_ART = new Map<SpriteClip, number>([[MASCOT_CLIPS.searching, 13]]);

/** How far a sheet sits above its container's bottom edge, in CSS pixels. */
export function mascotLift(clip: SpriteClip, artPx = MASCOT_ART_PX): number {
  return (LIFT_ART.get(clip) ?? 0) * artPx;
}
