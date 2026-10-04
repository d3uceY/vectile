# Design — vectile

<!-- impeccable:design-schema 1 -->

## World

**Cool-neutral reference desk.** A white ground with panel cards, hairline borders, a five-stop text ladder, and one dark-green accent that means the same thing everywhere: what is active, selectable, and actionable. Warm amber marks attention and in-progress work; red means errors only. One sans (Albert Sans) carries every voice; IBM Plex Mono handles data lines only. The app is a local knowledge library, so it reads like a calm, precisely set index: clear, fast, quietly confident. Operate mode: expression lives in precise details, never at the cost of scanability or task clarity.

The interface borrows its system from OpenDesign: a cool-neutral ramp, a strict radius scale, a small type scale with a 500/600/700 weight ladder, one authored ease, and card/row/drawer recipes that repeat. Surfaces stay flat — no glass, no frosted layers, no textures.

## Mode

Operate. Search, browse, and manage are the jobs; the interface stays out of the way and answers fast.

## Color

White ground (`--color-paper: #ffffff`); panel cards sit at `--color-surface: #fafafa` with `--color-line-strong: #dbdbdb` hairlines. No background texture. Dark inverts the grounds and raises the ink (see **Themes** below).

One pinned exception sits outside this palette. The **source folder art** in Browse (`frontend/public/folder.svg`) is the user's own asset, and its macOS yellow (`#FFCE00` flap, `#EDAF00` back) is deliberately the loudest hue on that surface. It is artwork the user pinned, not a second interface accent: the chrome around it stays white, panel-grey, and dark green.

| Token | Value | Role |
|---|---|---|
| `paper` | `#ffffff` | app ground |
| `surface` | `#fafafa` | cards, panels, settings sections |
| `surface-2` | `#ededed` | hover fills, rails, control tracks, tag fills |
| `sidebar` | `#fafafa` | navigation column |
| `paper-warm` | `#ededed` | legacy alias, now equal to `surface-2` |
| `ink` | `#202020` | primary text (16.3:1 on paper) |
| `ink-soft` | `#494949` | secondary (9.0:1 on paper) |
| `muted` | `#5c5c5c` | supporting text (6.7:1 on paper, 6.4:1 on surface) |
| `faint` | `#848484` | icons and decoration only, never text (3.7:1, clears 3:1 non-text) |
| `ghost` | `#bdbdbd` | decoration only, never text or icons (dashes, dot fills) |
| `line` | `#ededed` | decorative hairline: dividers, separators, card edges |
| `line-strong` | `#dbdbdb` | standard card / section border, dashed empty states |
| `line-control` | `#8c8c8c` | control boundary: inputs, selects, switches, outline buttons (3.2:1+) |
| `leaf` / `leaf-deep` | `#1e8a4e` / `#15703e` | brand, primary buttons, active/selected, links, success |
| `mint` / `mint-strong` | `#e5f3e8` / `#caecd3` | light green fills (active rows, selection) |
| `amber` / `amber-deep` | `#b45309` / `#92400e` | attention: unsaved, needs-reindex, recommended, write tools |
| `amber-soft` / `amber-warm` | `#fbeeda` / `#f0d9ad` | attention fills + border |
| `highlighter` | `#fff1a8` | keyword matches in snippets (carries `ink`, 14.3:1) |
| `danger` | `#c13b2f` | errors (5.3:1, and on `white` for filled buttons) |

**Three rules follow from the ramp:**

- **`faint` and `ghost` are not text colors.** `muted` is the lightest text token (6.4:1 worst case). `faint` is for chevrons, remove buttons, folder and file icons, and dot fills; `ghost` is for hairlines under dashes and idle dot fills only.
- **`line-control` is the only border that carries meaning.** A field fill is white on a white desk, so the border is the whole boundary; `line` and `line-strong` are decoration and stay light. Focus rings are `leaf-deep` at 2px.
- **Publish measured numbers, not eyeballed ones.** Every pair above was gated with `retna contrast` at 4.5:1 for text and 3:1 for non-text (20 pairs, 0 text failures).

One dark-green accent (`leaf-deep`) is the interaction color: active nav, selection, links, and primary buttons all read green, so "you can act here" keeps a single consistent meaning. Warm amber is reserved for things that need attention. The `indigo*` token family is aliased to the green values for legacy class-name compatibility; it is no longer used in new code.

## Themes

Two themes, one set of token names. Light is the `@theme` default; dark is a single `html[data-theme="dark"]` block in `frontend/src/index.css` that overrides only token **values**, so every utility re-themes itself with no component branching. `color-scheme` flips with it so native controls (scrollbars, date pickers) match.

Preference is `system` (default), `light`, or `dark`, stored in `localStorage` under `vectile.theme` and applied as `data-theme="light|dark"` on `<html>`. `frontend/index.html` repeats the same resolution inline before first paint so there is no flash of the wrong theme; `frontend/src/lib/theme.ts` owns the logic and `components/shell/ThemeToggle.tsx` is the control in the sidebar footer (three segments on the rail, one cycling icon on the 64px icon rail).

