# vectile documentation site

The documentation site for vectile. Built with [Docusaurus](https://docusaurus.io) 3, rendered as a static site.

## Run it

From the repository root:

```
task docs        # serves on http://localhost:3000
task docs:build  # builds into website/build
```

Or from this folder:

```bash
npm install
npm run dev      # same as npm start
npm run build
```

## Layout

| Path | What lives there |
|---|---|
| `docs/` | The documentation pages, one file per page, linked from `sidebars.ts` |
| `src/pages/index.tsx` | The landing page |
| `src/components/SearchDemo.tsx` | The interactive search card in the hero |
| `src/components/Shot.tsx` | The figure used for every documentation screenshot |
| `src/components/Reveal.tsx` | Scroll reveal, skipped under reduced motion |
| `src/css/custom.css` | Design tokens, Infima overrides, docs and figure styling |
| `src/pages/index.module.css` | Landing page styles |
| `static/screenshots/` | App screenshots captured from the real UI |
| `docusaurus.config.ts` | Site config, navbar, footer, docs options |

## Writing docs

Three rules, and they come from the app itself:

1. **No em dashes.** Use a period, a colon, or a semicolon. This matches the copy convention in the desktop app.
2. **Say the real thing.** If a control has a range, state it. If something is not supported, say so plainly and say why.
3. **Keep the measure readable.** Concrete numbers, no filler sentences.

Docs are Markdown, parsed as MDX. A `mermaid` code fence renders a diagram. Screenshots use the `Shot` component:

```mdx
import Shot from '@site/src/components/Shot';

<Shot src="screenshots/search.png" alt="..." caption="..." />
```

Wrap two of them in `<div className="doc-shots">` for a side by side row.

## Screenshots

Every figure on the docs pages comes from the running app, not from a mockup. To refresh them, from the repository root:

```bash
node scripts/dev-stub.mjs
```

then in a second terminal:

```bash
node C:\Users\deuce\.copilot\skills\screenshot-skill\scripts\capture.mjs --config scripts/capture.config.json
```

That writes `docs/screenshots/*.png` at 1440x900, which the README also uses. Copy them into the site afterwards:

```powershell
Copy-Item docs/screenshots/*.png website/static/screenshots/ -Force
```

Two things worth knowing about the pipeline:

- `scripts/vectile-interact.mjs` drives each shot. `interactView` picks the view, `settingsSection` opens one Settings rail card, and `searchExpand` opens the first result's full passage.
- The `Connect` page carries its own `loadingSelectors` without `[role="status"]`. The copy button's live region is a 1x1 clipped box that Playwright counts as a visible loading indicator, so without that override capture refuses to save the shot.

The demo data in `scripts/vectile-stub.mjs` is set to the app's real defaults (chunk 500/50, top k 10, weights 0.7/0.3) so the screenshots agree with the numbers the docs state.

## Design

The site carries the desktop app's world: a warm bone ground (`#f6f4ef`), flat white cards on hairline borders, one dark green accent (`#15703e`) for anything actionable, and warm amber for attention. Type is Plus Jakarta Sans for everything and IBM Plex Mono for data, both self-hosted through `@fontsource` so the site never reaches for a CDN. There is no dark mode, by design: the app does not have one either.

## Dependencies

Two notes for whoever touches `package.json` next.

**mermaid is pinned.** `@docusaurus/theme-mermaid` declares `mermaid: ">=11.6.0"` with no upper bound, so a fresh install can pick up a major the theme was never tested against. The `overrides` entry holds it on the 11.x line. Remove the pin only after checking a newer major **in a browser**: the theme renders diagrams client side, so a broken upgrade produces a page that builds cleanly and shows no diagram, with nothing in the build output to warn you.

**`npm audit` will not reach zero.** The standing count is about 22 advisories, all moderate except `serialize-javascript`:

| Source | Why it cannot be fixed |
|---|---|
| `serialize-javascript`, via `copy-webpack-plugin` and `css-minimizer-webpack-plugin` | No patched release exists. Build time only. |
| `uuid`, via `sockjs` and `webpack-dev-server` | Dev server only, and the advisory covers an API path sockjs does not use. |
| Everything else | Inherits one of the two above through `@docusaurus/core`. |

None of these are reachable at runtime. The site ships static HTML, CSS, and JS, and every flagged package runs during the build or on the local dev server. `npm audit fix --force` is not a fix here: it clears the reports by downgrading Docusaurus.

## Deploying

The site is fully static. `npm run build` writes `build/`, which any static host can serve.

`url` and `baseUrl` in `docusaurus.config.ts` are set for a GitHub Pages project site (`https://d3ucey.github.io/vectile/`). Change those two values if you deploy somewhere else.
