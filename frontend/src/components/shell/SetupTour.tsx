import { createEffect } from "solid-js";
import { driver } from "driver.js";
import { pickFolder } from "../../lib/api";
import { useAppStore } from "../../lib/store";

const SEEN_KEY = "vectile.setup-seen";
const DEFAULT_COLLECTION = "My Library";

/** One-time, three-step setup tour for a fresh (empty) library. Uses driver.js
    to walk Settings → Index → Search, each step's popover carrying the real
    action. Starts only once an active model exists (so it never collides with
    first-run model onboarding) and adds the chosen folder into the default
    "My Library" collection. Any close, skip, or finish marks the tour as seen
    so it never runs again. Renders nothing. */
export function SetupTour() {
  const store = useAppStore();
  let started = false;

  createEffect(() => {
    if (started || localStorage.getItem(SEEN_KEY)) return;
    if (store.status() === null) return; // status not fetched yet
    if (store.collections().length > 0) {
      localStorage.setItem(SEEN_KEY, "1"); // already has a library
      return;
    }
    if (!store.canIndex()) return; // wait for the first active model
    started = true;
    start();
  });

  const start = () => {
    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    store.openSettings("sources");

    const tour = driver({
      animate: !reduced,
      showProgress: true,
      progressText: "{{current}} of {{total}}",
      nextBtnText: "Next",
      prevBtnText: "Back",
      doneBtnText: "Done",
      allowClose: true,
      overlayColor: "#1b2226",
      overlayOpacity: 0.35,
      stagePadding: 6,
      onPopoverRender: (popover) => {
        popover.nextButton.classList.add("setup-next");
        popover.previousButton.classList.add("setup-prev");
        popover.closeButton.title = "Skip tour";
        popover.closeButton.setAttribute("aria-label", "Skip tour");
      },
      onDestroyed: () => localStorage.setItem(SEEN_KEY, "1"),
      steps: [
        {
          element: "#setup-add-folder",
          waitForElement: 1500,
          popover: {
            title: "Add a folder",
            description:
              "Point vectile at a folder of notes, books, or project files. It joins your My Library collection, which you'll find here under Project folders.",
            nextBtnText: "Choose a folder",
            onNextClick: async (_el, _step, opts) => {
              const dir = await pickFolder("Choose a folder to index");
              if (!dir) return; // cancelled; stay on this step
              const ok = await store.addSource("project", DEFAULT_COLLECTION, dir);
              if (!ok) return;
              store.setView("index");
              opts.driver.moveNext();
            },
          },
        },
        {
          element: "#setup-index-all",
          waitForElement: 1500,
          popover: {
            title: "Index it",
            description:
              "Index all turns that folder into a searchable collection.",
            nextBtnText: "Index now",
            onNextClick: (_el, _step, opts) => {
              void store.startIndexAll();
              store.setView("search");
              opts.driver.moveNext();
            },
          },
        },
        {
          element: "#search-input",
          waitForElement: 1500,
          popover: {
            title: "Search your library",
            description:
              "Ask anything you half-remember. Cmd/Ctrl+K focuses this box from anywhere.",
            doneBtnText: "Done",
            onDoneClick: (_el, _step, opts) => opts.driver.destroy(),
          },
        },
      ],
    });
    tour.drive();
  };

  return null;
}
