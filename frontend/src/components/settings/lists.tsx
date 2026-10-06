import { createEffect, createSignal, createUniqueId, For, Show, type JSX } from "solid-js";
import { Button, ConfirmDialog } from "../ui/primitives";
import { ChevronDown, CloseIcon, FolderOpenIcon, PlusIcon } from "../ui/icons";
import { pickFolder } from "../../lib/api";

/* One row of a source list: a chevron that unfolds what it holds, the name, and
   how much is inside. */
function AccordionToggle(props: {
  label: string;
  count: string;
  open: boolean;
  controls: string;
  onToggle: () => void;
}) {
  return (
    <button
      type="button"
      class="flex min-w-0 flex-1 items-center gap-2 rounded-md px-1.5 py-1.5 text-left transition-colors duration-100 ease-snappy hover:bg-surface-2"
      onClick={props.onToggle}
      aria-expanded={props.open}
      aria-controls={props.controls}
    >
      <ChevronDown
        size={14}
        class={`shrink-0 text-faint transition-transform duration-150 ease-snappy ${
          props.open ? "rotate-0" : "-rotate-90"
        }`}
      />
      <span class="min-w-0 truncate text-[13px] font-semibold text-ink">{props.label}</span>
      <span class="shrink-0 rounded-full border border-line bg-surface-2 px-2 py-0.5 font-mono text-[11px] tabular-nums text-muted">
        {props.count}
      </span>
    </button>
  );
}

/* The accordion body: 0fr -> 1fr, the app's one documented height animation. */
function AccordionBody(props: { id: string; open: boolean; children: JSX.Element }) {
  return (
    <div
      id={props.id}
      class={`accordion-collapsible ${props.open ? "open" : ""}`}
      inert={props.open ? undefined : true}
    >
      <div class="accordion-collapsible-inner">{props.children}</div>
    </div>
  );
}

/* The paths inside a row, each removable on hover. Rows rise in as they arrive,
   so an added path is impossible to miss. */
function PathRows(props: { values: string[]; onRemove: (v: string) => void; empty: string }) {
  return (
    <div class="pb-1.5 pl-6 pr-1 pt-1.5">
      {props.values.length === 0 ? (
        <p class="py-1 font-mono text-[12px] text-muted">{props.empty}</p>
      ) : (
        <ul class="divide-y divide-line overflow-hidden rounded-[8px] border border-line bg-paper">
          <For each={props.values}>
            {(v) => (
              <li class="od-fade-slide-up group/row flex items-center gap-2 px-3 py-1.5">
                <span class="min-w-0 flex-1 truncate font-mono text-[12px] text-ink-soft" title={v}>
                  {v}
                </span>
                <button
                  class="shrink-0 text-faint opacity-0 transition-opacity duration-150 group-hover/row:opacity-100 focus-visible:opacity-100 hover:text-danger"
                  onClick={() => props.onRemove(v)}
                  aria-label={`Remove ${v}`}
                >
                  <CloseIcon size={13} />
                </button>
              </li>
            )}
          </For>
        </ul>
      )}
    </div>
  );
}

/** A flat path list (every vault lands in one collection), folded into the same
    accordion as the collections so a machine with a dozen of them leaves the
    section the same height. */
export function PathList(props: {
  values: string[];
  onAdd: (v: string) => void;
  onRemove: (v: string) => void;
  label: string;
  noun: string;
  nounPlural: string;
  dialogTitle: string;
  pathLabel: string;
  pickerTitle?: string;
  empty?: string;
}) {
  const [open, setOpen] = createSignal(false);
  const [adding, setAdding] = createSignal(false);
  const bodyId = createUniqueId();
  const count = () => props.values.length;
  const countLabel = () => `${count()} ${count() === 1 ? props.noun : props.nounPlural}`;

  return (
    <div class="overflow-hidden rounded-[8px] border border-line bg-surface">
      <div class="flex items-center gap-1.5 px-3 py-2">
        <AccordionToggle
          label={props.label}
          count={countLabel()}
          open={open()}
          controls={bodyId}
          onToggle={() => setOpen(!open())}
        />
        <Button size="sm" variant="ghost" onClick={() => setAdding(true)}>
          <PlusIcon size={14} />
          Add {props.noun}
        </Button>
      </div>
      <AccordionBody id={bodyId} open={open()}>
        <PathRows
          values={props.values}
          onRemove={props.onRemove}
          empty={props.empty ?? "Nothing here yet."}
        />
      </AccordionBody>
      <AddPathDialog
        title={adding() ? props.dialogTitle : null}
        label={props.pathLabel}
        pickerTitle={props.pickerTitle}
        onAdd={props.onAdd}
        onClose={() => setAdding(false)}
      />
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
        <p class="text-[12px] font-medium leading-4 text-muted">
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
          class="h-9 min-w-0 flex-1 rounded-control border border-line-control bg-paper px-2.5 text-[13px] placeholder:text-muted transition-colors duration-100 focus:border-ink"
          spellcheck={false}
        />
        <Button size="sm" onClick={addInput}>
          Add
        </Button>
      </div>
    </div>
  );
}

