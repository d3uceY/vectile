import { For, Show } from "solid-js";
import { openExternal } from "../../lib/update";
import type { CatalogModel, ModelDownloadState, ModelInfo } from "../../lib/types";
import { Button } from "./primitives";
import { CatalogModelCard } from "./CatalogModelCard";

/**
 * Onboarding sheet shown on launch when no embedding model is installed. Offers
 * the curated catalog with an in-app download, or an escape hatch to import a
 * .gguf / browse Hugging Face. Matches the UpdateDialog notebook world.
 */
export function ModelDownloadDialog(props: {
  open: boolean;
  recommended: CatalogModel[];
  downloadState: ModelDownloadState | null;
  installedModels: ModelInfo[];
  onDownload: (key: string) => void;
  onUninstall: (file: string) => void;
  onCancel: () => void;
  onImport: () => void;
  onDismiss: () => void;
}) {
  return (
    <Show when={props.open}>
      <div
        class="fixed inset-0 z-50 flex items-center justify-center bg-ink/30 p-4"
        role="dialog"
        aria-modal="true"
        aria-label="Add an embedding model"
        onClick={props.onDismiss}
      >
        <div
          class="od-scale-in w-104 rounded-[12px] border border-line bg-paper p-[22px] shadow-overlay"
          onClick={(e) => e.stopPropagation()}
        >
         
          <h3 class="mt-2 text-[17px] font-semibold tracking-[-0.01em] text-ink">Add an embedding model</h3>
          <p class="mt-2.5 text-[13px] font-medium leading-[1.7] text-muted">
            Vectile searches by meaning. It needs one of these, or bring your own.
          </p>

          <div class="mt-4 max-h-76 space-y-2 overflow-y-auto pr-1">
            <For each={props.recommended}>
              {(m) => (
                <CatalogModelCard
                  model={m}
                  installedModels={props.installedModels}
                  downloadState={props.downloadState}
                  onDownload={props.onDownload}
                  onUninstall={props.onUninstall}
                  onCancel={props.onCancel}
                />
              )}
            </For>
          </div>

          <div class="mt-5 flex flex-wrap items-center justify-between gap-2">
            <Button size="sm" variant="outline" onClick={props.onImport}>
              Import my own .gguf
            </Button>
            <div class="flex gap-2">
              <Button
                size="sm"
                variant="outline"
                onClick={() => openExternal("https://huggingface.co/models?search=embedding")}
              >
                Browse Hugging Face
              </Button>
              <Button size="sm" variant="outline" onClick={props.onDismiss}>
                Not now
              </Button>
            </div>
          </div>
        </div>
      </div>
    </Show>
  );
}
