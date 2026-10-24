import type { SpriteClip } from "../../../lib/sprite";
import type { ActiveMascotState } from "./types";

/* The mascot is drawn as horizontal sprite sheets. A clip owns its frame
   timings and its play order: `play` runs once, then `loop` repeats forever, so
   a long animation can settle into a short repeat without a second file.
   Timings can be set once (`frameMs`) and then corrected per frame with
   `timing`, where later rules win, e.g. 60ms on 1-3 and 100ms everywhere else.

   The sheets were exported at different scales (4 and 3 sheet pixels per art
   pixel), so `pixelSize` keeps one art pixel the same size on screen in every
   state, and `content` crops each frame to the drawn area so the mascot sits on
   the same branch line whichever sheet is playing. Idle, cool, pacman and
   explode share one 64x64 art canvas with the chameleon's body at art x 29, so
   `anchor` points them at 116 and the branch never slides sideways when the
   sheet swaps. Searching is its own 60x60 canvas with the body at art x 28, so
   its anchor is 112. Indexing is another 64x64 canvas and stays centred on its
   crop. Idle's crop ends 14 art pixels below the branch, which is the line the
   stage stands every other sheet on. */

const MASCOT_DIR = "/mascot";

export const MASCOT_CLIPS: Record<ActiveMascotState, SpriteClip> = {
  idle: {
    sheet: {
      src: `${MASCOT_DIR}/idle.png`,
      /* 2560x256, ten 256 frames. The tail dips two art rows lower than the
         other idle-canvas sheets, so this crop sets the branch line. */
      frames: 10,
      frameWidth: 256,
      frameHeight: 256,
      pixelSize: 4,
      content: { x: 36, y: 88, width: 164, height: 148 },
      anchor: 116,
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
      /* Its own 60x60 canvas, with the body at art x 28: the same screen point
         idle's anchor holds, so the branch stays in place when the sheet swaps. */
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
     4096x256, 16 frames of 256 on idle's canvas and branch, so it anchors on the
     body's centre line (116) even though the crop is 8 sheet pixels wider. */
  {
    sheet: {
      src: `${MASCOT_DIR}/idle-cool.png`,
      frames: 16,
      frameWidth: 256,
      frameHeight: 256,
      pixelSize: 4,
      content: { x: 36, y: 88, width: 172, height: 148 },
      anchor: 116,
    },
    frameMs: 100,
    timing: [
      { frames: [1, 3], frameMs: 60 },
      { frames: [14, 15], frameMs: 60 },
    ],
    loop: false,
  },
  /* The pellet run: the tongue snaps out and the pellets vanish. 4864x256,
     19 frames of 256. Frame 4 is the 50ms snap; the rest hold for 200ms.
     The pellets reach 44 sheet pixels further right than idle's crop, so the
     frame is anchored on the body's centre line (116, idle's anchor) to keep the
     chameleon on its spot when the egg takes over. Its own drawn bottom stops
     two art rows short of idle's tail dip, so the crop borrows idle's height and
     spends two empty art rows under the pellets instead of a lift. */
  {
    sheet: {
      src: `${MASCOT_DIR}/idle-pacman.png`,
      frames: 19,
      frameWidth: 256,
      frameHeight: 256,
      pixelSize: 4,
      content: { x: 36, y: 88, width: 208, height: 148 },
      anchor: 116,
    },
    frameMs: 200,
    timing: [{ frames: 4, frameMs: 50 }],
    loop: false,
  },
  /* The eye glitch: the pupil blooms, bursts into sparks, the chameleon sits
     empty-eyed while the wisp drifts off, then a second burst and the eye is
     back. 4352x256, 17 frames of 256 at four sheet pixels per art pixel, so the
     same 2px art scale as the other eggs. The crop is the union of every frame,
     because the sparks reach above the head (art y 17) and past the body's
     right edge (art x 56) only during the bursts. It also carries two art rows of
     empty space under the tail (168 tall rather than the drawn 160) so the
     branch lands on idle's row, 14 art pixels above the crop's bottom edge. */
  {
    sheet: {
      src: `${MASCOT_DIR}/idle-explode.png`,
      frames: 17,
      frameWidth: 256,
      frameHeight: 256,
      pixelSize: 4,
      content: { x: 36, y: 68, width: 188, height: 168 },
      /* Body centre line: idle's art x 28, shifted by the one art pixel this
         export sits further right. Sparks spill right of it, which the
         symmetric anchor box absorbs. */
      anchor: 116,
    },
    frameMs: 100,
    timing: [
      { frames: [1, 5], frameMs: 200 }, // the blink
      { frames: [6, 9], frameMs: 120 }, // the bloom and the burst
      { frames: [15, 16], frameMs: 120 }, // the second burst
    ],
    loop: false,
  },
];

/** On-screen CSS pixels per art pixel. One value for every state, so the
    mascot keeps a single size while it switches sheets. */
export const MASCOT_ART_PX = 2;

/** Sheets whose branch sits above their content bottom, in art pixels, keyed by
    clip so a one-off clip never inherits a state's lift. The stage stands every
    box on its own crop's bottom edge, so each sheet is lifted by however much
    closer to the branch its art ends than idle's: searching ends at the branch
    (14), indexing one art pixel above idle's tail line (1). */
const LIFT_ART = new Map<SpriteClip, number>([
  [MASCOT_CLIPS.searching, 14],
  [MASCOT_CLIPS.indexing, 1],
]);

/** How far a sheet sits above its container's bottom edge, in CSS pixels. */
export function mascotLift(clip: SpriteClip, artPx = MASCOT_ART_PX): number {
  return (LIFT_ART.get(clip) ?? 0) * artPx;
}
