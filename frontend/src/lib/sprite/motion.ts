import { createSignal, onCleanup, type Accessor } from "solid-js";

/** True while the user asks for reduced motion, so a player can freeze. */
export function useReducedMotion(): Accessor<boolean> {
  const query =
    typeof window === "undefined"
      ? undefined
      : window.matchMedia("(prefers-reduced-motion: reduce)");
  const [reduced, setReduced] = createSignal(query?.matches ?? false);

  if (query) {
    const onChange = (event: MediaQueryListEvent) => setReduced(event.matches);
    query.addEventListener("change", onChange);
    onCleanup(() => query.removeEventListener("change", onChange));
  }

  return reduced;
}