| Token | Light | Dark |
|---|---|---|
| `paper` | `#ffffff` | `#1c1c1c` |
| `surface` | `#fafafa` | `#262626` |
| `surface-2` | `#ededed` | `#303030` |
| `sidebar` | `#fafafa` | `#202020` |
| `ink` | `#202020` | `#fafafa` |
| `ink-soft` | `#494949` | `#d4d4d4` |
| `muted` | `#5c5c5c` | `#bdbdbd` |
| `faint` | `#848484` | `#8f8f8f` |
| `ghost` | `#bdbdbd` | `#5c5c5c` |
| `line` / `line-strong` | `#ededed` / `#dbdbdb` | `#333333` / `#454545` |
| `line-control` | `#8c8c8c` | `#7a7a7a` |
| `leaf-deep` (ink) | `#15703e` | `#3bbf7d` |
| `accent` / `accent-ink` (fill) | `#15703e` / `#ffffff` | `#3bbf7d` / `#14201a` |
| `leaf` | `#1e8a4e` | `#4ecb8c` |
| `mint` / `mint-strong` | `#e5f3e8` / `#caecd3` | `#16301f` / `#1d3d29` |
| `amber` / `amber-deep` / `amber-soft` | `#b45309` / `#92400e` / `#fbeeda` | `#f0b453` / `#ffc740` / `#3a2a12` |
| `highlighter` | `#fff1a8` | `#6b5a12` |
| `danger` (ink) / `danger-strong` (fill) | `#c13b2f` / `#c13b2f` | `#ff7b72` / `#c13b2f` |

**The one structural change dark forced: fills and inks are separate tokens.** A single green cannot both sit under white button text and read as a link on a dark ground. So `accent` + `accent-ink` carry the *fill* (primary buttons, checkboxes, switch tracks) and `leaf-deep` carries the *ink* (links, active rows, focus, success text); dark brightens the ink and flips the fill's label to near-black. `danger` (ink) and `danger-strong` (fill) work the same way. Light values are unchanged, so light renders exactly as before.

The toast pill inverts with the theme by construction (`bg-ink` + `text-paper`), rather than hardcoding white text on a dark pill.

Dark pairs were gated with `retna contrast` (20 pairs). Every text pair clears 4.5:1: ink 16.3, ink-soft 11.5, muted 9.1 / 8.1 / 7.0, leaf-deep 7.3 / 6.4 / 6.1, accent-ink on accent 7.1, amber 9.2, amber-deep on amber-soft 8.9, danger 6.8, white on danger-strong 5.3, ink on highlighter 6.5, paper on ink 16.3. `faint` (5.3) is comfortable as text in dark but stays documented as icons-only for parity with light; `ghost` (2.6) and `line-control` (4.0) are non-text and clear the 3:1 rule.

`main.go`'s window `BackgroundColour` matches the light ground; the webview paints the themed ground immediately, so the native colour only shows for a frame on window resize.

## Typography

Two voices, both self-hosted via @fontsource (a desktop app never hits a CDN at runtime):

- **Albert Sans** (variable, normal + italic) — everything: display headings at weight 600 with tight tracking, UI labels, buttons, inputs, nav, and body copy.
- **IBM Plex Mono** — data only: paths, scores, counts, kbd hints, transport labels. Never as costume.

Scale: `12 / 13 / 14 / 16 / 18 / 20 / 24 / 32`. Body defaults to `14px / 1.4` at weight 500. The weight ladder is **500 / 600 / 700**: 500 for body and quiet rows, 600 for labels, controls, and headings, 700 for uppercase status pills and badges only. Display hierarchy comes from size, weight, and tracking, never a second typeface. Uppercase eyebrows sit at `11px / 600 / 0.06em`.

## Surface rules

- Flat surfaces only: no noise, dot, or grid texture backgrounds, no glass, no `backdrop-filter`.
- Cards are the OpenDesign recipe: `background: surface` + `1px solid line-strong` + `8px` radius + `shadow-xs`. They lift on hover with `translateY(-2px)` + `line-strong` + `shadow-card`, never a scale.
- Rows are `padding: 14px 16px` with a `1px solid line` hairline; hover fills with a lighter panel tone.
- The settings pane is `paper`; its sections are `surface` cards. Nested plates use `surface-2`, and fields use a `paper` fill with a `line-control` boundary.
- Popovers and menus are the lightest layer: `paper` with a `line-strong` hairline and `shadow-pop`.
- Elevation is declared once, in four steps: `shadow-xs`, `shadow-card`, `shadow-pop`, `shadow-overlay`. No zero-offset halos.
- No gradient text, no colored border-left/right accents, no emoji icons (all icons are drawn SVG, 24px grid, 1.75 stroke, rendered at 18px in nav rails).
- Type measure 65–75ch on readable passages; tracking never below -0.02em at display size.

