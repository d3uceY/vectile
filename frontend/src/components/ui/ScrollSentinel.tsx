import { createEffect, createSignal, onCleanup, onMount, type Accessor } from "solid-js";

/** How far before a sentinel is actually on screen its page is requested. */
const PREFETCH_PX = 600;

/**
 * Invisible marker that reports when it enters the scroll viewport. One above
 * the rows fetches the previous page, one below fetches the next, which is what
 * makes scrolling up and down load more data.
 *
 * `root` must be the scroll container. With the default root the ancestor's clip
 * rect hides the prefetch margin, so the callback would only fire once the row
 * was already visible.
 */
export function ScrollSentinel(props: {
  root: () => HTMLElement | undefined;
  onVisible: () => void;
  /** True while that direction is loading, failed, or has reached the end. */
  stop: Accessor<boolean>;
}) {
  let el!: HTMLDivElement;
  const [visible, setVisible] = createSignal(false);

  onMount(() => {
    const root = props.root();
    if (!root) return;
    const io = new IntersectionObserver(
      (entries) => setVisible(entries.some((e) => e.isIntersecting)),
      { root, rootMargin: `${PREFETCH_PX}px 0px` },
    );
    io.observe(el);
    onCleanup(() => io.disconnect());
  });

  // Also runs when `stop` flips back to false, so a scroller that is not filled
  // yet keeps loading pages until it is or the list ends.
  createEffect(() => {
    if (visible() && !props.stop()) props.onVisible();
  });

  return <div ref={el} aria-hidden="true" class="h-px w-full shrink-0" />;
}
