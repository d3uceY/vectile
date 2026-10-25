import { createSignal, For, onMount, Show } from "solid-js";
import { useAppStore } from "../../../lib/store";
import { Button, Chip, ConfirmDialog, Switch, Toggle } from "../../ui/primitives";
import { OcrNavIcon } from "../../ui/nav-icons";
import { OcrCard } from "../../ui/OcrCard";
import { DownloadProgressBar } from "../../ui/DownloadProgressBar";
import { CloseIcon, PlusIcon, ReindexIcon } from "../../ui/icons";
import { FieldList, Section, SubHeading } from "../fields";
import { OcrLanguageDialog } from "../OcrLanguageDialog";
import { useSettings } from "../context";

/** English ships with the plugin and is what a run falls back to, so it is the
    one language whose data cannot be deleted. */
const ENGLISH = "eng";

export function OcrSection() {
  const store = useAppStore();
  const { draft, setOCR } = useSettings();

  onMount(() => {
    void store.loadOCRState();
    void store.loadOCRLanguages();
  });

  const [confirmReindex, setConfirmReindex] = createSignal(false);
  const [reindexBusy, setReindexBusy] = createSignal(false);
  const [pickerOpen, setPickerOpen] = createSignal(false);

  const reindexForOCR = async () => {
    setReindexBusy(true);
    try {
      await store.startIndexAll(true);
    } finally {
      setReindexBusy(false);
      setConfirmReindex(false);
    }
  };

  const installed = () => store.ocrState()?.languages ?? [];
  const chosen = () => draft()?.ocr.languages ?? [];
  const busy = () => store.ocrLangInstall() !== null;
  const nameOf = store.ocrLanguageName;

  const rows = () =>
    [...new Set([...chosen(), ...installed()])].sort((a, b) => {
      if ((a === ENGLISH) !== (b === ENGLISH)) return a === ENGLISH ? -1 : 1;
      return nameOf(a).localeCompare(nameOf(b));
    });

  const has = (code: string) => installed().includes(code);
  const on = (code: string) => chosen().includes(code);

  const setLanguage = (code: string, enabled: boolean) => {
    const next = enabled
      ? [...chosen().filter((c) => c !== code), code]
      : chosen().filter((c) => c !== code);
    if (next.length === 0) next.push(ENGLISH);
    setOCR({ languages: next });
  };

  const pickLanguage = (code: string) => {
    setPickerOpen(false);
    if (!has(code)) void store.installOCRLanguage(code);
    if (!on(code)) setLanguage(code, true);
  };

  const removeData = async (code: string) => {
    await store.removeOCRLanguage(code);
    if (on(code)) setLanguage(code, false);
  };

  return (
    <Section
      icon={<OcrNavIcon size={16} />}
      title="OCR"
      note="Optional plugin for PDFs that are photos of pages."
    >
      <div class="space-y-6">
        <OcrCard
          state={store.ocrState()}
          onInstall={() => void store.installOCRPlugin()}
          onCancel={() => void store.cancelOCRPluginInstall()}
          onRemove={() => void store.removeOCRPlugin()}
        />

        <Show when={store.ocrState()?.supported}>
          <FieldList>
            <Toggle
              checked={draft()!.ocr.enabled}
              onChange={(v) => setOCR({ enabled: v })}
              label="Use OCR for pages with no text"
              description="Only runs for pages that come back empty."
              hint="Applies when you save settings. A PDF with real text is never sent through OCR, so this costs nothing on documents that already have a text layer."
            />
          </FieldList>
        </Show>

        <Show when={store.ocrState()?.installed}>
          <div>
            <SubHeading
              action={
                <Button size="sm" variant="ghost" disabled={busy()} onClick={() => setPickerOpen(true)}>
                  <PlusIcon size={14} />
                  Add language
                </Button>
              }
            >
              Languages
            </SubHeading>
            <p class="mb-2.5 max-w-[56ch] text-[12px] font-medium leading-4 text-muted">
              English comes with the plugin. Any other language downloads once, then reads
              alongside it.
            </p>

            <ul class="divide-y divide-line overflow-hidden rounded-[8px] border border-line bg-surface">
              <For each={rows()}>
                {(code) => (
                  <li class="px-3 py-2">
                    <div class="flex items-center gap-3">
                      <div class="flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-1">
                        <span class="truncate text-[13px] font-medium text-ink">{nameOf(code)}</span>
                        <span class="shrink-0 font-mono text-[11px] text-muted">{code}</span>
                        <Show when={!has(code) && store.ocrLangInstall()?.code !== code}>
                          <Chip tone="amber">no data</Chip>
                        </Show>
                      </div>

                      <Show when={!has(code)}>
                        <Button
                          size="sm"
                          variant="outline"
                          disabled={busy()}
                          onClick={() => void store.installOCRLanguage(code)}
                        >
                          Download
                        </Button>
                      </Show>

                      <Show when={has(code) && code !== ENGLISH}>
                        <button
                          type="button"
                          class="shrink-0 text-faint transition-colors duration-100 hover:text-danger disabled:opacity-40"
                          aria-label={`Remove ${nameOf(code)} language data`}
                          disabled={busy()}
                          onClick={() => void removeData(code)}
                        >
                          <CloseIcon size={13} />
                        </button>
                      </Show>

                      <Show when={has(code)}>
                        <Switch
                          checked={on(code)}
                          onChange={(v) => setLanguage(code, v)}
                          label={`Read ${nameOf(code)}`}
                        />
                      </Show>
                    </div>

                    <Show when={store.ocrLangInstall()?.code === code}>
                      <div class="mt-2">
                        <DownloadProgressBar
                          progress={store.ocrLangInstall()!}
                          label={`Downloading ${nameOf(code)}`}
                          onCancel={() => void store.cancelOCRPluginInstall()}
                        />
                      </div>
                    </Show>
                  </li>
                )}
              </For>
            </ul>
          </div>
        </Show>

        <Show when={store.ocrState()?.installed}>
          <div class="flex flex-wrap items-center justify-between gap-3">
            <p class="max-w-[46ch] text-[12px] font-medium leading-4 text-muted">
              Indexed PDFs keep their current text. Re-index to read them with OCR.
            </p>
            <Button size="sm" variant="outline" onClick={() => setConfirmReindex(true)}>
              <ReindexIcon size={14} />
              Re-index everything
            </Button>
          </div>
        </Show>

        <OcrLanguageDialog
          open={pickerOpen()}
          installed={installed()}
          download={store.ocrLangInstall()}
          onPick={pickLanguage}
          onCancelDownload={() => void store.cancelOCRPluginInstall()}
          onDismiss={() => setPickerOpen(false)}
        />

        <ConfirmDialog
          open={confirmReindex()}
          title="Re-index everything?"
          body={
            <p>
              Every file in every library is read again so scanned PDFs get read as
              text. On a large library this takes a while. Nothing is deleted: each
              file's chunks are replaced as it is re-read.
            </p>
          }
          confirmLabel="Re-index all"
          busyLabel="Starting…"
          busy={reindexBusy()}
          onCancel={() => setConfirmReindex(false)}
          onConfirm={() => void reindexForOCR()}
        />
      </div>
    </Section>
  );
}
