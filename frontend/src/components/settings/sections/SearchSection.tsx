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
              hint="Results a search returns by default. Override it per search under Filters."
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
              hint="Smoothing constant for merging the two result lists. Higher k flattens rank gaps, giving lower matches a fairer shot. 60 is standard."
            />
            <RangeField
              label="Vector weight"
              value={draft()!.search_defaults.vector_weight}
              onChange={(n) => setSearch("vector_weight", n)}
              hint="Weight of meaning-based ranking. Raise it and results lean semantic, even when the words don't line up."
              min={STATIC_BOUNDS.vector_weight.min}
              max={STATIC_BOUNDS.vector_weight.max}
              step={STATIC_BOUNDS.vector_weight.step}
            />
            <RangeField
              label="Full-text weight"
              value={draft()!.search_defaults.fts_weight}
              onChange={(n) => setSearch("fts_weight", n)}
              hint="Weight of exact-word matches. Raise it for precise phrases and names."
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
