---
target: Connect section (Settings) - MCP client connection UI
total_score: 20
max_score: 40
na_heuristics: 
p0_count: 1
p1_count: 3
timestamp: 2026-09-18T11-24-36Z
slug: frontend-src-components-settings-settingsview-tsx
---
Method: dual-agent (A: Explore · B: Explore), parent-run browser overlay

## Design Health Score

Scored on the Connect section **as reviewed** (before this pass's change). Items marked ✅ were addressed in the same pass; the rest are still open.

| # | Heuristic | Score | Key Issue |
|---|-----------|-------|-----------|
| 1 | Visibility of System Status | 2 | Status plate reads saved state (`mcpStatus()`) while the switch above it edits draft state (`draft()!.mcp.enabled`). Flip the switch and the plate still says "stopped". |
| 2 | Match System / Real World | 2 | `--transport sse`, "Any MCP SSE client", `mcpServers` are protocol vocabulary in a product that otherwise says "Share your library". ✅ "Any MCP SSE client" is now "Other". |
| 3 | User Control and Freedom | 2 | No stop/restart from the section, no test-connection, copy failure silently swallowed, no undo path. |
| 4 | Consistency and Standards | 3 | Copy buttons had `hover:` but no `focus-visible:` ring, while `Switch` uses one. ✅ Fixed. |
| 5 | Error Prevention | 1 | One unconfirmed toggle grants an AI read access to every collection. Port edits silently desync the payloads from the running server. No port-availability check. |
| 6 | Recognition Rather Than Recall | 1 | Nothing answers "which client do I use, and where does the file live?". The destination path — the one thing a user cannot infer — was absent. ✅ Per-client destination + extra step now shown. |
| 7 | Flexibility and Efficiency | 2 | `claude mcp add` was snippet #2 of 3 with no advanced grouping; no way to see all clients at once. ✅ Per-client tabs. |
| 8 | Aesthetic and Minimalist Design | 3 | Four copy buttons for one URL, `PlugIcon` twice, a hairline divider splitting one thought, two `bg-surface/20` fills that render as ~nothing. |
| 9 | Error Recovery | 1 | `MCPStatus` has no error field. A bind failure is a transient red toast racing a green "Settings saved" toast, then the plate tells the user to "Enable it below, then save settings" — which they just did. |
| 10 | Help and Documentation | 2 | `InfoTip`s are honest, but the five tool rows are hand-written paraphrases that have already drifted from `backend/mcp/server.go`. |
| **Total** | | **20/40** | **Needs work** |

## Design Specificity Verdict

**LLM assessment: the vessel is vectile; the spine is category-generic. Roughly 60/40.**

Genuinely vectile: `binds to 127.0.0.1 · nothing leaves this machine` states the product thesis as an interface fact; the 13.5px ledger rows, `divide-line/70` hairlines and right-aligned controls inherit the app's own voice; the tool names are real identifiers.

Category-interchangeable: status plate → enable → port → permission → tool catalog → per-client snippets is the canonical layout of every app that ships an MCP server. Nothing is derived from *this* product. There is no authored moment for the highest-stakes state change in the app — turning on a server that exposes an entire private library is a grey dot becoming a green dot.

Missed product character, ranked:
1. **The section never quantifies what it exposes.** "Share your library with AI tools" grants unbounded scope; `Status` already carries `collections`/`chunks` and already renders in the top status strip. One line in the plate would fix it.
2. **Local-first, but all-or-nothing.** No per-collection scoping in a product whose thesis is "your data, your terms".
3. **No destination.** The client picker said *what* to paste but never *where* — the only per-client fact that actually differs.

**Deterministic scan:** `detect.mjs --json` over `SettingsView.tsx` returned `[]`, exit 0 — 0 findings, both before and after the change. Cross-checked against `frontend/src`, which yields one real finding (`layout-transition` at `index.css:280`, TRUE positive, unrelated to this section) — so the clean result is not vacuous. Two coverage gaps were confirmed: the detector has no rule for the 34 `draft()!.` non-null assertions in this file, and its `overused-font` rule does not match the `@theme` token-declaration form (it misses `--font-sans` at `index.css:20`).

**Visual overlays:** injection succeeded and the overlay ran inside the page. Console reported 7 findings across the app: `cream / beige palette` (page ground `rgb(246,244,239)`), `layout property animation: transition: width`, `auto-scrolling marquee` (`.index-progress__fill--preparing`), `glowing shadow accents` (`#1f8a50`), `hairline border with wide shadow`, and `nested cards` ×3. **Three of the seven landed on this section**, and all three were the old `bg-surface/20` snippet rows reading as a card inside a card. The change in this pass removes two of them.

## Overall Impression

A well-built generic MCP settings page wearing vectile's typography. The status plate is the best object here and the only one that couldn't be lifted into another product. The single biggest opportunity is to stop treating this as server configuration and start treating it as the moment the user decides what another intelligence is allowed to see.

## What's Working

1. **The status plate is the best-composed object in the section** — live dot, plain-language `running`/`stopped`, mono URL with copy, hairline, then the promise line. It reads top-down as state → value → why it's safe, exactly the order a nervous user needs. `bg-paper-warm` is the one place the section earns real surface contrast.
2. **The permission copy is honest and the badge tracks real state.** "Allow write tools" names what happens; the `write` pill switches to `bg-amber-soft text-amber-deep` only when writes are actually permitted — the design system's amber-means-attention rule used correctly, not as decoration.
3. **It inherits the app's ledger rather than inventing one.** `Section` → `SubHeading` → `FieldList` → `Toggle`/`NumField` gives the same 13.5px rhythm as Model and Chunking, and the Port field appearing only when the server is enabled is correct progressive disclosure.

## Priority Issues

### [P0] The status plate and the enable toggle can display contradictory states
- **What**: The plate reads live saved state (`store.mcpStatus()`, ~line 1597); the switch directly below it edits the draft (`draft()!.mcp.enabled`). The plate sits *above* the switch.
- **Why it matters**: The user flips the switch, looks up, and reads "stopped" plus "Enable it below, then save settings." The single most consequential control in the app appears broken. This is the deepest emotional valley in the section and it is self-inflicted.
- **Fix**: Give the plate three draft-aware states instead of two — `stopped` (grey), `enabled, unsaved` (amber dot + `bg-amber-soft`, "Starts when you save"), `running` (green). Derive from `draft()!.mcp.enabled` + `running()` + `store.settingsDirty()`, and stop rendering the "Enable it below" fallback once the draft is already enabled.
- **Suggested command**: `$impeccable harden`

### [P1] A failed start is invisible in the section and produces a false instruction
- **What**: `MCPStatus` (`backend/mcp/service.go:17`) is `{Running, Port, URL}` — no error field. `StartServer` can fail with "port in use", surfaced only as a transient toast at `store.tsx:278`, fired immediately before a green "Settings saved" toast at `store.tsx:283` into the same stacked corner.
- **Why it matters**: The plate then says "Enable it below, then save settings" to a user who did exactly that, while the client panel below still offers a copyable URL nothing is listening on. The user's next move is to open their client and guess.
- **Fix**: Add `Error string` to `MCPStatus` (Go + `frontend/bindings/`), render it in the plate in `text-danger` (red = errors only), suppress the success toast when the start failed, and gate the connect panel on `running()` with an honest inline state.
- **Suggested command**: `$impeccable harden`

### [P1] The URL was truncated everywhere and both copy affordances failed accessibility
- **What**: `truncate` + mouse-only `title` on the plate URL and single-line snippets; no `focus-visible:` ring on either copy button while `Switch` has one; targets ≈22×22px and ≈21px tall, under WCAG 2.2's 24×24 minimum; no `aria-live` confirmation; `Snippet`'s `aria-label` never changed after a successful copy; a failed `copyText` was silently ignored. The payload text also could not be selected, because `body` sets `user-select: none` and only `input`/`textarea` opt back in.
- **Why it matters**: The primary artifact of the section — the URL — was the one thing a user could not fully read, read out, or hand-select. A screen-reader user got zero confirmation that the copy succeeded.
- **Fix**: ✅ Addressed in this pass — the payload is a `<pre>` with `overflow-x-auto` (scrolls rather than breaking tokens), `select-text` re-enables selection, the copy button is a labelled 24px+ control with a `focus-visible:` ring, and an `aria-live` `role="status"` region announces success. The plate's own URL and copy button are still truncated and still below target size.
- **Suggested command**: `$impeccable audit`

### [P2] "Make the text background lighter" treated a symptom, and the naive fix would have made it worse
- **What**: The snippet rows used `bg-surface/20` over the `bg-paper` card, which composites to ≈`#f7f6f2` — about a 1% luminance delta, i.e. invisible. Contrast was never the problem: `text-ink-soft` on that fill measured ≈8.9:1 (AAA). Pushing the fill to full white `surface` would have painted a white rectangle *inside* a bone card, inverting the app's elevation logic (white = card, bone = ground) and turning each snippet into the loudest object on the panel. The detector independently flagged exactly these rows as `nested cards`.
- **Why it matters**: The real defect was that the row was neither a well nor a code block — a ghost with a border. Lightening it as asked would have made the wrong read stronger.
- **Fix**: ✅ Addressed in this pass, honoring the request — the whole connect panel is now one `bg-surface` (white) surface with hairline separators and **no** inner box, so the text sits on the lightest available ground without creating a card inside a card. This is the one nested surface in the section, deliberately.
- **Suggested command**: `$impeccable layout`

### [P2] A seven-tab picker is the wrong control, and it still answers the wrong question
- **What**: The requested tab row answers "which product is this?" when the question that actually varies per client is "which file, and what extra step?". Measured on the real surface: seven tabs total 484px inside a 563px panel at a 748px window — it fits on one row only because the labels were tightened to 12px/`px-2`; at the original 12.5px/`px-2.5` the total was ≈550px and "Other" orphaned onto a second row. At the app's 980px max column the panel is ≈639px, so the row is inherently near the wrap threshold, and the "VS Code / GitHub Copilot" label the brief asked for adds ≈115px to that budget.
- **Why it matters**: Below `md` the Settings rail already collapses into a `rounded-full` chip row, so the app would ship two stacked horizontal selectors. Tabs also imply persistent, returnable state for content that is ~80% identical across clients — four of seven differ by a single key name.
- **Fix**: ✅ Shipped as asked, with the fixes that make it hold: underline tabs (visually distinct from the rail's pills), 12px labels so all seven fit one row, full ARIA tabs wiring (roving tabindex, arrow/Home/End keys, `aria-controls`), and — the part the brief was missing — a per-client **destination line** ("Add this to `.vscode/mcp.json`…") plus the extra step the client needs. Still open: a `Select` would be more robust at narrow widths, and the transport (`sse`) is still baked into the UI rather than being an orthogonal axis, so adding Streamable HTTP later means revisiting this matrix.
- **Suggested command**: `$impeccable shape`

### [P2] The section never says what it is exposing, and its tool list has already drifted
- **What**: "Share your library with AI tools" grants scope that is never quantified, even though `Status` carries `collections`/`sources`/`chunks` and renders them in the top strip. Separately, `MCP_TOOLS` is a hand-copied paraphrase that has diverged: the UI says `vectile_search` filters by "collection, source type, path, and date", while `backend/mcp/server.go` also advertises `top_k`, `sender`, `author` and `metadata_filter`.
- **Why it matters**: The user is asked to make a privacy decision with no statement of its size, and the catalog the AI is actually told about is not the catalog on screen.
- **Fix**: Add `4 collections · 7,029 chunks readable` (or `· writable`) to the status plate, and derive the tool catalog from one source of truth so it cannot drift.
- **Suggested command**: `$impeccable clarify`

## Persona Red Flags

**Jenna (first-timer, has never configured an MCP client)**
- She got a payload and no destination. Nothing said the config lives in `claude_desktop_config.json`, that the file may not exist, or that Claude must be **fully quit and relaunched** — the one thing she cannot infer is the one thing the UI hid. ✅ Now shown.
- Her first action appeared to do nothing: she flipped "Share your library with AI tools" and the panel directly above still read **stopped**. She will read this as broken. The correction — the sticky save bar — is pinned to the bottom of a long scrolling card, outside her focal area at the moment she needs it. **Still open (P0).**
- The line written for her, `binds to 127.0.0.1 · nothing leaves this machine`, is the smallest, weakest element on the panel: 11.5px `text-muted` in the aside voice, and rendered in browser-synthesized oblique because Plus Jakarta Sans is imported wght-only with no italic axis. The section sets protocol detail in its loudest visual language and the trust promise in its quietest — exactly backwards.
- She could not select the payload text to copy it by hand, and copy confirmation was a 14px icon swapping colourlessly for 1.6s. ✅ Both fixed.

**Marcus (power user, already runs four MCP servers)**
- He wants the URL and a shell command. `claude mcp add` was the second of three footnotes with no CLI grouping. ✅ It is now its own tab.
- The transport is what he cares about and the UI treated it as the whole world: "Any MCP SSE client", a `/sse` path baked into the URL, `--transport sse` in the only command on screen. Tabs re-encode that framing seven times; they do not fix it. **Still open.**
- His most likely failure is a port collision, and it is discovered last — the port field has no availability check and no "checking…" state, and the failure arrives as a red toast racing a green success toast. **Still open (P1).**
- He has no test-connection affordance and no way to see the real advertised tool schema, only a paraphrase.

## Minor Observations

1. `PlugIcon` appears twice in one section — the `Section` header and the "What your AI can do" `SubHeading`.
2. Tool names render in `text-indigo-deep` (green) but are not interactive; green is reserved for active/selectable/actionable, so this trains the wrong read.
3. The plate's internal `border-t` divider separates the URL from the "binds to…" line — one rule between two halves of one thought, and it is `aria-hidden`.
4. The `write` pill stacked three legibility taxes (10px, uppercase, wide tracking) plus mono on the smallest label in the app.
5. `copyText`'s fallback appends a textarea to `document.body` and removes it inside the `try`; if `execCommand` throws after the append, the textarea is orphaned. In a Wails webview `window.isSecureContext` is not guaranteed, so this path is live.
6. No scope or count is stated in the tool catalog header — nothing says whether five is all of them or whether the list changes with `allow_write`.
7. `.note` sets `font-style: italic` but the sans is imported without an italic axis, so every aside and empty state in the app renders as a synthesized oblique.
8. Documentation debt this change creates: `DESIGN.md` specifies the three-snippet arrangement verbatim and `README.md` promises "per-client setup directions"; both now describe what was replaced.

## Questions to Consider

1. If four of seven clients differ by a single key name, why is the **client** the axis of the UI instead of the **URL** being the artifact and the client being a footnote?
2. What is this section selling: an MCP server, or the promise that the AI can only see what **you chose**? If it is the second, why is there no scope control at all — and why is the only privacy statement on the panel set in the smallest, weakest type?
3. What does this section say when the library is empty — 0 collections, 0 chunks? Today it offers to share nothing, with no mention that there is nothing to share.
