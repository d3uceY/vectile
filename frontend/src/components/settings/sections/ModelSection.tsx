import { createEffect, createSignal, For, Show } from "solid-js";
import { useAppStore } from "../../../lib/store";
import { importModel, pickModelFile } from "../../../lib/api";
import type { ModelInfo } from "../../../lib/types";
import { Button, ConfirmDialog, InfoTip, Select, StatusPill } from "../../ui/primitives";
import { CloseIcon, UploadIcon } from "../../ui/icons";
import { ModelNavIcon } from "../../ui/nav-icons";
import { CatalogModelCard } from "../../ui/CatalogModelCard";
import { openExternal } from "../../../lib/update";
import { STATIC_BOUNDS } from "../bounds";
import { FieldList, NumField, RangeField, Section, SubHeading } from "../fields";

export function ModelSection() {
  const store = useAppStore();

  const activeModel = (): ModelInfo | null => store.models().find((m) => m.isActive) ?? null;

  const [selModel, setSelModel] = createSignal("");
  const [switchBusy, setSwitchBusy] = createSignal(false);
  const [confirmDim, setConfirmDim] = createSignal<{ path: string; name: string } | null>(null);
  const [confirmBusy, setConfirmBusy] = createSignal(false);

  createEffect(() => {
    const m = activeModel();
    if (m && !switchBusy() && confirmDim() === null) setSelModel(m.path);
  });

  const cpuCount = (): number => (store.cpuCount() > 0 ? store.cpuCount() : 64);

  const [modelCtx, setModelCtx] = createSignal(0);
  const [modelBatch, setModelBatch] = createSignal(32);
  const [modelThreads, setModelThreads] = createSignal(0);
  const [syncedActivePath, setSyncedActivePath] = createSignal<string | null>(null);
  createEffect(() => {
    const m = activeModel();
    if (m && m.path !== syncedActivePath()) {
      setSyncedActivePath(m.path);
      setModelCtx(m.contextWindow);
      setModelBatch(m.batchSize);
      setModelThreads(m.threads);
    }
  });

  const modelLabel = (m: ModelInfo) => (m.dimensions > 0 ? `${m.name} · ${m.dimensions}d` : m.name);

  const switchModel = async (path: string) => {
    if (path === activeModel()?.path) return;
    setSwitchBusy(true);
    setSelModel(path); // preview the pending choice in the dropdown
    try {
      const r = await store.setActiveModel(path);
      if (r.needsRebuild) setConfirmDim({ path, name: r.name });
      else setSelModel(activeModel()?.path ?? ""); // applied; resync from the store
    } catch (err) {
      setSelModel(activeModel()?.path ?? ""); // rejected; revert the dropdown
      store.pushToast(`Couldn't switch model: ${err}`, "danger");
    } finally {
      setSwitchBusy(false);
    }
  };

  const confirmDimSwitch = async () => {
    const c = confirmDim();
    if (!c) return;
    setConfirmBusy(true);
    try {
      await store.setActiveModel(c.path, true);
      setConfirmDim(null);
    } catch (err) {
      setSelModel(activeModel()?.path ?? ""); // failed; revert the dropdown
      store.pushToast(`Couldn't switch model: ${err}`, "danger");
    } finally {
      setConfirmBusy(false);
    }
  };

  const importModelFlow = async () => {
    const p = await pickModelFile();
    if (!p) return;
    try {
      const m = await importModel(p);
      await store.loadModels();
      store.pushToast(`Imported ${m.name}`, "success");
    } catch (err) {
      store.pushToast(`Import failed: ${err}`, "danger");
    }
  };

  const removeModel = async (m: ModelInfo) => {
    await store.deleteModel(m.path, m.name);
  };

  const saveModelSettings = async () => {
    const m = activeModel();
    if (!m) return;
    await store.updateModelSettings(m.id, modelCtx(), modelBatch(), modelThreads());
  };

  return (
    <Section
      icon={<ModelNavIcon size={16} />}
      title="Model"
      note="Drop a .gguf into the models folder, or import one below."
    >
      <div class="space-y-6">
        <div class="space-y-2">
          <div class="flex flex-wrap items-center gap-3">
            <StatusPill state={store.modelState()} name={store.modelName()} />
          </div>
          <div class="truncate font-mono text-[12px] text-muted" title={store.status()?.modelPath ?? ""}>
            {store.status()?.modelPath ?? "…"}
          </div>
        </div>

        <FieldList>
          <div class="flex items-center justify-between gap-4 py-3.5">
            <span class="text-[13px] font-medium text-ink">Active model</span>
            <Select
              aria-label="Active model"
              placeholder="No model installed"
              value={selModel()}
              options={store.models().map((m) => ({ value: m.path, label: modelLabel(m) }))}
              onChange={(v) => void switchModel(v)}
            />
          </div>
          <div class="flex items-center justify-between gap-4 py-3.5">
            <span class="text-[13px] font-medium text-ink">Add a model file</span>
            <Button size="sm" variant="outline" onClick={() => void importModelFlow()}>
              <UploadIcon size={14} />
              Import model…
            </Button>
          </div>
        </FieldList>

        <div>
          <SubHeading
            action={
              <button
                type="button"
                class="text-[12px] font-semibold text-leaf-deep hover:underline"
                onClick={() =>
                  openExternal(
                    "https://huggingface.co/models?library=gguf&sort=trending&search=embedding",
                  )
                }
              >
                Browse Hugging Face
              </button>
            }
          >
            Get a model
          </SubHeading>
          <div class="space-y-2.5">
            <For each={store.recommended()}>
              {(m) => (
                <CatalogModelCard
                  model={m}
                  installedModels={store.models()}
                  downloadState={store.downloadState()}
                  onDownload={(k) => store.downloadModelByKey(k)}
                  onUninstall={(f) => store.uninstallCatalogFile(f)}
                  onCancel={() => store.cancelDownload()}
                />
              )}
            </For>
          </div>
        </div>

        <Show when={activeModel()}>
          {(m) => (
            <div class="rounded-[8px] border border-line bg-surface p-5">
              <p class="mb-1 flex items-center gap-1.5 text-[13px] font-semibold text-ink">
                {m().name} settings
                <InfoTip text="Each model carries its own settings. Context window 0 falls back to the model's native maximum (shown here when the .gguf reports one); threads 0 uses all cores." />
              </p>
              <div class="mb-3 truncate font-mono text-[12px] text-muted">
                Dimensions: {m().dimensions > 0 ? m().dimensions : "auto"}
              </div>
              <FieldList>
                <NumField
                  label="Context window (tokens)"
                  value={modelCtx()}
                  onChange={setModelCtx}
                  hint="How many tokens the model can read at once. 0 = the model's native maximum. Raise it for long chunks, lower it to save memory."
                  min={0}
                  max={8192}
                  step={256}
                />
                <NumField
                  label="Embedding batch size"
                  value={modelBatch()}
                  onChange={setModelBatch}
                  hint="How many chunks get fed to the model at once. A bigger number finishes indexing faster but uses more memory while it runs. If a large library makes the app stall, drop it to something like 16."
                  min={STATIC_BOUNDS.embedding_batch_size.min}
                  max={STATIC_BOUNDS.embedding_batch_size.max}
                  step={STATIC_BOUNDS.embedding_batch_size.step}
                />
                <RangeField
                  label="CPU threads"
                  value={modelThreads()}
                  onChange={setModelThreads}
                  hint={`0 = auto, which uses all ${cpuCount()} logical cores. Drag to reserve some for the rest of the system. Lower it if indexing starves other apps.`}
                  min={0}
                  max={cpuCount()}
                  step={1}
                  format={(n) => (n <= 0 ? "auto" : String(Math.round(n)))}
                  suffix={`of ${cpuCount()}`}
                />
              </FieldList>
              <div class="mt-2 flex justify-end">
                <Button size="sm" onClick={() => void saveModelSettings()}>
                  Save model settings
                </Button>
              </div>
            </div>
          )}
        </Show>

        <Show when={store.models().length > 0}>
          <div>
            <SubHeading>Installed models</SubHeading>
            <ul class="divide-y divide-line overflow-hidden rounded-[8px] border border-line bg-surface pb-1.5">
              <For each={store.models()}>
                {(m) => (
                  <li class="flex items-center gap-2 px-3 py-2">
                    <span class="flex-1 truncate font-mono text-[12px] text-muted" title={m.path}>
                      {modelLabel(m)}
                    </span>
                    {m.isActive && (
                      <span class="shrink-0 rounded-full border border-transparent bg-mint px-2 py-0.5 text-[11px] font-semibold text-leaf-deep">
                        active
                      </span>
                    )}
                    <button
                      class="shrink-0 text-faint transition-colors hover:text-danger disabled:opacity-40 disabled:pointer-events-none"
                      aria-label={`Remove ${m.name}`}
                      disabled={m.isActive}
                      onClick={() => void removeModel(m)}
                    >
                      <CloseIcon size={14} />
                    </button>
                  </li>
                )}
              </For>
            </ul>
          </div>
        </Show>

        <Show when={store.models().length === 0}>
          <p class="text-[13px] font-medium text-muted">No models yet. Import a .gguf or drop one into the models folder.</p>
        </Show>

        <ConfirmDialog
          open={confirmDim() !== null}
          title="Switching changes the embedding dimension"
          body={
            <>
              <p>
                <span class="font-medium text-ink">{confirmDim()?.name}</span> uses a
                different embedding dimension than the current model. Every indexed
                collection will need to be re-indexed before meaning search works again,
                and all existing embeddings will be cleared.
              </p>
              <p class="mt-2">Switch anyway?</p>
            </>
          }
          confirmLabel="Switch & re-index"
          busyLabel="Switching…"
          busy={confirmBusy()}
          onCancel={() => {
            setConfirmDim(null);
            setSelModel(activeModel()?.path ?? ""); // keep the previous model
          }}
          onConfirm={() => void confirmDimSwitch()}
        />
      </div>
    </Section>
  );
}
