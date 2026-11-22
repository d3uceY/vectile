import { loadSheet, type SpriteClip } from "../../lib/sprite";

/* Five frames of the file sliding out of the folder, two art pixels per frame.
   The sheet is 48x48 per frame: frame 1 is the folder closed, frame 5 is the
   file fully out. Hover steps the strip one frame at a time, so the motion
   reads as pixel jumps and can reverse from the frame already on screen. */
export const FOLDER_HOVER_CLIP: SpriteClip = {
  sheet: {
    src: "/folder-hover.png",
    frames: 5,
    frameWidth: 48,
    frameHeight: 48,
  },
  play: [1, 5],
  loop: false,
};

/** Warm the strip before the first hover so the opening frames never paint as
    an empty box. */
export function preloadFolderSprite(): void {
  void loadSheet(FOLDER_HOVER_CLIP.sheet.src);
}