/* Adding a path is its own protected step: the title names the collection it
   lands in, and Browse fills the field instead of adding, so a path is confirmed
   once and cannot land in the list twice. */
function AddPathDialog(props: {
  title: string | null;
  label: string;
  pickerTitle?: string;
  onAdd: (v: string) => void;
  onClose: () => void;
}) {
  const [input, setInput] = createSignal("");
  const fieldId = createUniqueId();
  let field: HTMLInputElement | undefined;

  createEffect(() => {
    if (props.title) queueMicrotask(() => field?.focus());
  });

  const close = () => {
    setInput("");
    props.onClose();
  };

  const submit = () => {
    const v = input().trim();
    if (!v) return;
    props.onAdd(v);
    close();
  };

  const browse = async () => {
    const dir = await pickFolder(props.pickerTitle);
    if (dir) setInput(dir);
  };

  return (
    <ConfirmDialog
      open={props.title !== null}
      title={props.title ?? ""}
      confirmLabel="Add"
      confirmIcon={<PlusIcon size={14} />}
      confirmTone="primary"
      confirmDisabled={input().trim() === ""}
      cancelLabel="Cancel"
      onCancel={close}
      onConfirm={submit}
      body={
        <div>
          <label for={fieldId} class="block text-[12px] font-semibold text-ink-soft">
            {props.label}
          </label>
          <div class="mt-1.5 flex gap-2">
            <input
              id={fieldId}
              ref={(el) => (field = el)}
              value={input()}
              onInput={(e) => setInput(e.currentTarget.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") submit();
              }}
              placeholder="/absolute/path"
              spellcheck={false}
              class="h-9 min-w-0 flex-1 rounded-control border border-line-control bg-paper px-3 text-[13px] text-ink placeholder:text-muted transition-colors duration-100 focus:border-ink"
            />
            <Button size="sm" variant="outline" onClick={() => void browse()} aria-label="Browse for folder">
              <FolderOpenIcon size={15} />
              Browse
            </Button>
          </div>
          <p class="mt-2 text-[12px] font-medium leading-4">
            Browse fills the path in; nothing is added until you confirm.
          </p>
        </div>
      }
    />
  );
}

export function GroupItem(props: {
  name: string;
  paths: string[];
  open: boolean;
  onToggle: () => void;
  onAddPath: (name: string, v: string) => void;
  onRemovePath: (name: string, v: string) => void;
  onRemoveGroup: (name: string) => void;
  title?: string;
}) {
  const [adding, setAdding] = createSignal(false);
  const bodyId = createUniqueId();

  return (
    <li class="px-3 py-1.5">
      <div class="flex items-center gap-1.5">
        <AccordionToggle
          label={props.name}
          count={`${props.paths.length} ${props.paths.length === 1 ? "folder" : "folders"}`}
          open={props.open}
          controls={bodyId}
          onToggle={props.onToggle}
        />
        <Button size="sm" variant="ghost" onClick={() => setAdding(true)}>
          <PlusIcon size={14} />
          Add folder
        </Button>
        <button
          class="shrink-0 px-1 text-faint transition-colors hover:text-danger"
          onClick={() => props.onRemoveGroup(props.name)}
          aria-label={`Remove ${props.name}`}
        >
          <CloseIcon size={14} />
        </button>
      </div>

      <AccordionBody id={bodyId} open={props.open}>
        <PathRows
          values={props.paths}
          onRemove={(v) => props.onRemovePath(props.name, v)}
          empty="No folders yet."
        />
      </AccordionBody>

      <AddPathDialog
        title={adding() ? props.name : null}
        label="Folder path"
        pickerTitle={props.title}
        onAdd={(v) => props.onAddPath(props.name, v)}
        onClose={() => setAdding(false)}
      />
    </li>
  );
}

