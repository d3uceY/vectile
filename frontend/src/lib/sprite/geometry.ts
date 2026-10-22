import type { SpriteRect, SpriteSheet } from "./types";

/** The drawn part of a frame, in sheet pixels. */
export function spriteContent(sheet: SpriteSheet): SpriteRect {
  return sheet.content ?? { x: 0, y: 0, width: sheet.frameWidth, height: sheet.frameHeight };
}

/** CSS pixels per sheet pixel for a given on-screen art-pixel size. */
export function spriteScale(sheet: SpriteSheet, artPx: number): number {
  return artPx / (sheet.pixelSize ?? 1);
}

/** A box that straddles the sheet's anchor point, in sheet pixels. */
export interface SpriteAnchorBox {
  /** Width of the box, symmetric about the anchor. */
  width: number;
  /** Where the crop box sits inside it, from the left. */
  offset: number;
}

/** The crop box placed inside a box that is symmetric about the sheet's anchor.
    A crop is rarely centred on the character (a tongue or a pair of sunglasses
    reaches further one way), and a centred crop would drag the character off
    its spot by half that difference. Centring the anchor instead keeps the
    character still and lets the extra width spill to whichever side it is on.
    The padding is rounded up to whole art pixels so the crop's offset stays on
    the art grid: a fraction of a sheet pixel here becomes a fraction of a
    screen pixel everywhere below, which is what makes a pixel-art sheet
    shimmer as it steps. */
export function spriteAnchor(sheet: SpriteSheet): SpriteAnchorBox {
  const box = spriteContent(sheet);
  const anchor = sheet.anchor ?? box.x + box.width / 2;
  const unit = sheet.pixelSize ?? 1;
  const left = anchor - box.x;
  const right = box.x + box.width - anchor;
  const pad = Math.ceil(Math.max(left, right) / unit) * unit;
  return { width: pad * 2, offset: pad - left };
}
