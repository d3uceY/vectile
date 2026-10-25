import { createEffect } from "solid-js";
import { driver, type DriveStep } from "driver.js";
import { pickFolder } from "../../lib/api";
import { useAppStore } from "../../lib/store";

const SEEN_KEY = "vectile.setup-seen";
const DEFAULT_COLLECTION = "My Library";

/** One-time setup tour for a fresh (empty) library. Uses driver.js to walk
    Model → Sources → Index → Search, each step's popover carrying the real
    action. Starts once onMount has offered the first-run dialogs and they are
    closed (the model dialog hands straight off to the OCR offer), so it never
    covers onboarding. Search is gated on an embedding model, so a tour that
    starts without one opens on Settings → Model and holds there until a model
    is ready; the later steps would otherwise point at disabled controls.
    Adds the chosen folder into the default "My Library" collection. Any close,
    skip, or finish marks the tour as seen so it never runs again.
    Renders nothing. */
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
    if (!store.firstRunDone()) return; // onMount hasn't offered the first-run dialogs yet
    // Read the dialog signals last so this effect stays live and re-runs when
    // they close; the tour then starts on its own.
    if (store.modelDialogOpen() || store.ocrSetupOpen()) return;
    started = true;
    start();
  });

  const start = () => {
    const reduced = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const needsModel = !store.canIndex();
    store.openSettings(needsModel ? "model" : "sources");

    const modelStep: DriveStep = {
      element: "#setup-model",
      waitForElement: 2000,
      popover: {
        title: "Get a model",
        description:
          "Search runs on a local embedding model, so nothing works without one. Download a model below, or import a .gguf you already have.",
        nextBtnText: "Continue",
        onNextClick: (_el, _step, opts) => {
          if (!store.canIndex()) {
            store.pushToast("Add a model to continue the tour", "neutral");
            return; // stay put until one is ready
          }
          store.openSettings("sources");
          opts.driver.moveNext();
        },
      },
    };

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
        ...(needsModel ? [modelStep] : []),
        {
          element: "#setup-add-folder",
          waitForElement: 1500,
          popover: {
            title: "Add a folder",
            description:
              "A folder of notes, books, or project files. It lands in the My Library collection, under Project folders.",
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
              if (!store.canIndex()) {
                store.pushToast("Indexing needs an active model", "neutral");
                return;
              }
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
