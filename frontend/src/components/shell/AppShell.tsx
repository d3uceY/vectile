import { createSignal, onCleanup, onMount, Show } from "solid-js";
import * as api from "../../lib/api";
import { useAppStore } from "../../lib/store";
import { fetchLatestRelease, isDesktop, isNewer } from "../../lib/update";
import { ToastStack } from "../ui/primitives";
import { UpdateDialog } from "../ui/UpdateDialog";
import { ModelDownloadDialog } from "../ui/ModelDownloadDialog";
import { Sidebar } from "./Sidebar";
import { StatusStrip } from "./StatusStrip";
import { SearchView } from "../search/SearchView";
import { LibraryView } from "../library/LibraryView";
import { BrowseView } from "../library/BrowseView";
import { IndexView } from "../index/IndexView";
import { SettingsView } from "../settings/SettingsView";
import { SetupTour } from "./SetupTour";

export function AppShell() {
  const store = useAppStore();
  const [version, setVersion] = createSignal<string | null>(null);
  const [updateInfo, setUpdateInfo] = createSignal<{ latest: string; current: string } | null>(null);

  onMount(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        store.focusSearch();
      }
    };
    window.addEventListener("keydown", onKey);
    onCleanup(() => window.removeEventListener("keydown", onKey));
  });

  onMount(() => {
    void (async () => {
      const v = await api.getVersion().catch(() => null);
      if (!v) return;
      setVersion(v);
      if (!isDesktop) return;
      const latest = await fetchLatestRelease();
      if (latest && isNewer(latest, v)) setUpdateInfo({ latest, current: v });
    })();
  });

  return (
    <div class="relative flex h-full overflow-hidden bg-paper text-ink">
      <Sidebar />

      <div class="relative flex min-w-0 flex-1 flex-col">
        <StatusStrip version={version() ?? undefined} />
        {/* One non-scrolling frame; each view owns its scroll (per-view scroll) */}
        <main class="relative min-h-0 flex-1 overflow-hidden">
          <div class="relative mx-auto h-full w-full max-w-[61.25rem] px-5 py-7 md:px-8">
            <Show when={store.view() === "search"}>
              <SearchView />
            </Show>
            <Show when={store.view() === "library"}>
              <LibraryView />
            </Show>
            <Show when={store.view() === "browse"}>
              <BrowseView />
            </Show>
            <Show when={store.view() === "index"}>
              <IndexView />
            </Show>
            <Show when={store.view() === "settings"}>
              <SettingsView />
            </Show>
          </div>
        </main>
      </div>

      <UpdateDialog
        open={updateInfo() !== null}
        latest={updateInfo()?.latest ?? ""}
        current={updateInfo()?.current ?? ""}
        onDismiss={() => setUpdateInfo(null)}
      />

      <ModelDownloadDialog
        open={store.modelDialogOpen()}
        recommended={store.recommended()}
        downloadState={store.downloadState()}
        installedModels={store.models()}
        onDownload={(k) => store.downloadModelByKey(k)}
        onUninstall={(f) => store.uninstallCatalogFile(f)}
        onCancel={() => store.cancelDownload()}
        onImport={() => store.importModelFile()}
        onDismiss={() => store.closeModelDialog()}
      />

      <ToastStack toasts={store.toasts()} onDismiss={(id) => store.dismissToast(id)} />
      <SetupTour />
    </div>
  );
}
