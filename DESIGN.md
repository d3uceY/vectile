# Design — vectile

<!-- impeccable:design-schema 1 -->

## World

**Field notebook / index-card library.** A bright paper ground with a faint graph-paper dot texture, hairline borders, one friendly leaf-green accent, mono data labels, and a serif-italic "margin note" voice for asides. The app is a local knowledge library, so it reads like a well-kept notebook: calm, precise, quietly clever. Operate mode: expression lives in precise details, never at the cost of scanability or task clarity.

## Mode

Operate. Search, browse, and manage are the jobs; the interface stays out of the way and answers fast.

## Color

Restrained palette on a paper ground (`--color-paper: #fbfcff`).

| Token | Value | Role |
|---|---|---|
| `paper` | `#fbfcff` | ground |
| `ink` | `#1b2226` | primary text |
| `ink-soft` | `#39434b` | secondary |
| `muted` | `#56616b` | supporting text (6.1:1) |
| `faint` | `#66707a` | data labels, paths, placeholders (4.9:1) |
| `line` / `line-strong` | `#e4e8ec` / `#d1d8de` | hairline borders |
| `mint` | `#e5f6ec` | the light friendly green — fills: selection, active nav, open folders |
| `mint-strong` | `#c9eed8` | selection color |
| `leaf` | `#1f8a50` | decorative accent (icons, dots) |
| `leaf-deep` | `#177340` | interactive text and primary buttons (5.7:1) |
| `highlighter` | `#fff1a8` | keyword matches in snippets (the one warm accent) |
| `danger` | `#c13b2f` | errors |

The green is one accent, used at page scale for what is selected or active, not scattered. The warm highlighter is reserved for matched terms only.

## Typography

Three voices:

- **Fraunces** (variable, opsz) — the serif voice, and the one with the personality: page heads at 28px with tight -0.025em tracking, section/smaller titles, and reading passages (search snippets, document chunks) set like book text. Optical sizing (`font-optical-sizing: auto`) keeps the face open at small sizes and showy at display sizes. The serif-italic margin-note voice lives here too.
- **Plus Jakarta Sans** (variable) — the chrome: UI labels, buttons, inputs, nav, body copy 13–16px. Clean and highly legible next to the serif.
- **IBM Plex Mono** — data only: paths, scores, counts, kbd hints, collection metadata. Never as costume.

Self-hosted via @fontsource; the app never hits a CDN at runtime.

## Surface rules

- Hairline 1px borders (`line`) on paper; cards are `sheet` (border + 14px radius), never nested.
- Elevation declared once: cards may take `shadow-card` (offset + blur), never a zero-offset halo.
- No gradient text, no glass, no colored border-left/right accents, no emoji icons (all icons are drawn SVG, 24px grid, 1.75 stroke).
- Type measure 65–75ch on readable passages; tracking never below -0.02em at display size.

## Motion

One authored moment: result lists stagger in (fade + 4px rise, 220ms, snappy ease, 30ms steps). Buttons press to 0.98 scale in 120ms. The model-loaded dot pulses softly. Everything respects `prefers-reduced-motion`.

## Layout

- Left sidebar (224px, hairline right border): a title plate — logo in a hairline bookplate + "vectile" in the serif voice, serif-italic tagline "your private library" — then five nav items as filing-cabinet index tabs (the active view is a mint square-cornered tab pulled 5px past the spine), and a cardstock colophon plate in the footer carrying the model state, model name, and a serif-italic "nothing leaves this machine" note.
- Top status strip: "all local" + library summary (collections · chunks · size) left; Jump-to-search with ⌘K right.
- Main: max-width 980px column, 32px gutters, per-view scroll.
- Dot pattern (20px grid, ~10% ink, 1.25px dots) is the app-wide ground — visible but quiet; grid pattern (34–40px, ~7% leaf) is the "blueprint" surface for Browse, Library, and empty states.

## Views

- **Search** (home): large search bar with ⌘K; filter bar (collection, source type, advanced: path, sender/author, date, top-k); index-card results with title, highlighted snippet, rank (toggleable to score), collection chip, mono path, and a "Read full passage" expand that reveals Open file / Reveal in folder; a quiet stale-library hint under the filter bar; idle state with example queries on graph paper; skeleton loading; honest empty state.
- **Library**: collections as expandable rows (type badge, file count, chunks, last indexed) revealing per-source file lists.
- **Browse**: collections → sources → chunks file tree (folders-first, indicator lines, arrow-key nav, expand/collapse all) beside a chunk preview card.
- **Index**: per-collection "Index new" / "Re-index all" / "Prune" with a header "Index all" / "Re-index all" and live progress. **Settings**: model, chunking, search, sources, and indexing ledgers with a sticky save bar when edited. **Connect** (a Settings section): a cardstock status plate for the in-app MCP server (state dot, mono `127.0.0.1:<port>/sse` URL with a copy button, "binds to 127.0.0.1 · nothing leaves this machine"), an enable toggle + port field (applied on save), an allow-write toggle that gates the index/prune tools, the five `vectile_*` tools listed as mono rows (write tools tagged), and copyable setup snippets for Claude Desktop, Claude Code, and any MCP SSE client. **First run**: a driver.js tour on an empty library walks Settings → Index → Search and never shows again.

## Direction contract

- THESIS: a local knowledge library that feels like a well-kept notebook — bright, precise, quietly clever; it refuses the dark "AI retrieval" default and the neon-glass dashboard.
- OWN-WORLD: paper ground, graph-paper dot/grid textures, hairline borders, one leaf-green accent for active/selected, mono data lines, serif-italic margin notes.
- STORY: the user's whole private library is searchable in one calm, fast surface; nothing leaves the machine.
- FIRST VIEWPORT: sidebar wordmark and nav, status strip, one big search bar, filter row, and a centered "Ask your library" idle state with example queries.
- FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, and DESIGN.md.