## Motion

OpenDesign's motion vocabulary. Durations are `100 / 150 / 200 / 250ms`; enter is ~200ms, exit ~140ms. One authored ease, `--ease-snappy: cubic-bezier(0.23, 1, 0.32, 1)`. Animated properties are `transform` and `opacity` only.

- Result lists and idle example chips use `.enter-stagger` / `.od-stagger` (fade + 10px rise, 200ms, 40ms steps).
- Entrance utilities: `.od-fade-in`, `.od-fade-slide-up`, `.od-popover-in`, `.od-slide-left`, `.od-scale-in`.
- Drawers slide `translateX(18px)` + fade over 220ms; dialogs fade the backdrop at 150ms and scale the panel `0.97 → 1` over 250ms. Never animate from `scale(0)`.
- Buttons press with `translateY(1px)` over 100ms.
- Accordions animate `grid-template-rows: 0fr → 1fr` over 200ms.
- The indexing loader keeps its width-based fill (a deliberate exception to transform-only), caps at 96%, and carries a sweeping shine so it reads as "pushing".
- Everything respects `prefers-reduced-motion`, which neutralizes animation and transition durations.

## Layout

- Left rail (224px, hairline right border, `surface` fill): a wordmark, five destinations as 38px rounded rows (18px icon slot, 13px/600 label, 12px radius, 2px gap). The active row is a **mint fill with `leaf-deep` ink** — green still means "you are here". A colophon dock in the footer carries the model state, model name, and a "nothing leaves this machine" note.
- Top chrome strip: 44px tall, hairline bottom border, library summary (collections · chunks · size · last indexed) at 12px/500 left; "Jump to search" + ⌘K and the version right.
- Main: max-width 980px column, per-view scroll. Cards sit on the white ground.

## Views

- **Search** (home): a 44px search bar with ⌘K; a filter row (collection, source type, advanced: path, sender/author, date, top-k); result cards with title, highlighted snippet (yellow highlighter), rank (toggleable to score), collection chip, mono path, and a "Read full passage" expander revealing Open file / Reveal in folder; a quiet stale-library hint under the filter bar; a 32px idle hero with staggered example chips; skeleton loading; honest dashed empty state.
- **Library**: collections as an expandable panel table (uppercase 11px/600 column heads on `surface-2`, 13px/600 names, type chips, amber needs-reindex tag, mono counts) revealing per-source file rows.
- **Browse**: a collection as a gallery grid of source cards — the pinned folder art on a `surface-2` plate, the chunk count pill on the flap, the source name in a hairline footer bar — that opens a **floating rounded drawer** (480px, mono chunk count under a 17px/600 title) with that file's chunks; opening a chunk swaps the drawer to its text at a 68ch measure. Delete library sits in the collection bar above the grid; chunk selection and deletion live in the drawer.
- **Index**: per-collection cards (icon tile, 14px/600 name, kind chip, mono `sources · chunks`), primary "Index new", outline "Re-index all", quiet "Prune", an enable switch, and an inline progress bar. A header carries "Add sources" / "Index all" / "Re-index all"; a dashed footer names any kind this library is not indexing yet. **Settings**: a grouped left rail (Engine / Library / Companions, 12px/600 labels, 18px icons, 8px-radius rows, green active) beside a `paper` pane of `surface` section cards — Model, OCR, Chunking, Search, Cache, Sources, Indexing, Vexter, Connect. Fields are `field-row`s divided by hairlines with mono readouts; a sticky bar with an amber "unsaved" pill appears while the draft is dirty. **Sources**: one tab per source type in a two-column picker carrying a count per type; the active panel states what its paths become and nests its own skip list under a hairline. **Connect**: a status plate (state dot, mono URL, copy button, scope line), enable/transport/port/allow-write controls, the write-tagged `vectile_*` tools as mono rows, and copyable setup snippets per client. **Dialogs**: 420–480px panels, 12px radius, 22px padding, `shadow-overlay`, 17px/600 title, 13px muted body, right-aligned pill footer buttons. **Toasts**: a single bottom-center dark pill at 13px, rising 8px over 200ms. **First run**: a driver.js tour on an empty library walks Settings → Index → Search and never shows again.

## Direction contract

- THESIS: a local knowledge library that feels like a calm, cool-neutral reference desk — bright, precise, quietly clever; it refuses the dark "AI retrieval" default and the neon-glass dashboard.
- OWN-WORLD: white ground, panel cards, hairline borders, a five-stop text ladder, Albert Sans type, mono data lines, one dark-green interaction accent for active/selected/action (brightened to emerald in dark), a warm amber for attention, flat surfaces and OpenDesign motion. Light and dark share one token set; only values swap.
- STORY: the user's whole private library is searchable in one calm, fast surface; nothing leaves the machine.
- FIRST VIEWPORT: rail wordmark and nav, 44px chrome strip, one search bar, a filter row, and a centered "Ask your library" idle state with example chips.
- FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, and DESIGN.md.
