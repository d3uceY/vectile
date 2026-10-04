import { SearchNavIcon } from "../../ui/nav-icons";
import { STATIC_BOUNDS } from "../bounds";
import { FieldList, NumField, RangeField, Section, SubHeading } from "../fields";
import { useSettings } from "../context";

export function SearchSection() {
  const { draft, setSearch } = useSettings();

  return (
    <Section
      icon={<SearchNavIcon size={16} />}
      title="Search"
      note="Hybrid ranking blends exact-term and meaning results."
    >
      <div class="space-y-6">
        <div>
          <SubHeading>Results</SubHeading>
          <FieldList>
            <NumField
              label="Top results"
              value={draft()!.search_defaults.top_k}
              onChange={(n) => setSearch("top_k", n)}
              hint="How many matches a search returns by default. Raise it for a longer list, lower it for a shorter one. Set a different number per search under Filters."
              min={STATIC_BOUNDS.top_k.min}
              max={STATIC_BOUNDS.top_k.max}
              step={STATIC_BOUNDS.top_k.step}
            />
          </FieldList>
        </div>

        <div>
          <SubHeading>Ranking blend</SubHeading>
          <FieldList>
            <NumField
              label="RRF constant (k)"
              value={draft()!.search_defaults.rrf_k}
              onChange={(n) => setSearch("rrf_k", n)}
              hint="A smoothing value in the math that merges the two search lists. Bigger k flattens the gap between high- and low-ranked matches, so entries further down still get a fair shot. 60 is the usual value."
              min={STATIC_BOUNDS.rrf_k.min}
              max={STATIC_BOUNDS.rrf_k.max}
              step={STATIC_BOUNDS.rrf_k.step}
            />
            <RangeField
              label="Vector weight"
              value={draft()!.search_defaults.vector_weight}
              onChange={(n) => setSearch("vector_weight", n)}
              hint="How much the meaning-based ranking counts when the two search lists are blended. It works against the full-text weight like a seesaw: raise it and results lean toward semantic matches, even when the words don't line up exactly."
              min={STATIC_BOUNDS.vector_weight.min}
              max={STATIC_BOUNDS.vector_weight.max}
              step={STATIC_BOUNDS.vector_weight.step}
            />
            <RangeField
              label="Full-text weight"
              value={draft()!.search_defaults.fts_weight}
              onChange={(n) => setSearch("fts_weight", n)}
              hint="How much exact-word matches count in the final blend. Raise it when you're hunting a precise phrase or a name and want literal hits to win. Lower it and meaning takes over from wording."
              min={STATIC_BOUNDS.fts_weight.min}
              max={STATIC_BOUNDS.fts_weight.max}
              step={STATIC_BOUNDS.fts_weight.step}
            />
          </FieldList>
        </div>
      </div>
    </Section>
  );
}
