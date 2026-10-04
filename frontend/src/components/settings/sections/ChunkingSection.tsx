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
          hint="How many words each indexed slice holds. Search matches slices, not whole files, so this sets how finely results are cut. Smaller chunks match more precisely; bigger ones carry more context. 500 is a safe start."
          min={STATIC_BOUNDS.chunk_size_tokens.min}
          max={STATIC_BOUNDS.chunk_size_tokens.max}
          step={STATIC_BOUNDS.chunk_size_tokens.step}
        />
        <NumField
          label="Chunk overlap (words)"
          value={draft()!.chunk_overlap_tokens}
          onChange={(n) => setNumber("chunk_overlap_tokens", n)}
          hint="How many words repeat from one slice into the next, so sentences that straddle a cut stay searchable whole. Too little overlap and text slips through; too much and it gets stored twice. 50 is the usual start."
          min={STATIC_BOUNDS.chunk_overlap_tokens.min}
          max={Math.max(STATIC_BOUNDS.chunk_overlap_tokens.min, draft()!.chunk_size_tokens - 1)}
          step={STATIC_BOUNDS.chunk_overlap_tokens.step}
        />
      </FieldList>
    </Section>
  );
}
