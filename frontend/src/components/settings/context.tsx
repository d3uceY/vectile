import { createContext, useContext, type JSX } from "solid-js";
import type {
  AppConfig,
  GUIConfig,
  MascotConfig,
  MCPConfig,
  OCRConfig,
  SearchDefaults,
} from "../../lib/types";
import { useAppStore } from "../../lib/store";
import { boundsFor, clamp, STATIC_BOUNDS } from "./bounds";
import type { PathKey } from "./config";

type NumberKey =
  | "embedding_batch_size"
  | "chunk_size_tokens"
  | "chunk_overlap_tokens"
  | "git_history_in_months";

type GroupKey = "projects" | "repositories";

export type SettingsEditor = {
  draft: () => AppConfig | null;
  setNumber: (k: NumberKey, n: number) => void;
  setSearch: (k: keyof SearchDefaults, n: number) => void;
  setGui: (p: Partial<GUIConfig>) => void;
  setMCP: (p: Partial<MCPConfig>) => void;
  setOCR: (p: Partial<OCRConfig>) => void;
  setMascot: (k: keyof MascotConfig, v: boolean) => void;
  setMascotAll: (v: boolean) => void;
  mascotAllDisabled: () => boolean;
  addPath: (k: PathKey, v: string) => void;
  removePath: (k: PathKey, v: string) => void;
  addGroupPath: (mapKey: GroupKey, name: string, v: string) => void;
  removeGroupPath: (mapKey: GroupKey, name: string, v: string) => void;
  addGroup: (mapKey: GroupKey, name: string) => void;
  removeGroup: (mapKey: GroupKey, name: string) => void;
};

const SettingsContext = createContext<SettingsEditor>();

export function useSettings(): SettingsEditor {
  const ctx = useContext(SettingsContext);
  if (!ctx) throw new Error("useSettings must be used within <SettingsProvider>");
  return ctx;
}

/**
 * Builds the draft-editing surface every Settings section shares. Lives above
 * the sections so each one only depends on the pieces it actually uses.
 */
function createSettingsEditor(): SettingsEditor {
  const store = useAppStore();
  const draft = () => store.settingsDraft();

  const setNumber: SettingsEditor["setNumber"] = (k, n) =>
    store.setSettingsDraft((d) => {
      if (!d) return d;
      if (k === "chunk_size_tokens") {
        const size = clamp(n, STATIC_BOUNDS.chunk_size_tokens.min, STATIC_BOUNDS.chunk_size_tokens.max);
        const overlap = clamp(
          d.chunk_overlap_tokens,
          STATIC_BOUNDS.chunk_overlap_tokens.min,
          Math.max(STATIC_BOUNDS.chunk_overlap_tokens.min, size - 1),
        );
        return { ...d, chunk_size_tokens: size, chunk_overlap_tokens: overlap };
      }
      const b = boundsFor(k, d.chunk_size_tokens);
      return { ...d, [k]: clamp(n, b.min, b.max) };
    });

  const setSearch: SettingsEditor["setSearch"] = (k, n) =>
    store.setSettingsDraft((d) => {
      if (!d) return d;
      const b = STATIC_BOUNDS[k];
      return { ...d, search_defaults: { ...d.search_defaults, [k]: clamp(n, b.min, b.max) } };
    });

  const setGui: SettingsEditor["setGui"] = (p) =>
    store.setSettingsDraft((d) => {
      if (!d) return d;
      const gui = { ...d.gui, ...p };
      if (p.auto_reindex_interval_minutes !== undefined) {
        const b = STATIC_BOUNDS.auto_reindex_interval_minutes;
        gui.auto_reindex_interval_minutes = clamp(p.auto_reindex_interval_minutes, b.min, b.max);
      }
      return { ...d, gui };
    });

  const setMCP: SettingsEditor["setMCP"] = (p) =>
    store.setSettingsDraft((d) => {
      if (!d) return d;
      const mcp = { ...d.mcp, ...p };
      if (p.port !== undefined) {
        mcp.port = clamp(p.port, STATIC_BOUNDS.mcp_port.min, STATIC_BOUNDS.mcp_port.max);
      }
      return { ...d, mcp };
    });

  const setOCR: SettingsEditor["setOCR"] = (p) =>
    store.setSettingsDraft((d) => (d ? { ...d, ocr: { ...d.ocr, ...p } } : d));

  const setMascot: SettingsEditor["setMascot"] = (k, v) =>
    store.setSettingsDraft((d) => {
      if (!d) return d;
      return { ...d, gui: { ...d.gui, mascot: { ...d.gui.mascot, [k]: v } } };
    });

  const setMascotAll: SettingsEditor["setMascotAll"] = (v) =>
    store.setSettingsDraft((d) => {
      if (!d) return d;
      return {
        ...d,
        gui: { ...d.gui, mascot: { show_searching: !v, show_indexing: !v, show_nothing: !v } },
      };
    });

  const mascotAllDisabled = () => {
    const m = draft()?.gui.mascot;
    return m ? !m.show_searching && !m.show_indexing && !m.show_nothing : false;
  };

  const addPath: SettingsEditor["addPath"] = (k, v) =>
    store.setSettingsDraft((d) => (d ? { ...d, [k]: [...d[k], v] } : d));

  const removePath: SettingsEditor["removePath"] = (k, v) =>
    store.setSettingsDraft((d) => (d ? { ...d, [k]: d[k].filter((x) => x !== v) } : d));

  const addGroupPath: SettingsEditor["addGroupPath"] = (mapKey, name, v) =>
    store.setSettingsDraft((d) =>
      d ? { ...d, [mapKey]: { ...d[mapKey], [name]: [...(d[mapKey][name] ?? []), v] } } : d,
    );

  const removeGroupPath: SettingsEditor["removeGroupPath"] = (mapKey, name, v) =>
    store.setSettingsDraft((d) =>
      d
        ? { ...d, [mapKey]: { ...d[mapKey], [name]: (d[mapKey][name] ?? []).filter((x) => x !== v) } }
        : d,
    );

  const addGroup: SettingsEditor["addGroup"] = (mapKey, name) =>
    store.setSettingsDraft((d) => (d ? { ...d, [mapKey]: { ...d[mapKey], [name]: [] } } : d));

  const removeGroup: SettingsEditor["removeGroup"] = (mapKey, name) =>
    store.setSettingsDraft((d) => {
      if (!d) return d;
      const m = { ...d[mapKey] };
      delete m[name];
      return { ...d, [mapKey]: m };
    });

  return {
    draft,
    setNumber,
    setSearch,
    setGui,
    setMCP,
    setOCR,
    setMascot,
    setMascotAll,
    mascotAllDisabled,
    addPath,
    removePath,
    addGroupPath,
    removeGroupPath,
    addGroup,
    removeGroup,
  };
}

export function SettingsProvider(props: { children: JSX.Element }) {
  const editor = createSettingsEditor();
  return <SettingsContext.Provider value={editor}>{props.children}</SettingsContext.Provider>;
}
