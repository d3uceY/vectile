import { createSignal, For, Show, type JSX } from "solid-js";
import type { AppConfig } from "../../../lib/types";
import { InfoTip } from "../../ui/primitives";
import { CodeIcon, FileIcon, FolderOpenIcon, LibraryIcon, SlashIcon } from "../../ui/icons";
import { SourcesNavIcon } from "../../ui/nav-icons";
import { ChipList, GroupList, PathList } from "../lists";
import { Section } from "../fields";
import { useSettings } from "../context";

/* One entry per type, because each is a different machine underneath: a vault
   walks markdown, a repository walks git history, a project group is a named
   bag of folders. Each panel states what its paths become, since that name is
   what shows up later in Index, Library, Browse, and Search. */

type SourceKindId = "projects" | "repositories" | "obsidian" | "calibre";

type SourceKind = {
  id: SourceKindId;
  label: string;
  icon: (p: { size?: number }) => JSX.Element;
  blurb: string;
  count: (c: AppConfig) => number;
};

const SOURCE_KINDS: SourceKind[] = [
  {
    id: "projects",
    label: "Project folders",
    icon: FolderOpenIcon,
    blurb: "Folders you work in. Every file type vectile reads is parsed, subfolders included.",
    count: (c) => Object.keys(c.projects).length,
  },
  {
    id: "repositories",
    label: "Code repositories",
    icon: CodeIcon,
    blurb: "Git repositories: the current file tree, plus commit history. Nested repos included.",
    count: (c) => Object.keys(c.repositories).length,
  },
  {
    id: "obsidian",
    label: "Obsidian vaults",
    icon: FileIcon,
    blurb: "Markdown notes, subfolders included.",
    count: (c) => c.obsidian_vaults.length,
  },
  {
    id: "calibre",
    label: "Calibre libraries",
    icon: LibraryIcon,
    blurb: "Book text and metadata, read straight from the library folder.",
    count: (c) => c.calibre_libraries.length,
  },
];

/** Type picker. Arrow keys, Home, and End move between tabs, per the ARIA tabs pattern. */
function SourceTabs(props: {
  value: SourceKindId;
  counts: Record<SourceKindId, number>;
  onChange: (id: SourceKindId) => void;
}) {
  let els: HTMLButtonElement[] = [];
  const onKey = (e: KeyboardEvent, i: number) => {
    const n = SOURCE_KINDS.length;
    let next = -1;
    if (e.key === "ArrowRight") next = (i + 1) % n;
    else if (e.key === "ArrowLeft") next = (i - 1 + n) % n;
    else if (e.key === "Home") next = 0;
    else if (e.key === "End") next = n - 1;
    if (next < 0) return;
    e.preventDefault();
    props.onChange(SOURCE_KINDS[next].id);
    els[next]?.focus();
  };
  return (
    <div
      role="tablist"
      aria-label="Source type"
      class="grid grid-cols-1 gap-1 rounded-control border border-line-control bg-surface p-1 min-[430px]:grid-cols-2"
    >
      <For each={SOURCE_KINDS}>
        {(k, i) => {
          const active = () => props.value === k.id;
          return (
            <button
              ref={(el) => (els[i()] = el)}
              type="button"
              role="tab"
              id={`source-tab-${k.id}`}
              aria-selected={active()}
              aria-controls="source-panel"
              tabindex={active() ? 0 : -1}
              onClick={() => props.onChange(k.id)}
              onKeyDown={(e) => onKey(e, i())}
              class={`flex min-w-0 items-center gap-2 rounded-[8px] px-2.5 py-2 text-[13px] font-semibold transition-colors duration-100 ease-snappy ${
                active() ? "bg-mint text-leaf-deep" : "text-muted hover:bg-surface-2 hover:text-ink"
              }`}
            >
              <k.icon size={15} />
              <span class="min-w-0 flex-1 truncate text-left">{k.label}</span>
              <span
                class={`shrink-0 rounded-full px-1.5 py-0.5 font-mono text-[11px] tabular-nums ${
                  active() ? "bg-paper text-leaf-deep" : "bg-surface-2 text-muted"
                }`}
              >
                {props.counts[k.id]}
              </span>
            </button>
          );
        }}
      </For>
    </div>
  );
}

