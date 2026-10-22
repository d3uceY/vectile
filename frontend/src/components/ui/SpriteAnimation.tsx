import { createEffect } from "solid-js";
import {
  createSpritePlayer,
  spriteAnchor,
  spriteContent,
  spriteScale,
  useReducedMotion,
  type SpriteClip,
} from "../../lib/sprite";
import "./sprite.css";

/** Plays one clip from a horizontal sprite sheet, cropped to the drawn area of
    the frame. `artPx` is the on-screen size of one art pixel, so sheets
    exported at different scales line up without per-state maths at the call
    site. `lift` raises the frame box off the bottom of its container by that
    many CSS pixels, which is how a pose whose ground line sits elsewhere in its
    own art is set back onto the same line as the others. Horizontally the crop
    sits inside a box symmetric about the sheet's `anchor`, so a clip that
    reaches further to one side grows outward instead of dragging the subject
    across. Decorative by default: the frame is hidden from assistive tech. */
export function SpriteAnimation(props: {
  clip: SpriteClip;
  /** On-screen CSS pixels per art pixel. Defaults to 1. */
  artPx?: number;
  /** CSS pixels to raise the frame box off the container's bottom edge. */
  lift?: number;
  /** Called once a clip with `loop: false` reaches its last frame. */
  onDone?: () => void;
  class?: string;
}) {
  const player = createSpritePlayer(
    () => props.clip,
    useReducedMotion(),
  );

  createEffect(() => {
    if (player.done()) props.onDone?.();
  });

  const scale = () => spriteScale(props.clip.sheet, props.artPx ?? 1);
  const box = () => spriteContent(props.clip.sheet);
  const anchor = () => spriteAnchor(props.clip.sheet);
  const width = () => box().width * scale();
  const height = () => box().height * scale();
  const anchorWidth = () => anchor().width * scale();
  const anchorOffset = () => anchor().offset * scale();
  const imageWidth = () => props.clip.sheet.frameWidth * props.clip.sheet.frames * scale();
  const imageHeight = () => props.clip.sheet.frameHeight * scale();
  const offset = () =>
    `translate(${-((player.frame() - 1) * props.clip.sheet.frameWidth + box().x) * scale()}px, ${-box().y * scale()}px)`;

  return (
    <span
      class="sprite-anchor"
      style={{
        width: `${anchorWidth()}px`,
        height: `${height()}px`,
        "margin-bottom": props.lift ? `${props.lift}px` : undefined,
      }}
      aria-hidden="true"
    >
      <span
        class={props.class ? `sprite ${props.class}` : "sprite"}
        style={{
          width: `${width()}px`,
          height: `${height()}px`,
          "margin-left": anchorOffset() ? `${anchorOffset()}px` : undefined,
        }}
      >
        <img
          src={props.clip.sheet.src}
          alt=""
          draggable={false}
          style={{ width: `${imageWidth()}px`, height: `${imageHeight()}px`, transform: offset() }}
        />
      </span>
    </span>
  );
}
