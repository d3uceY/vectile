import { createContext, createSignal, onCleanup, onMount, useContext, type JSX } from "solid-js";
import { Events } from "@wailsio/runtime";
import * as api from "./api";
import { baseName } from "./format";
import type {
  AppConfig,
  CatalogModel,
  Collection,
  Document,
  IndexCancelled,
  IndexComplete,
  IndexFailed,
  IndexFileProgress,
  IndexProgress,
  IndexState,
  MCPStatus,
  ModelDownloadError,
  ModelDownloadProgress,
  ModelDownloadState,
  ModelInfo,
  ModelState,
  SearchFilters,
  SearchResult,
  Source,
  Status,
  ViewId,
} from "./types";

export interface Toast {
  id: number;
  message: string;
  tone: "neutral" | "success" | "danger";
}

const DEFAULT_TOP_K = 12;

const defaultFilters = (topK = DEFAULT_TOP_K): SearchFilters => ({
  collection: "",
  sourceType: "",
  path: "",
  sender: "",
  dateFrom: "",
  dateTo: "",
  topK,
});

export function createAppStore() {
  const [view, setViewRaw] = createSignal<ViewId>("search");

  const [pendingLeave, setPendingLeave] = createSignal<ViewId | null>(null);

  const [settingsDraft, setSettingsDraftRaw] = createSignal<AppConfig | null>(null);
  const [settingsDirty, setSettingsDirty] = createSignal(false);

  const setSettingsDraft = (
    v: AppConfig | null | ((d: AppConfig | null) => AppConfig | null),
  ) => {
    setSettingsDraftRaw((d) =>
      typeof v === "function" ? (v as (x: AppConfig | null) => AppConfig | null)(d) : v,
    );
    setSettingsDirty(true);
  };
  const replaceSettingsDraft = (v: AppConfig | null) => setSettingsDraftRaw(v);
  const discardSettingsDraft = () => {
    setSettingsDraftRaw(null);
    setSettingsDirty(false);
  };

  const [cpuCount, setCpuCount] = createSignal(0);

  const [pendingSection, setPendingSection] = createSignal<string | null>(null);
  const openSettings = (section: string) => {
    setPendingSection(section);
    setViewRaw("settings");
  };
  const takePendingSettingsSection = (): string | null => {
    const s = pendingSection();
    setPendingSection(null);
    return s;
  };

  const setView = (next: ViewId) => {
    if (next === view()) return;
    if (view() === "settings" && settingsDirty()) {
      setPendingLeave(next);
      return;
    }
    setViewRaw(next);
  };
  const cancelLeave = () => setPendingLeave(null);
  const confirmLeave = (opts?: { discard?: boolean }) => {
    const next = pendingLeave();
    if (opts?.discard) {
      replaceSettingsDraft(null);
      setSettingsDirty(false);
    }
    setPendingLeave(null);
    if (next) setViewRaw(next);
  };

  const [modelState, setModelState] = createSignal<ModelState>("idle");
  const [modelName, setModelName] = createSignal("bge-m3");

  const [status, setStatus] = createSignal<Status | null>(null);
  const [collections, setCollections] = createSignal<Collection[]>([]);
  const [config, setConfig] = createSignal<AppConfig | null>(null);
  const [models, setModels] = createSignal<ModelInfo[]>([]);

  const [recommended, setRecommended] = createSignal<CatalogModel[]>([]);
  const [downloadState, setDownloadState] = createSignal<ModelDownloadState | null>(null);
  const [modelDialogOpen, setModelDialogOpen] = createSignal(false);
  const [modelDialogDismissed, setModelDialogDismissed] = createSignal(false);

  const [mcpStatus, setMCPStatus] = createSignal<MCPStatus | null>(null);
  const refreshMCP = async () => {
    try {
      setMCPStatus(await api.getMCPStatus());
    } catch {
      /* backend not ready yet */
    }
  };

  const [sources, setSources] = createSignal<Source[]>([]);
  const [documents, setDocuments] = createSignal<Document[]>([]);

  const [query, setQuery] = createSignal("");
  const [filters, setFilters] = createSignal<SearchFilters>(defaultFilters());
  const [results, setResults] = createSignal<SearchResult[]>([]);
  const [searchState, setSearchState] = createSignal<"idle" | "searching" | "done">("idle");

  const [scoreDisplay, setScoreDisplayRaw] = createSignal<"rank" | "percent">(
    localStorage.getItem("vectile.score-display") === "percent" ? "percent" : "rank",
  );
  const setScoreDisplay = (v: "rank" | "percent") => {
    setScoreDisplayRaw(v);
    localStorage.setItem("vectile.score-display", v);
  };

  const [expandedCollection, setExpandedCollection] = createSignal<string | null>(null);
  const [selectedDoc, setSelectedDoc] = createSignal<Document | null>(null);

  const [indexing, setIndexing] = createSignal(false);
  const [indexProgress, setIndexProgress] = createSignal<IndexProgress | null>(null);
  const [indexLast, setIndexLast] = createSignal<IndexComplete | null>(null);
  const [indexByCollection, setIndexByCollection] = createSignal<Record<string, IndexFileProgress>>({});
  const [indexAllActive, setIndexAllActive] = createSignal(false);

  const [toasts, setToasts] = createSignal<Toast[]>([]);
  let toastSeq = 0;

  const pushToast = (message: string, tone: Toast["tone"] = "neutral") => {
    const id = ++toastSeq;
    setToasts((t) => [...t, { id, message, tone }]);
    setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), 3600);
  };
  const dismissToast = (id: number) => setToasts((t) => t.filter((x) => x.id !== id));

  let searchSeq = 0;

  const runSearch = async (q: string, f: SearchFilters = filters()) => {
    const seq = ++searchSeq;
    setQuery(q);
    setFilters(f);
    if (!q.trim()) {
      setResults([]);
      setSearchState("idle");
      return;
    }
    setSearchState("searching");
    try {
      const res = await api.search(q, f);
      if (seq !== searchSeq) return; 
      setResults(res);
    } catch (err) {
      if (seq !== searchSeq) return;
      setResults([]);
      pushToast(`Search failed: ${err}`, "danger");
    } finally {
      if (seq === searchSeq) setSearchState("done");
    }
  };

  const clearSearch = () => {
    searchSeq++; 
    setQuery("");
    setFilters(defaultFilters(config()?.search_defaults.top_k));
    setResults([]);
    setSearchState("idle");
  };

  let searchInput: HTMLInputElement | null = null;
  const registerSearchInput = (el: HTMLInputElement | null) => {
    searchInput = el;
  };
  const focusSearch = () => {
    setView("search");
    setTimeout(() => searchInput?.focus(), 0);
  };

  const refresh = async () => {
    try {
      const st = await api.getStatus();
      setStatus(st);
      setModelState(st.modelState);
      setModelName(st.modelName);
      setCollections(await api.listCollections());
    } catch {
      /* backend not ready yet */
    }
  };

  const loadConfig = async () => {
    try {
      const c = await api.getConfig();
      setConfig(c);
      setFilters((f) =>
        f.topK === DEFAULT_TOP_K ? { ...f, topK: c.search_defaults.top_k } : f,
      );
    } catch {
      /* ignore */
    }
  };

  const loadSources = async (collectionId: number) => {
    try {
      const list = await api.listSources(collectionId);
      setSources((prev) => [
        ...prev.filter((s) => s.collectionId !== collectionId),
        ...list,
      ]);
    } catch {
      /* ignore */
    }
  };

  const loadLibrary = async () => {
    try {
      const cols = await api.listCollections();
      const srcs: Source[] = [];
      const docs: Document[] = [];
      for (const c of cols) {
        const cs = await api.listSources(c.id);
        srcs.push(...cs);
        for (const s of cs) {
          try {
            docs.push(...(await api.listDocuments(s.collectionId, s.id)));
          } catch {
            /* ignore */
          }
        }
      }
      setSources(srcs);
      setDocuments(docs);
    } catch {
      /* ignore */
    }
  };

  const saveSettings = async () => {
    const d = settingsDraft();
    if (!d) return;
    await api.setConfig(d);
    setConfig(d);
    setSettingsDirty(false);
    await refresh();
    const st = mcpStatus();
    const wantRunning = d.mcp.enabled;
    const portChanged = Boolean(st?.running && st.port !== d.mcp.port);
    if (st?.running && (!wantRunning || portChanged)) {
      await api.stopMCP();
    }
    if (wantRunning) {
      try {
        const url = await api.startMCP(d.mcp.port);
        if (!st?.running) pushToast(`MCP server on ${url}`, "success");
        void refreshMCP();
      } catch (err) {
        pushToast(`MCP server failed to start: ${err}`, "danger");
      }
    } else {
      void refreshMCP();
    }
    pushToast("Settings saved", "success");
  };

  const loadCPUCount = async () => {
    try {
      const n = await api.getCPUCount();
      setCpuCount(typeof n === "number" && Number.isFinite(n) && n > 0 ? n : 0);
    } catch {
      /* backend not ready yet */
    }
  };

  const loadModels = async () => {
    try {
      setModels([...(await api.listModels())]);
    } catch {
      /* backend not ready yet */
    }
  };


  const canIndex = () =>
    (models().some((m) => m.isActive) || modelState() === "loaded") &&
    modelState() !== "failed";

  const setActiveModel = async (path: string, force = false) => {
    const res = await api.setActiveModel(path, force);
    if (!res.needsRebuild) {
      await refresh();
      await loadModels();
      pushToast(`Active model: ${res.model.name}`, "success");
    }
    return { needsRebuild: res.needsRebuild, name: res.model.name };
  };

  const deleteModel = async (path: string, name: string): Promise<boolean> => {
    try {
      await api.deleteModel(path);
      pushToast(`Removed ${name}`, "success");
      void loadModels();
      return true;
    } catch (err) {
      pushToast(`Delete failed: ${err}`, "danger");
      return false;
    }
  };

  const updateModelSettings = async (
    id: number,
    contextWindow: number,
    batchSize: number,
    threads: number,
  ) => {
    try {
      await api.updateModelSettings(id, contextWindow, batchSize, threads);
      pushToast("Model settings saved", "success");
      void loadModels();
    } catch (err) {
      pushToast(`Settings failed: ${err}`, "danger");
    }
  };

  const loadRecommended = async () => {
    try {
      setRecommended(await api.listRecommendedModels());
    } catch {
      /* backend not ready yet */
    }
  };

  const downloadModelByKey = async (key: string) => {
    try {
      const started = await api.downloadModel(key);
      if (!started) pushToast("A download is already running", "neutral");
    } catch (err) {
      pushToast(`Download failed: ${err}`, "danger");
    }
  };

  const cancelDownload = async () => {
    try {
      await api.cancelModelDownload();
    } catch {
      /* ignore */
    }
  };

  const importModelFile = async () => {
    const p = await api.pickModelFile();
    if (!p) return;
    try {
      await api.importModel(p);
      await loadModels();
      await refresh();
      setModelDialogOpen(false);
      pushToast("Model imported", "success");
    } catch (err) {
      pushToast(`Import failed: ${err}`, "danger");
    }
  };

  const closeModelDialog = () => {
    setModelDialogOpen(false);
    setModelDialogDismissed(true);
  };

  const uninstallCatalogFile = async (file: string) => {
    const m = models().find((x) => baseName(x.path) === file);
    if (m) await deleteModel(m.path, m.name);
  };

  const hydrateDownload = async () => {
    try {
      const st = await api.getDownloadState();
      if (!st || typeof st !== "object" || typeof (st as { active?: unknown }).active !== "boolean") return;
      setDownloadState(st as ModelDownloadState);
    } catch {
      /* backend not ready yet */
    }
  };


  const preparingProgress = (name: string): IndexFileProgress => ({
    collection: name,
    file: "Preparing…",
    indexed: 0,
    total: 0,
  });

  const configuredNames = (): string[] => {
    const cfg = config();
    if (!cfg) return [];
    const disabled = new Set(cfg.disabled_collections);
    const names: string[] = [];
    if (cfg.obsidian_vaults.length && !disabled.has("obsidian")) names.push("obsidian");
    if (cfg.calibre_libraries.length && !disabled.has("calibre")) names.push("calibre");
    for (const [name, paths] of Object.entries(cfg.repositories)) {
      if (paths.length && !disabled.has(name)) names.push(name);
    }
    for (const [name, paths] of Object.entries(cfg.projects)) {
      if (paths.length && !disabled.has(name)) names.push(name);
    }
    return names;
  };

  const startIndex = async (name: string, force = false) => {
    setIndexLast(null);
    setIndexProgress(null);
    setIndexAllActive(false); // a single-collection run reloads on its own complete
    setIndexByCollection((m) => {
      const n = { ...m };
      delete n[name];
      return n;
    });
    const started = await api.indexCollection(name, force);
    if (!started) {
      pushToast("Another index is already running", "neutral");
      return;
    }
    setIndexByCollection((m) => ({ ...m, [name]: preparingProgress(name) }));
    setIndexing(true);
  };

  const startIndexAll = async (force = false) => {
    setIndexLast(null);
    setIndexProgress(null);
    const started = await api.indexAll(force);
    if (!started) {
      pushToast("Another index is already running", "neutral");
      return;
    }
    setIndexByCollection((m) => {
      const n = { ...m };
      for (const name of configuredNames()) n[name] = preparingProgress(name);
      return n;
    });
    setIndexAllActive(true);
    setIndexing(true);
  };

  const cancelIndex = async () => {
    if (await api.cancelIndexing()) pushToast("Cancelling index…", "neutral");
  };

  const runPrune = async (name: string) => {
    try {
      await api.prune(name);
      pushToast(`Pruned ${name || "all"}`, "success");
      await refresh();
    } catch (err) {
      pushToast(`Prune failed: ${err}`, "danger");
    }
  };

  const addSource = async (kind: string, name: string, path: string): Promise<boolean> => {
    try {
      await api.addSourcePath(kind, name, path);
      await loadConfig();
      discardSettingsDraft();
      return true;
    } catch (err) {
      pushToast(`Could not add ${name}: ${err}`, "danger");
      return false;
    }
  };

  const toggleCollection = async (name: string, enabled: boolean) => {
    try {
      await api.toggleCollectionEnabled(name, enabled);
      setCollections((cs) => cs.map((c) => (c.name === name ? { ...c, enabled } : c)));
      await loadConfig();
      discardSettingsDraft();
    } catch (err) {
      pushToast(`Toggle failed: ${err}`, "danger");
    }
  };

  const deleteSource = async (sourceId: number, label: string): Promise<boolean> => {
    try {
      const removed = await api.deleteSource(sourceId);
      pushToast(
        `Removed ${label} from the index · ${removed} chunk${removed === 1 ? "" : "s"}`,
        "success",
      );
      void refresh();
      void loadLibrary();
      return true;
    } catch (err) {
      pushToast(`Delete failed: ${err}`, "danger");
      return false;
    }
  };

  const deleteDocuments = async (docIDs: number[], label: string): Promise<boolean> => {
    try {
      const removed = await api.deleteDocuments(docIDs);
      pushToast(
        `Removed ${label} · ${removed} chunk${removed === 1 ? "" : "s"} deleted`,
        "success",
      );
      setSelectedDoc(null);
      void refresh();
      void loadLibrary();
      return true;
    } catch (err) {
      pushToast(`Delete failed: ${err}`, "danger");
      return false;
    }
  };

  const deleteCollection = async (name: string, label: string): Promise<boolean> => {
    try {
      const removed = await api.deleteCollection(name);
      pushToast(
        `Deleted ${label} · ${removed.toLocaleString()} chunk${removed === 1 ? "" : "s"} removed`,
        "success",
      );
      setExpandedCollection(null);
      setSelectedDoc(null);
      await loadConfig();
      discardSettingsDraft();
      void refresh();
      void loadLibrary();
      return true;
    } catch (err) {
      pushToast(`Delete failed: ${err}`, "danger");
      return false;
    }
  };

  const offProgress = Events.On("indexing:progress", (ev) =>
    setIndexProgress(ev.data as IndexProgress),
  );
  const offFile = Events.On("indexing:file", (ev) => {
    const e = ev.data as IndexFileProgress;
    setIndexByCollection((m) => ({ ...m, [e.collection]: e }));
  });
  const offComplete = Events.On("indexing:complete", (ev) => {
    const e = ev.data as IndexComplete;
    setIndexByCollection((m) => {
      const n = { ...m };
      delete n[e.collection];
      return n;
    });
    setIndexProgress(null);
    setIndexLast(e);
    if (!indexAllActive()) {
      setIndexing(false);
      void loadLibrary();
    }
    void refresh();
    if (e.errors > 0) pushToast(`${e.collection}: ${e.errors} error(s)`, "danger");
    else pushToast(`Indexed ${e.collection} · ${e.indexed} new`, "success");
  });
  const offAllDone = Events.On("indexing:all-done", () => {
    setIndexAllActive(false);
    setIndexing(false);
    void loadLibrary(); 
  });
  const offCancelled = Events.On("indexing:cancelled", (ev) => {
    const e = ev.data as IndexCancelled;
    setIndexByCollection((m) => {
      const n = { ...m };
      delete n[e.collection];
      return n;
    });
    setIndexProgress(null);
    setIndexing(false);
    setIndexAllActive(false); 
    void refresh();
    pushToast(`Indexing ${e.collection} cancelled · ${e.indexed} indexed`, "neutral");
  });
  const offFailed = Events.On("indexing:failed", (ev) => {
    const e = ev.data as IndexFailed;
    setIndexByCollection((m) => {
      const n = { ...m };
      if (e.collection) delete n[e.collection];
      return n;
    });
    setIndexProgress(null);
    setIndexing(false);
    setIndexAllActive(false);
    void refresh();
    const label = e.collection ? `${e.collection} indexing failed` : "Indexing failed";
    pushToast(`${label}: ${e.message}`, "danger");
  });
  const offPruned = Events.On("indexing:pruned", (ev) => {
    const n = ev.data as number;
    if (n > 0) pushToast(`Pruned ${n} stale sources`, "neutral");
  });
  const offModelChanged = Events.On("model:changed", () => {
    void refresh();
    void loadModels();
  });
  const offMCP = Events.On("mcp:status", (ev) => {
    setMCPStatus(ev.data as MCPStatus);
  });
  const offDlProgress = Events.On("model:download-progress", (ev) => {
    const d = ev.data as ModelDownloadProgress;
    setDownloadState({
      active: true, key: d.key, status: "downloading",
      downloaded: d.downloaded, total: d.total, percent: d.percent, speed: d.speed, error: "",
    });
  });
  const offDlComplete = Events.On("model:download-complete", (ev) => {
    const m = ev.data as ModelInfo;
    pushToast(`Model ready: ${m?.name ?? "Embedding model"}`, "success");
    void loadModels();
    void refresh();
    setDownloadState(null);
    setModelDialogOpen(false);
  });
  const offDlFailed = Events.On("model:download-failed", (ev) => {
    const e = ev.data as ModelDownloadError;
    pushToast(`Download failed: ${e.message}`, "danger");
    setDownloadState(null);
  });
  const offDlCancelled = Events.On("model:download-cancelled", () => {
    pushToast("Download cancelled", "neutral");
    setDownloadState(null);
  });

  const hydrateIndexing = async () => {
    try {
      const st = await api.getIndexingState();
      if (!st || typeof st !== "object" || typeof (st as { active?: unknown }).active !== "boolean") {
        return;
      }
      const s = st as IndexState;
      if (!s.active) return; 
      setIndexing(true);
      setIndexAllActive(Boolean(s.all));
      const map: Record<string, IndexFileProgress> = {};
      for (const [name, p] of Object.entries(s.collections ?? {})) map[name] = p;
      setIndexByCollection(map);
    } catch {
      /* backend not ready yet */
    }
  };

  onMount(() => {
    void refresh();
    void loadConfig();
    void loadRecommended();
    void hydrateIndexing();
    void hydrateDownload();
    void loadCPUCount();
    void refreshMCP();
    void (async () => {
      await loadModels();
      if (models().length === 0 && !modelDialogDismissed()) setModelDialogOpen(true);
    })();
  });
  onCleanup(() => {
    offProgress();
    offFile();
    offComplete();
    offCancelled();
    offFailed();
    offAllDone();
    offPruned();
    offModelChanged();
    offMCP();
    offDlProgress();
    offDlComplete();
    offDlFailed();
    offDlCancelled();
  });

  return {
    view,
    setView,
    openSettings,
    takePendingSettingsSection,
    canIndex,
    modelState,
    modelName,
    status,
    collections,
    config,
    models,
    recommended,
    downloadState,
    modelDialogOpen,
    closeModelDialog,
    downloadModelByKey,
    cancelDownload,
    importModelFile,
    uninstallCatalogFile,
    mcpStatus,
    refreshMCP,
    loadModels,
    setActiveModel,
    deleteModel,
    updateModelSettings,
    settingsDraft,
    settingsDirty,
    setSettingsDraft,
    replaceSettingsDraft,
    saveSettings,
    pendingLeave,
    cancelLeave,
    confirmLeave,
    cpuCount,
    loadConfig,
    sources,
    documents,
    loadSources,
    loadLibrary,
    query,
    setQuery,
    filters,
    setFilters,
    results,
    scoreDisplay,
    setScoreDisplay,
    searchState,
    runSearch,
    clearSearch,
    registerSearchInput,
    focusSearch,
    expandedCollection,
    setExpandedCollection,
    selectedDoc,
    setSelectedDoc,
    indexing,
    indexProgress,
    indexByCollection,
    indexLast,
    startIndex,
    startIndexAll,
    cancelIndex,
    runPrune,
    addSource,
    toggleCollection,
    deleteSource,
    deleteDocuments,
    deleteCollection,
    toasts,
    pushToast,
    dismissToast,
  };
}

export type AppStore = ReturnType<typeof createAppStore>;

const AppStoreCtx = createContext<AppStore>();

export function AppStoreProvider(props: { children: JSX.Element }) {
  const store = createAppStore();
  return (
    <AppStoreCtx.Provider value={store}>{props.children}</AppStoreCtx.Provider>
  );
}

export function useAppStore(): AppStore {
  const ctx = useContext(AppStoreCtx);
  if (!ctx) throw new Error("useAppStore must be used inside <AppStoreProvider>");
  return ctx;
}