/** Folder and file names kept out of the index, at any depth, under the type
    they belong to. */
function ExcludeBlock(props: {
  values: string[];
  onAdd: (v: string) => void;
  onRemove: (v: string) => void;
  hint: string;
  note: string;
  empty: string;
}) {
  return (
    <div class="mt-5 border-t border-line pt-4">
      <div class="flex items-center gap-2">
        <SlashIcon size={14} class="shrink-0 text-muted" />
        <h4 class="text-[13px] font-semibold text-ink">Skip folders or files</h4>
        <InfoTip text={props.hint} />
        <span class="ml-auto shrink-0 rounded-full border border-line bg-surface-2 px-2 py-0.5 font-mono text-[11px] text-muted tabular-nums">
          {props.values.length}
        </span>
      </div>
      <p class="mt-1.5 text-[12px] font-medium leading-4 text-muted">{props.note}</p>
      <div class="mt-2.5">
        <ChipList
          values={props.values}
          onAdd={props.onAdd}
          onRemove={props.onRemove}
          empty={props.empty}
        />
      </div>
    </div>
  );
}

export function SourcesSection() {
  const { draft, addPath, removePath, addGroupPath, removeGroupPath, addGroup, removeGroup } =
    useSettings();

  const [sourceKind, setSourceKind] = createSignal<SourceKindId>("projects");
  const activeKind = (): SourceKind =>
    SOURCE_KINDS.find((k) => k.id === sourceKind()) ?? SOURCE_KINDS[0];
  const sourceCounts = (): Record<SourceKindId, number> => ({
    projects: SOURCE_KINDS[0].count(draft()!),
    repositories: SOURCE_KINDS[1].count(draft()!),
    obsidian: SOURCE_KINDS[2].count(draft()!),
    calibre: SOURCE_KINDS[3].count(draft()!),
  });

  return (
    <Section
      icon={<SourcesNavIcon size={16} />}
      title="Sources"
      note="Choose your sources for indexing."
    >
      <SourceTabs value={sourceKind()} counts={sourceCounts()} onChange={setSourceKind} />

      <div
        id="source-panel"
        role="tabpanel"
        aria-labelledby={`source-tab-${sourceKind()}`}
        class="mt-5"
      >
        <div
          id={sourceKind() === "projects" ? "setup-add-folder" : undefined}
          class="flex items-start gap-2.5"
        >
          <span class="mt-0.75 shrink-0 text-muted">
            {activeKind().icon({ size: 18 })}
          </span>
          <div class="min-w-0">
            <h3 class="text-[15px] font-semibold tracking-[-0.01em] text-ink">
              {activeKind().label}
            </h3>
            <p class="mt-1 max-w-[64ch] text-[13px] font-medium leading-5 text-muted">
              {activeKind().blurb}
            </p>
          </div>
        </div>

        <Show when={sourceKind() === "projects"}>
          <p class="mt-2.5 max-w-[64ch] text-[12px] font-medium leading-5 text-muted">
            Each group becomes one collection, named after the group.
          </p>
          <div class="mt-3.5">
            <GroupList
              groups={draft()!.projects}
              onAddPath={(n, v) => addGroupPath("projects", n, v)}
              onRemovePath={(n, v) => removeGroupPath("projects", n, v)}
              onAddGroup={(n) => addGroup("projects", n)}
              onRemoveGroup={(n) => removeGroup("projects", n)}
              title="Choose a project folder"
              empty="No collections yet. Create one, then add its folders."
            />
          </div>
          <ExcludeBlock
            values={draft()!.project_exclude_folders}
            onAdd={(v) => addPath("project_exclude_folders", v)}
            onRemove={(v) => removePath("project_exclude_folders", v)}
            hint="Names skipped anywhere inside a project folder, at any depth. node_modules is always skipped; add any other folder or file you never want in search results."
            note="Names skipped at any depth, inside every project collection."
            empty="None. Only node_modules is skipped."
          />
        </Show>

        <Show when={sourceKind() === "repositories"}>
          <p class="mt-2.5 max-w-[64ch] text-[12px] font-medium leading-5 text-muted">
            Each group becomes one collection. How far back commit history reaches is
            set under Indexing.
          </p>
          <div class="mt-3.5">
            <GroupList
              groups={draft()!.repositories}
              onAddPath={(n, v) => addGroupPath("repositories", n, v)}
              onRemovePath={(n, v) => removeGroupPath("repositories", n, v)}
              onAddGroup={(n) => addGroup("repositories", n)}
              onRemoveGroup={(n) => removeGroup("repositories", n)}
              title="Choose a code repository"
              empty="No collections yet. Create one, then add its repositories."
            />
          </div>
          <ExcludeBlock
            values={draft()!.repository_exclude_folders}
            onAdd={(v) => addPath("repository_exclude_folders", v)}
            onRemove={(v) => removePath("repository_exclude_folders", v)}
            hint="Names skipped anywhere inside a repository, at any depth. Build output and lock files are already skipped. This filters the file tree only; commit history is still indexed in full."
            note="Names skipped at any depth, inside every repository."
            empty="None. Only the built-in list is skipped."
          />
        </Show>

        <Show when={sourceKind() === "obsidian"}>
          <p class="mt-2.5 max-w-[64ch] text-[12px] font-medium leading-5 text-muted">
            Every vault lands in one collection called{" "}
            <span class="font-mono text-[12px] text-ink-soft">obsidian</span>.
          </p>
          <div class="mt-3.5">
            <PathList
              values={draft()!.obsidian_vaults}
              onAdd={(v) => addPath("obsidian_vaults", v)}
              onRemove={(v) => removePath("obsidian_vaults", v)}
              title="Choose an Obsidian vault"
              placeholder="path to a vault…"
              empty="No vaults yet. Add one and its notes become searchable."
            />
          </div>
          <ExcludeBlock
            values={draft()!.obsidian_exclude_folders}
            onAdd={(v) => addPath("obsidian_exclude_folders", v)}
            onRemove={(v) => removePath("obsidian_exclude_folders", v)}
            hint="Names listed here are skipped when vaults are indexed. Handy for hiding attachments, templates, .trash, or a stray file you don't want in search results."
            note="Names skipped at any depth, inside every vault."
            empty="None. Every folder inside a vault is indexed."
          />
        </Show>

        <Show when={sourceKind() === "calibre"}>
          <p class="mt-2.5 max-w-[64ch] text-[12px] font-medium leading-5 text-muted">
            Every library lands in one collection called{" "}
            <span class="font-mono text-[12px] text-ink-soft">calibre</span>.
          </p>
          <div class="mt-3.5">
            <PathList
              values={draft()!.calibre_libraries}
              onAdd={(v) => addPath("calibre_libraries", v)}
              onRemove={(v) => removePath("calibre_libraries", v)}
              title="Choose a Calibre library"
              placeholder="path to a library…"
              empty="No libraries yet. Add a Calibre library to search its books."
            />
          </div>
          <ExcludeBlock
            values={draft()!.calibre_exclude_folders}
            onAdd={(v) => addPath("calibre_exclude_folders", v)}
            onRemove={(v) => removePath("calibre_exclude_folders", v)}
            hint="Names listed here are skipped when a library is read. A book's folder is its author and title, so one entry can drop a whole author or a single book folder."
            note="Names skipped at any depth, inside every library."
            empty="None. Every book in the library is indexed."
          />
        </Show>
      </div>
    </Section>
  );
}
