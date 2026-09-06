import { Show } from "solid-js";
import { baseName, fmtBytes } from "../../lib/format";
import type { CatalogModel, ModelDownloadState, ModelInfo } from "../../lib/types";
import { Button } from "./primitives";
import { DownloadProgressBar } from "./DownloadProgressBar";

/** One catalog model row: name + recommended badge, spec line, description, and
    a Download/installed/Uninstall action, swapping to a live progress bar while
    that model downloads. Shared by the onboarding dialog and the Settings panel. */
export function CatalogModelCard(props: {
  model: CatalogModel;
  installedModels: ModelInfo[];
  downloadState: ModelDownloadState | null;
  onDownload: (key: string) => void;
  onUninstall: (file: string) => void;
  onCancel: () => void;
}) {
  const m = () => props.model;
  const installed = () => props.installedModels.some((x) => baseName(x.path) === m().file);
  const active = () => props.downloadState?.key === m().key;

  return (
    <div class="rounded-control border border-line bg-paper p-3">
      <div class="flex items-center justify-between gap-3">
        <div class="min-w-0">
          <p class="flex items-center gap-1.5 text-[13px] font-medium text-ink">
            {m().name}
            <Show when={m().recommended}>
              <span class="rounded-control bg-amber-soft px-1.5 py-0.5 text-[10.5px] font-medium text-amber-deep">
                recommended
              </span>
            </Show>
          </p>
          <p class="data mt-0.5 text-muted">
            {m().dimensions} dims · {fmtBytes(m().sizeBytes)} · {m().quantization}
            {m().language ? ` · ${m().language}` : ""}
          </p>
          <p class="mt-0.5 text-[12.5px] text-faint">{m().description}</p>
        </div>
        <Show when={!active()}>
          <Show
            when={!installed()}
            fallback={
              <div class="flex items-center gap-2">
                <span class="rounded-control bg-surface-2 px-1.5 py-0.5 text-[10.5px] font-medium text-muted">
                  installed
                </span>
                <Button size="sm" variant="danger" onClick={() => props.onUninstall(m().file)}>
                  Uninstall
                </Button>
              </div>
            }
          >
            <Button size="sm" onClick={() => props.onDownload(m().key)}>
              Download
            </Button>
          </Show>
        </Show>
      </div>
      <Show when={active()}>
        <div class="mt-2">
          <DownloadProgressBar
            progress={{
              key: m().key,
              downloaded: props.downloadState!.downloaded,
              total: props.downloadState!.total,
              percent: props.downloadState!.percent,
              speed: props.downloadState!.speed,
            }}
            onCancel={props.onCancel}
          />
        </div>
      </Show>
    </div>
  );
}
