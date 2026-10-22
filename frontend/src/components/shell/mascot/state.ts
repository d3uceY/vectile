import { createMemo, type Accessor } from "solid-js";
import { useAppStore } from "../../../lib/store";
import type { ActiveMascotState } from "./types";

/** Which sheet the mascot should be playing right now.

    Idle is the floor, indexing outranks searching, and searching is tied to the
    Search view rather than to a running query, so the sheet plays for as long as
    that page is open. Each of the three honours its own switch from Settings
    (config.gui.mascot); turning all three off hides Vexter, which is the one
    case where there is no sheet at all.

    There is no minimum time on screen: the sheet follows the state directly, so
    leaving the Search view swaps straight back to idle. */
export function useMascotState(): Accessor<ActiveMascotState | null> {
  const store = useAppStore();

  return createMemo<ActiveMascotState | null>(() => {
    const m = store.config()?.gui.mascot;
    // A config written before a switch existed keeps that sheet on.
    const show = (k: "show_idle" | "show_searching" | "show_indexing") =>
      m ? m[k] !== false : true;

    if ((store.indexing() || store.indexProgress() !== null) && show("show_indexing")) {
      return "indexing";
    }
    if (store.view() === "search" && show("show_searching")) {
      return "searching";
    }
    return show("show_idle") ? "idle" : null;
  });
}
