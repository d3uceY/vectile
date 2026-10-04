import type { JSX } from "solid-js";
import {
  CacheNavIcon,
  ChunkNavIcon,
  ConnectNavIcon,
  IndexNavIcon,
  ModelNavIcon,
  OcrNavIcon,
  SearchNavIcon,
  SourcesNavIcon,
} from "../ui/nav-icons";
import { MASCOT_STATIC } from "../shell/mascot/assets";
import type { NavIconSlot, SectionKey } from "./types";

export function VexterNavIcon(_p: NavIconSlot) {
  return (
    <span class="flex h-4 w-4 items-center justify-center overflow-hidden">
      <img src={MASCOT_STATIC} alt="" class="h-full w-full object-contain" style="image-rendering: pixelated" />
    </span>
  );
}

export const NAV_GROUPS: {
  label: string;
  items: { key: SectionKey; label: string; icon: (p: NavIconSlot) => JSX.Element }[];
}[] = [
  {
    label: "Engine",
    items: [
      { key: "model", label: "Model", icon: ModelNavIcon },
      { key: "ocr", label: "OCR", icon: OcrNavIcon },
      { key: "chunking", label: "Chunking", icon: ChunkNavIcon },
      { key: "search", label: "Search", icon: SearchNavIcon },
      { key: "cache", label: "Cache", icon: CacheNavIcon },
    ],
  },
  {
    label: "Library",
    items: [
      { key: "sources", label: "Sources", icon: SourcesNavIcon },
      { key: "indexing", label: "Indexing", icon: IndexNavIcon },
    ],
  },
  {
    label: "Companions",
    items: [
      { key: "vexter", label: "Vexter", icon: VexterNavIcon },
      { key: "connect", label: "Connect", icon: ConnectNavIcon },
    ],
  },
];
