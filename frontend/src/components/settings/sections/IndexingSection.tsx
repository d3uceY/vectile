import { Show } from "solid-js";
import { Toggle } from "../../ui/primitives";
import { IndexNavIcon } from "../../ui/nav-icons";
import { STATIC_BOUNDS } from "../bounds";
import { FieldList, NumField, Section } from "../fields";
import { useSettings } from "../context";

export function IndexingSection() {
  const { draft, setNumber, setGui } = useSettings();

  return (
    <Section
      icon={<IndexNavIcon size={16} />}
      title="Indexing"
      note="How far back git history reaches, and whether the library refreshes itself."
    >
      <FieldList>
        <NumField
          label="Commit history (months)"
          value={draft()!.git_history_in_months}
          onChange={(n) => setNumber("git_history_in_months", n)}
          hint="Months of git history to index for a repository. Each commit becomes a searchable document."
          min={STATIC_BOUNDS.git_history_in_months.min}
          max={STATIC_BOUNDS.git_history_in_months.max}
          step={STATIC_BOUNDS.git_history_in_months.step}
        />
        <Toggle
          checked={draft()!.gui.auto_reindex}
          onChange={(v) => setGui({ auto_reindex: v })}
          label="Auto-reindex"
          description="Re-index all enabled collections on a timer."
          hint="Re-indexes every enabled collection on a timer, so new and edited files show up without a manual run. A full pass uses CPU for a while, so it is off by default."
        />
        <Show when={draft()!.gui.auto_reindex}>
          <NumField
            label="Interval (minutes)"
            value={draft()!.gui.auto_reindex_interval_minutes}
            onChange={(n) => setGui({ auto_reindex_interval_minutes: n })}
            hint="How often the auto-reindex timer fires. 60 means once an hour. Only matters when Auto-reindex is on."
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
          hint="Opens vectile when you sign in, already indexing before you need it."
        />
      </FieldList>
    </Section>
  );
}