/** Collections land in a headed list, so "New collection" keeps one fixed spot
    whether the list is empty, one row tall, or forty rows of folders deep. */
export function GroupList(props: {
  groups: Record<string, string[]>;
  onAddPath: (name: string, v: string) => void;
  onRemovePath: (name: string, v: string) => void;
  onAddGroup: (name: string) => void;
  onRemoveGroup: (name: string) => void;
  title?: string;
  empty?: string;
}) {
  const [creating, setCreating] = createSignal(false);
  const [name, setName] = createSignal("");
  const [openName, setOpenName] = createSignal<string | null>(null);
  const nameFieldId = createUniqueId();
  let nameField: HTMLInputElement | undefined;

  const entries = () => Object.entries(props.groups);
  const count = () => entries().length;
  const duplicate = () => name().trim() !== "" && props.groups[name().trim()] !== undefined;

  createEffect(() => {
    if (creating()) queueMicrotask(() => nameField?.focus());
  });

  const closeCreate = () => {
    setCreating(false);
    setName("");
  };

  const create = () => {
    const n = name().trim();
    if (!n || duplicate()) return;
    props.onAddGroup(n);
    closeCreate();
    setOpenName(n); // open it, so adding its folders is the obvious next move
  };

  return (
    <div class="flex flex-col gap-2">
      <Show
        when={count() > 0}
        fallback={
          <div class="rounded-[8px] border border-dashed border-line-strong px-3 py-3">
            <p class="text-[13px] font-medium leading-5 text-muted">
              {props.empty ?? "No collections yet. Create one, then add its folders."}
            </p>
            <Button size="sm" class="mt-2.5" onClick={() => setCreating(true)}>
              <PlusIcon size={14} />
              New collection
            </Button>
          </div>
        }
      >
        <div class="overflow-hidden rounded-[8px] border border-line bg-surface">
          <div class="flex items-center gap-2 border-b border-line bg-surface-2/60 px-3 py-2">
            <span class="text-[11px] font-semibold uppercase tracking-[0.06em] text-muted">
              {count()} {count() === 1 ? "collection" : "collections"}
            </span>
            <Button size="sm" class="ml-auto" onClick={() => setCreating(true)}>
              <PlusIcon size={14} />
              New collection
            </Button>
          </div>
          <ul class="divide-y divide-line">
            <For each={entries()}>
              {([gname, paths]) => (
                <GroupItem
                  name={gname}
                  paths={paths}
                  open={openName() === gname}
                  onToggle={() => setOpenName(openName() === gname ? null : gname)}
                  onAddPath={props.onAddPath}
                  onRemovePath={props.onRemovePath}
                  onRemoveGroup={props.onRemoveGroup}
                  title={props.title}
                />
              )}
            </For>
          </ul>
        </div>
      </Show>

      <ConfirmDialog
        open={creating()}
        title="New collection"
        confirmLabel="Create"
        confirmIcon={<PlusIcon size={14} />}
        confirmTone="primary"
        confirmDisabled={name().trim() === "" || duplicate()}
        cancelLabel="Cancel"
        onCancel={closeCreate}
        onConfirm={create}
        body={
          <div>
            <label for={nameFieldId} class="block text-[12px] font-semibold text-ink-soft">
              Collection name
            </label>
            <input
              id={nameFieldId}
              ref={(el) => (nameField = el)}
              value={name()}
              onInput={(e) => setName(e.currentTarget.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") create();
              }}
              placeholder="client-work"
              class="mt-1.5 h-9 w-full rounded-control border border-line-control bg-paper px-3 text-[13px] text-ink placeholder:text-muted transition-colors duration-100 focus:border-ink"
            />
            <Show
              when={duplicate()}
              fallback={
                <p class="mt-2 text-[12px] font-medium leading-4">Then add its folders.</p>
              }
            >
              <p class="mt-2 text-[12px] font-semibold leading-4 text-danger">
                That name is already in use.
              </p>
            </Show>
          </div>
        }
      />
    </div>
  );
}
