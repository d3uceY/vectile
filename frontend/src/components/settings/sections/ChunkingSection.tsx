import { ChunkNavIcon } from "../../ui/nav-icons";
import { STATIC_BOUNDS } from "../bounds";
import { FieldList, NumField, Section } from "../fields";
import { useSettings } from "../context";

export function ChunkingSection() {
  const { draft, setNumber } = useSettings();

  return (
    <Section
      icon={<ChunkNavIcon size={16} />}
      title="Chunking"
      note="Smaller chunks match more precisely; overlap keeps sentences intact."
    >
      <FieldList>
        <NumField
          label="Chunk size (words)"
          value={draft()!.chunk_size_tokens}
          onChange={(n) => setNumber("chunk_size_tokens", n)}
          hint="Words per indexed slice. Search matches slices, not whole files: smaller is more precise, bigger carries more context. 500 is a safe start."
          min={STATIC_BOUNDS.chunk_size_tokens.min}
          max={STATIC_BOUNDS.chunk_size_tokens.max}
          step={STATIC_BOUNDS.chunk_size_tokens.step}
        />
        <NumField
          label="Chunk overlap (words)"
          value={draft()!.chunk_overlap_tokens}
          onChange={(n) => setNumber("chunk_overlap_tokens", n)}
          hint="Words repeated from one slice into the next, so sentences that straddle a cut stay whole. Too little and text slips through; too much and it is stored twice. 50 is a safe start."
          min={STATIC_BOUNDS.chunk_overlap_tokens.min}
          max={Math.max(STATIC_BOUNDS.chunk_overlap_tokens.min, draft()!.chunk_size_tokens - 1)}
          step={STATIC_BOUNDS.chunk_overlap_tokens.step}
        />
      </FieldList>
    </Section>
  );
}
