import { createEffect, createMemo, createSignal, onCleanup, type Accessor } from "solid-js";
import { useAppStore } from "../../../lib/store";
import type { ActiveMascotState } from "./types";

/** Minimum time the searching sheet stays on screen once it starts, in ms.
    Typing is bursty and the debounced query often comes back in a few hundred
    milliseconds, so without a floor the sheet flickers on every keystroke. The
    search still ends when it ends; the sheet just finishes its beat. */
const SEARCH_MIN_MS = 5000;

/** Which sheet the mascot should be playing right now.

    Idle is the floor and indexing outranks searching. Searching belongs to the
    act of searching, not to the Search view being open: the sheet starts on the
    first keystroke (the input's debounce is ignored) and stops when the last
    keystroke's search settles, held to SEARCH_MIN_MS so it never flashes. Each
    of the three honours its own switch from Settings (config.gui.mascot);
    turning all three off hides Vexter, which is the one case where there is no
    sheet at all. */
export function useMascotState(): Accessor<ActiveMascotState | null> {
  const store = useAppStore();

  /** Typing right now, or a query is in flight. */
  const searchActive = createMemo(
    () =>
      store.view() === "search" &&
      (store.searchTyping() || store.searchState() === "searching"),
  );

  /** searchActive, but latched on for at least SEARCH_MIN_MS from the moment
      it first became true. */
  const [searchHeld, setSearchHeld] = createSignal(false);
  let since = 0;
  let timer: number | null = null;

  const stopTimer = () => {
    if (timer !== null) {
      window.clearTimeout(timer);
      timer = null;
    }
  };

  createEffect(() => {
    if (searchActive()) {
      stopTimer();
      if (!searchHeld()) {
        since = Date.now();
        setSearchHeld(true);
      }
      return;
    }
    if (!searchHeld() || timer !== null) return;
    const left = SEARCH_MIN_MS - (Date.now() - since);
    if (left <= 0) {
      setSearchHeld(false);
      return;
    }
    timer = window.setTimeout(() => {
      timer = null;
      setSearchHeld(false);
    }, left);
  });

  onCleanup(stopTimer);

  return createMemo<ActiveMascotState | null>(() => {
    const m = store.config()?.gui.mascot;
    // A config written before a switch existed keeps that sheet on.
    const show = (k: "show_idle" | "show_searching" | "show_indexing") =>
      m ? m[k] !== false : true;

    if ((store.indexing() || store.indexProgress() !== null) && show("show_indexing")) {
      return "indexing";
    }
    if ((searchActive() || searchHeld()) && show("show_searching")) {
      return "searching";
    }
    return show("show_idle") ? "idle" : null;
  });
}
