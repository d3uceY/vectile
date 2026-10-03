import { Show } from "solid-js";
import { Toggle } from "../../ui/primitives";
import { IndexIcon } from "../../ui/icons";
import { STATIC_BOUNDS } from "../bounds";
import { FieldList, NumField, Section } from "../fields";
import { useSettings } from "../context";

export function IndexingSection() {
  const { draft, setNumber, setGui } = useSettings();

  return (
    <Section
      icon={<IndexIcon size={16} />}
      title="Indexing"
      note="How far back git history reaches, and whether the library refreshes itself."
    >
      <FieldList>
        <NumField
          label="Commit history (months)"
          value={draft()!.git_history_in_months}
          onChange={(n) => setNumber("git_history_in_months", n)}
          hint="How many months of git history get indexed for a repository. Each commit becomes a searchable document, so this controls how far back you can dig through your changelog. 6 months is the default."
          min={STATIC_BOUNDS.git_history_in_months.min}
          max={STATIC_BOUNDS.git_history_in_months.max}
          step={STATIC_BOUNDS.git_history_in_months.step}
        />
        <Toggle
          checked={draft()!.gui.auto_reindex}
          onChange={(v) => setGui({ auto_reindex: v })}
          label="Auto-reindex"
          description="Re-index all enabled collections on a timer."
          hint="Re-index all your collections on a timer, so files you add or edit show up in search without running anything manually. Off by default: a full pass uses your CPU and model for a while. Turn it on if you add files often."
        />
        <Show when={draft()!.gui.auto_reindex}>
          <NumField
            label="Interval (minutes)"
            value={draft()!.gui.auto_reindex_interval_minutes}
            onChange={(n) => setGui({ auto_reindex_interval_minutes: n })}
            hint="How often the auto-reindex timer fires. 60 means once an hour. Only matters when Auto-reindex is switched on."
            min={STATIC_BOUNDS.auto_reindex_interval_minutes.min}
            max={STATIC_BOUNDS.auto_reindex_interval_minutes.max}
            step={STATIC_BOUNDS.auto_reindex_interval_minutes.step}
          />
        </Show>
        <Toggle
          checked={draft()!.gui.start_on_login}
          onChange={(v) => setGui({ start_on_login: v })}
          label="Start on login"
          description="Launch vectile when you sign in."
          hint="Launch vectile when you sign in, so it's already open and indexing before you need it."
        />
      </FieldList>
    </Section>
  );
}
