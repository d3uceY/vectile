import { createSignal, For } from "solid-js";
import { Button } from "../ui/primitives";
import { CloseIcon, FolderOpenIcon } from "../ui/icons";
import { pickFolder } from "../../lib/api";

export function PathList(props: {
  values: string[];
  onAdd: (v: string) => void;
  onRemove: (v: string) => void;
  title?: string;
  placeholder?: string;
  empty?: string;
}) {
  const [input, setInput] = createSignal("");

  const addInput = () => {
    if (input().trim()) {
      props.onAdd(input().trim());
      setInput("");
    }
  };

  const browse = async () => {
    const dir = await pickFolder(props.title);
    if (dir) props.onAdd(dir);
  };

  return (
    <div class="flex flex-col gap-2">
      {props.values.length === 0 ? (
        <p class="rounded-control border border-dashed border-line-strong px-3 py-2.5 text-[13px] leading-5 text-muted">
          {props.empty ?? "Nothing here yet. Add a path below."}
        </p>
      ) : (
        <ul class="divide-y divide-line overflow-hidden rounded-control border border-line bg-paper-warm pb-1.5">
          <For each={props.values}>
            {(v) => (
              <li class="group flex items-center gap-2 px-3 py-2">
                <span class="data min-w-0 flex-1 truncate font-mono text-[12.5px] text-muted" title={v}>
                  {v}
                </span>
                <button
                  class="shrink-0 text-faint opacity-0 transition-opacity duration-150 group-hover:opacity-100 focus-visible:opacity-100 hover:text-danger"
                  onClick={() => props.onRemove(v)}
                  aria-label={`Remove ${v}`}
                >
                  <CloseIcon size={14} />
                </button>
              </li>
            )}
          </For>
        </ul>
      )}
      <div class="flex gap-2">
        <input
          value={input()}
          onInput={(e) => setInput(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") addInput();
          }}
          placeholder={props.placeholder ?? "/absolute/path"}
          class="h-8 min-w-0 flex-1 rounded-control border border-line-control bg-surface px-3 text-[13px] transition-colors focus:border-leaf"
          spellcheck={false}
        />
        <Button size="sm" variant="outline" onClick={() => void browse()} aria-label="Browse for folder">
          <FolderOpenIcon size={15} />
          Browse
        </Button>
        <Button size="sm" onClick={addInput}>
          Add
        </Button>
      </div>
    </div>
  );
}

export function ChipList(props: {
  values: string[];
  onAdd: (v: string) => void;
  onRemove: (v: string) => void;
  empty?: string;
}) {
  const [input, setInput] = createSignal("");

  const addInput = () => {
    if (input().trim()) {
      props.onAdd(input().trim());
      setInput("");
    }
  };

  return (
    <div class="flex flex-col gap-2">
      {props.values.length === 0 ? (
        <p class="text-[12.5px] leading-4 text-muted">
          {props.empty ?? "None."}
        </p>
      ) : (
        <ul class="flex flex-wrap gap-1.5">
          <For each={props.values}>
            {(v) => (
              <li class="inline-flex max-w-full items-center gap-1.5 rounded-full border border-line-control bg-surface px-2.5 py-1">
                <span class="min-w-0 truncate font-mono text-[12px] text-ink-soft" title={v}>
                  {v}
                </span>
                <button
                  class="shrink-0 text-faint transition-colors hover:text-danger"
                  onClick={() => props.onRemove(v)}
                  aria-label={`Remove ${v}`}
                >
                  <CloseIcon size={12} />
                </button>
              </li>
            )}
          </For>
        </ul>
      )}
      <div class="flex items-center gap-2">
        <input
          value={input()}
          onInput={(e) => setInput(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") addInput();
          }}
          placeholder="name, e.g. .trash or CHANGELOG.md"
          class="h-8 min-w-0 flex-1 rounded-control border border-line-control bg-surface px-2.5 text-[12.5px] transition-colors focus:border-leaf"
          spellcheck={false}
        />
        <Button size="sm" onClick={addInput}>
          Add
        </Button>
      </div>
    </div>
  );
}

