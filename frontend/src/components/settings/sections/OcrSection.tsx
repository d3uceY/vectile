import { createSignal, onMount, Show } from "solid-js";
import { useAppStore } from "../../../lib/store";
import { Button, ConfirmDialog, Toggle } from "../../ui/primitives";
import { OcrNavIcon } from "../../ui/nav-icons";
import { OcrCard } from "../../ui/OcrCard";
import { FieldList, Section } from "../fields";
import { useSettings } from "../context";

export function OcrSection() {
  const store = useAppStore();
  const { draft, setOCR } = useSettings();

  onMount(() => void store.loadOCRState());

  const [confirmReindex, setConfirmReindex] = createSignal(false);
  const [reindexBusy, setReindexBusy] = createSignal(false);

  const reindexForOCR = async () => {
    setReindexBusy(true);
    try {
      await store.startIndexAll(true);
    } finally {
      setReindexBusy(false);
      setConfirmReindex(false);
    }
  };

  return (
    <Section
      icon={<OcrNavIcon size={16} />}
      title="OCR"
      note="Read PDFs that are photos of pages instead of text."
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
          <div class="flex flex-wrap items-center justify-between gap-3">
            <p class="max-w-[46ch] text-[12px] font-medium leading-4 text-muted">
              PDFs you already indexed keep the text they have. Re-indexing reads them
              again, this time with OCR.
            </p>
            <Button size="sm" variant="outline" onClick={() => setConfirmReindex(true)}>
              Re-index everything
            </Button>
          </div>
        </Show>

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
