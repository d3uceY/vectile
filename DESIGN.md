# Design — vectile

<!-- impeccable:design-schema 1 -->

## World

**Studio editorial, warm and precise.** A warm bone ground with clean white cards, hairline borders, soft shadows, and one dark-green accent that means the same thing everywhere: what is active, selectable, and actionable. A restrained warm amber marks attention and in-progress work; red means errors only. Sans type throughout. The app is a local knowledge library, so it reads like a calm, carefully set reference desk: clear, fast, quietly confident. Operate mode: expression lives in precise details, never at the cost of scanability or task clarity.

## Mode

Operate. Search, browse, and manage are the jobs; the interface stays out of the way and answers fast.

## Color

Warm-neutral ground (`--color-paper: #f6f4ef`); no background texture.

| Token | Value | Role |
|---|---|---|
| `paper` | `#f6f4ef` | warm bone ground |
| `surface` | `#ffffff` | white cards, `.sheet` |
| `surface-2` | `#f1efe9` | hovers, light fills |
| `ink` | `#24272c` | primary text |
| `ink-soft` | `#3f454d` | secondary |
| `muted` | `#5e656c` | supporting text |
| `faint` | `#767d85` | data labels |
| `line` / `line-strong` | `#e6e2da` / `#cfcac0` | hairline borders |
| `leaf` / `leaf-deep` | `#1e8a4e` / `#15703e` | brand, primary buttons, active/selected, links, success |
| `mint` / `mint-strong` | `#e4f3e9` / `#c8ecd6` | light green fills (active pills, selection) |
| `amber` / `amber-deep` | `#b45309` / `#92400e` | attention: unsaved, needs-reindex, recommended, write tools |
| `amber-soft` | `#fbeeda` | attention fills |
| `highlighter` | `#fff1a8` | keyword matches in snippets |
| `danger` | `#c13b2f` | errors |

One dark-green accent (`leaf-deep`) is the interaction color: active nav, selection, links, and primary buttons all read green, so "you can act here" keeps a single consistent meaning. Warm amber is reserved for things that need attention. The `indigo*` token family is aliased to the green values for legacy class-name compatibility.

## Typography

Two sans voices:

- **Plus Jakarta Sans** (variable) — everything: display headings at weight 600 with tight tracking, UI labels, buttons, inputs, nav, and body copy 13–16px.
- **IBM Plex Mono** — data only: paths, scores, counts, kbd hints, collection metadata. Never as costume.

The old Fraunces serif voice was removed; display hierarchy comes from weight and tracking, not a second typeface. Self-hosted via @fontsource; the app never hits a CDN at runtime.

## Surface rules

- Flat surfaces only: no noise, dot, or grid texture backgrounds.
- White cards on a warm bone ground; hairline 1px borders (`line`); cards are `.sheet` (border + 14px radius).
- Elevation declared once: cards may take `shadow-card` (offset + blur), never a zero-offset halo.
- No gradient text, no glass, no colored border-left/right accents, no emoji icons (all icons are drawn SVG, 24px grid, 1.75 stroke).
- Type measure 65–75ch on readable passages; tracking never below -0.02em at display size.

## Motion

One authored moment: result lists stagger in (fade + 4px rise, 220ms, snappy ease, 30ms steps). Buttons press to 0.98 scale in 120ms. The model-loaded dot pulses softly. Everything respects `prefers-reduced-motion`.

## Layout

- Left sidebar (224px, hairline right border): a title plate (logo + "vectile" wordmark), five nav items as index tabs, and a colophon plate in the footer carrying the model state, model name, and a "nothing leaves this machine" note. The active view is a dark-green (`leaf-deep`) square-cornered tab pulled 5px past the spine — clearly darker than the sidebar.
- Top status strip: "all local" + library summary (collections · chunks · size) left; Jump-to-search with ⌘K right.
- Main: max-width 980px column, 32px gutters, per-view scroll. White cards sit on the warm bone ground; no background textures.

## Views

- **Search** (home): large search bar with ⌘K; filter bar (collection, source type, advanced: path, sender/author, date, top-k); card results with title, highlighted snippet, rank (toggleable to score), collection chip, mono path, and a "Read full passage" expand that reveals Open file / Reveal in folder; a quiet stale-library hint under the filter bar; idle state with example queries; skeleton loading; honest empty state.
- **Library**: collections as expandable rows (type badge, file count, chunks, last indexed) revealing per-source file lists.
- **Browse**: collections → sources → chunks file tree (folders-first, indicator lines, arrow-key nav, expand/collapse all) beside a chunk preview card.
- **Index**: per-collection "Index new" / "Re-index all" / "Prune" with a header "Index all" / "Re-index all" and live progress. **Settings**: model, chunking, search, sources, and indexing ledgers with a sticky save bar when edited. **Connect** (a Settings section): a cardstock status plate for the in-app MCP server (state dot, mono `127.0.0.1:<port>/sse` URL with a copy button, "binds to 127.0.0.1 · nothing leaves this machine"), an enable toggle + port field (applied on save), an allow-write toggle that gates the index/prune tools, the five `vectile_*` tools listed as mono rows (write tools tagged), and copyable setup snippets for Claude Desktop, Claude Code, and any MCP SSE client. **First run**: a driver.js tour on an empty library walks Settings → Index → Search and never shows again.

## Direction contract

- THESIS: a local knowledge library that feels like a calm, carefully set reference desk — bright, precise, quietly clever; it refuses the dark "AI retrieval" default and the neon-glass dashboard.
- OWN-WORLD: warm bone ground, flat white cards, hairline borders, one dark-green interaction accent for active/selected/action, a warm amber for attention, sans type, mono data lines.
- STORY: the user's whole private library is searchable in one calm, fast surface; nothing leaves the machine.
- FIRST VIEWPORT: sidebar wordmark and nav, status strip, one big search bar, filter row, and a centered "Ask your library" idle state with example queries.
- FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, and DESIGN.md.
