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
| `paper-warm` | `#f0ece3` | cardstock plates |
| `surface` | `#ffffff` | white cards, `.sheet` |
| `surface-2` | `#f2efe9` | hovers, light fills |
| `sidebar` | `#f0ede7` | neutral column |
| `ink` | `#292721` | primary text (13.6:1 on paper) |
| `ink-soft` | `#48443c` | secondary (8.8:1 on paper) |
| `muted` | `#68645b` | supporting text (5.4:1 on paper, 4.6:1 worst case) |
| `faint` | `#807c72` | icons and decoration only, never text (3.8:1 on paper) |
| `line` | `#e6e2da` | decorative hairline: card edges, dividers, separators |
| `line-strong` | `#cac5bb` | decorative reinforcement: section cards, dashed empty states |
| `line-control` | `#8c877e` | control boundary: inputs, selects, switches, outline buttons (3:1+ on every plate) |
| `leaf` / `leaf-deep` | `#1e8a4e` / `#15703e` | brand, primary buttons, active/selected, links, success |
| `mint` / `mint-strong` | `#e5f3e8` / `#caecd3` | light green fills (active pills, selection) |
| `amber` / `amber-deep` | `#b45309` / `#92400e` | attention: unsaved, needs-reindex, recommended, write tools |
| `amber-soft` | `#fbeeda` | attention fills |
| `highlighter` | `#fff1a8` | keyword matches in snippets (carries `ink`, 13.1:1) |
| `danger` | `#c13b2f` | errors |

**One warm hue holds the whole neutral family** (OKLCH hue 82-92: `ink` 92, `muted` 87, `faint` 89, `line-control` 82, every surface and border 85-89). The ramp is authored in OKLCH and only lightness moves between stops: `ink` L 0.27, `ink-soft` L 0.39, `muted` L 0.50, `faint` L 0.59, `line-control` L 0.63, `line-strong` L 0.82, `line` L 0.91, `paper` L 0.97. Chroma stays at 0.007-0.016, so the neutrals are warm greys rather than a second color, and the ordering survives grayscale. The mint fills hold the leaf hue (153), so the green family is one scale instead of two.

Three rules that follow from the ramp:

- **`faint` is not a text color.** A fourth text step light enough to read as "quieter than `muted`" cannot reach 4.5:1 on bone (the darkest plate pins it around 4.2:1). `muted` is the lightest text token; `faint` is for chevrons, remove buttons, folder and file icons, and dot fills, and clears 3:1 on every surface.
- **`line-control` is the only border that carries meaning.** A field fill is white on a white desk, so the border is the whole boundary; `line` and `line-strong` are decoration and stay light. Focus rings are `leaf-deep` at 5.6:1 on paper.
- **Publish measured numbers, not eyeballed ones.** Every pair above was gated with `retna contrast` at 4.5:1 for text and 3:1 for non-text (52 pairs, 0 failures).

One dark-green accent (`leaf-deep`) is the interaction color: active nav, selection, links, and primary buttons all read green, so "you can act here" keeps a single consistent meaning. Warm amber is reserved for things that need attention. The `indigo*` token family is aliased to the green values for legacy class-name compatibility.

## Typography

Two sans voices:

- **Plus Jakarta Sans** (variable) — everything: display headings at weight 600 with tight tracking, UI labels, buttons, inputs, nav, and body copy 13–16px.
- **IBM Plex Mono** — data only: paths, scores, counts, kbd hints, collection metadata. Never as costume.

The old Fraunces serif voice was removed; display hierarchy comes from weight and tracking, not a second typeface. Self-hosted via @fontsource; the app never hits a CDN at runtime.

## Surface rules

- Flat surfaces only: no noise, dot, or grid texture backgrounds.
- White cards on a warm bone ground; hairline 1px borders (`line`); cards are `.sheet` (border + 14px radius).
- Settings is the one white desk: its pane is `surface` and its section cards are bone (`paper`, `line-strong`). Anything nested inside those cards is either a **cardstock plate** (`paper-warm`, `line` hairline) or a **field** (`surface` fill, `line-strong` border). A fill that is neither is not a fill: ghost values like `bg-surface/20` are banned, because on bone they render as nothing.
- Popovers are the lightest layer: dropdown menus and info tips are `surface` with a `line-strong` hairline and `shadow-pop`, so they read above both the bone cards and the white desk.
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
- **Index**: per-collection "Index new" / "Re-index all" / "Prune" with a header "Add sources" (opens Settings → Sources), "Index all", and "Re-index all", plus live progress. Every row states its kind with a source icon and a label (vaults, ebooks, project folders, code repos), and a dashed footer names any kind this library is not indexing yet. **Settings**: model, chunking, search, sources, and indexing ledgers with a sticky save bar when edited. **Sources**: one tab per source type (project folders, code repositories, Obsidian vaults, Calibre libraries) in a two-column picker that carries a count per type and a white field plate whose active cell is the dark-green pill. The active panel states what its paths become (one collection named after the group, or the single `obsidian` / `calibre` collection), holds that type's editor, and nests its own excluded-folder list under a hairline. **Connect** (a Settings section): a cardstock status plate for the in-app MCP server (state dot, mono `127.0.0.1:<port>/sse` URL with a copy button, "binds to 127.0.0.1 · nothing leaves this machine"), an enable toggle + port field (applied on save), an allow-write toggle that gates the index/prune tools, the five `vectile_*` tools listed as mono rows (write tools tagged), and copyable setup snippets for Claude Desktop, Claude Code, and any MCP SSE client. **First run**: a driver.js tour on an empty library walks Settings → Index → Search and never shows again.

## Direction contract

- THESIS: a local knowledge library that feels like a calm, carefully set reference desk — bright, precise, quietly clever; it refuses the dark "AI retrieval" default and the neon-glass dashboard.
- OWN-WORLD: warm bone ground, flat white cards, hairline borders, one dark-green interaction accent for active/selected/action, a warm amber for attention, sans type, mono data lines.
- STORY: the user's whole private library is searchable in one calm, fast surface; nothing leaves the machine.
- FIRST VIEWPORT: sidebar wordmark and nav, status strip, one big search bar, filter row, and a centered "Ask your library" idle state with example queries.
- FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, and DESIGN.md.