export function GroupItem(props: {
  name: string;
  paths: string[];
  onAddPath: (name: string, v: string) => void;
  onRemovePath: (name: string, v: string) => void;
  onRemoveGroup: (name: string) => void;
  title?: string;
}) {
  const [input, setInput] = createSignal("");

  const addInput = () => {
    if (input().trim()) {
      props.onAddPath(props.name, input().trim());
      setInput("");
    }
  };

  const browse = async () => {
    const dir = await pickFolder(props.title);
    if (dir) props.onAddPath(props.name, dir);
  };

  return (
    <li class="px-3 py-3">
      <div class="mb-2 flex items-center gap-2">
        <span class="min-w-0 flex-1 truncate text-[13px] font-semibold text-ink">{props.name}</span>
        <span class="data shrink-0 rounded-full border border-line bg-surface px-2 py-0.5 font-mono text-[11px] text-muted">
          {props.paths.length} {props.paths.length === 1 ? "path" : "paths"}
        </span>
        <button
          class="shrink-0 text-faint transition-colors hover:text-danger"
          onClick={() => props.onRemoveGroup(props.name)}
          aria-label={`Remove ${props.name}`}
        >
          <CloseIcon size={14} />
        </button>
      </div>
      {props.paths.length === 0 ? (
        <p class="text-[12.5px] leading-4 text-muted">No paths yet.</p>
      ) : (
        <ul class="divide-y divide-line">
          <For each={props.paths}>
            {(v) => (
              <li class="group flex items-center gap-2 py-1.5">
                <span class="data min-w-0 flex-1 truncate font-mono text-[12.5px] text-muted" title={v}>
                  {v}
                </span>
                <button
                  class="shrink-0 text-faint opacity-0 transition-opacity duration-150 group-hover:opacity-100 focus-visible:opacity-100 hover:text-danger"
                  onClick={() => props.onRemovePath(props.name, v)}
                  aria-label={`Remove ${v}`}
                >
                  <CloseIcon size={13} />
                </button>
              </li>
            )}
          </For>
        </ul>
      )}
      <div class="mt-2 flex items-center gap-2">
        <input
          value={input()}
          onInput={(e) => setInput(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") addInput();
          }}
          placeholder="/absolute/path"
          class="h-8 min-w-0 flex-1 rounded-control border border-line-control bg-surface px-2.5 text-[12.5px] transition-colors focus:border-leaf"
          spellcheck={false}
        />
        <Button size="sm" variant="outline" onClick={() => void browse()}>
          Browse
        </Button>
        <Button size="sm" onClick={addInput}>
          Add
        </Button>
      </div>
    </li>
  );
}

export function GroupList(props: {
  groups: Record<string, string[]>;
  onAddPath: (name: string, v: string) => void;
  onRemovePath: (name: string, v: string) => void;
  onAddGroup: (name: string) => void;
  onRemoveGroup: (name: string) => void;
  title?: string;
  empty?: string;
}) {
  const [name, setName] = createSignal("");

  const addGroup = () => {
    if (name().trim()) {
      props.onAddGroup(name().trim());
      setName("");
    }
  };

  const entries = () => Object.entries(props.groups);

  return (
    <div class="flex flex-col gap-2">
      {entries().length === 0 ? (
        <p class="rounded-control border border-dashed border-line-strong px-3 py-2.5 text-[13px] leading-5 text-muted">
          {props.empty ?? "No collections yet. Create one, then add its folders."}
        </p>
      ) : (
        <ul class="divide-y divide-line overflow-hidden rounded-control border border-line bg-paper-warm pb-1.5">
          <For each={entries()}>
            {([gname, paths]) => (
              <GroupItem
                name={gname}
                paths={paths}
                onAddPath={props.onAddPath}
                onRemovePath={props.onRemovePath}
                onRemoveGroup={props.onRemoveGroup}
                title={props.title}
              />
            )}
          </For>
        </ul>
      )}
      <div class="flex gap-2">
        <input
          value={name()}
          onInput={(e) => setName(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") addGroup();
          }}
          placeholder="collection name, e.g. client-work"
          class="h-8 min-w-0 flex-1 rounded-control border border-line-control bg-surface px-3 text-[13px] transition-colors focus:border-leaf"
        />
        <Button size="sm" onClick={addGroup}>
          New collection
        </Button>
      </div>
    </div>
  );
}
