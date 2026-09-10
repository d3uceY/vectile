import { createEffect, createSignal, Show, type JSX } from "solid-js";
import "./nav-icons.css";

/* ============================================================
   Animated two-state nav icons.
   Used ONLY by the primary sidebar and the settings rail/chips so
   the shared primitives in icons.tsx stay untouched elsewhere.

   Each icon keeps its resting glyph until its tab/section becomes
   active, then transforms into a bespoke "open" glyph and holds it
   while active. Parts are tagged with CSS classes (see nav-icons.css):
     .mo / .mof - appear when open (staggered draw / pop)
     .mv        - rest-only, fades out when open
     .ms        - persistent parts that slide (--tx/--ty)
   A one-shot .morph-echo ring replays on each real activation (never
   on first paint), for the small "it happened" beat.
   ============================================================ */

type NavIconProps = {
  size?: number;
  strokeWidth?: number;
  class?: string;
  /** true while the owning tab/section is the active one */
  active?: boolean;
};

/** Replay a one-shot echo each time `active` turns on (skipping first paint). */
function useEcho(active: boolean | undefined) {
  const [echo, setEcho] = createSignal(false);
  let boot = true;
  let was = false;
  createEffect(() => {
    const on = !!active;
    if (boot) {
      boot = false;
      was = on;
      return;
    }
    if (on && !was) setEcho(true);
    else if (!on && was) setEcho(false);
    was = on;
  });
  return echo;
}

function NavSvg(props: NavIconProps & { open?: boolean; children: JSX.Element }) {
  const size = () => props.size ?? 17;
  const sw = () => props.strokeWidth ?? 1.75;
  return (
    <svg
      class={`navico ${props.open ? "navico--open" : ""} ${props.class ?? ""}`}
      width={size()}
      height={size()}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width={sw()}
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      {props.children}
    </svg>
  );
}

/* ---------------- primary sidebar ---------------- */

/** Search: magnifier locks onto a "found" mark (dot + radar ring). */
export function SearchNavIcon(props: NavIconProps) {
  const echo = useEcho(props.active);
  return (
    <NavSvg {...props} open={props.active}>
      <circle cx="11" cy="11" r="7" />
      <path d="M20.1 20.1 16.4 16.4" />
      <circle class="mof" cx="11" cy="11" r="2" />
      <circle class="mo mo-2" pathLength={1} cx="11" cy="11" r="4.3" />
      <Show when={echo()}>
        <g class="morph-echo">
          <circle pathLength={1} cx="11" cy="11" r="4.6" />
        </g>
      </Show>
    </NavSvg>
  );
}

/** Library: closed book opens into a read spread. */
export function LibraryNavIcon(props: NavIconProps) {
  return (
    <NavSvg {...props} open={props.active}>
      <g class="mv">
        <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
        <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2Z" />
        <path d="M9 7h7" />
        <path d="M9 11h7" />
      </g>
      <path class="mo" pathLength={1} d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z" />
      <path class="mo mo-2" pathLength={1} d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z" />
      <path class="mo mo-3" pathLength={1} d="M4.5 12h4.6" />
      <path class="mo mo-4" pathLength={1} d="M14.9 12h4.6" />
    </NavSvg>
  );
}

/** Browse: a 2x2 grid of collections expands into the nested
    collection -> source -> chunk list you actually navigate. */
export function BrowseNavIcon(props: NavIconProps) {
  return (
    <NavSvg {...props} open={props.active}>
      <g class="mv">
        <rect x="3" y="3" width="7" height="7" rx="1.5" />
        <rect x="14" y="3" width="7" height="7" rx="1.5" />
        <rect x="3" y="14" width="7" height="7" rx="1.5" />
        <rect x="14" y="14" width="7" height="7" rx="1.5" />
      </g>
      <rect class="mo" pathLength={1} x="4" y="5" width="15" height="3" rx="1.5" />
      <rect class="mo mo-2" pathLength={1} x="7.5" y="11" width="11.5" height="3" rx="1.5" />
      <rect class="mo mo-3" pathLength={1} x="11" y="17" width="8" height="3" rx="1.5" />
    </NavSvg>
  );
}

/** Index: the ingest arrow resolves into an indexed document. */
export function IndexNavIcon(props: NavIconProps) {
  return (
    <NavSvg {...props} open={props.active}>
      <path class="mv" d="M12 3.5v11" />
      <path class="mv" d="m7 10.5 5 5 5-5" />
      <path d="M4 19.2h16" />
      <rect class="mo" pathLength={1} x="6.5" y="4" width="11" height="12.6" rx="1.9" />
      <path class="mo mo-2" pathLength={1} d="M9.2 8.6h5.6" />
      <path class="mo mo-3" pathLength={1} d="M9.2 11.6h5.6" />
      <path class="mo mo-4" pathLength={1} d="m11.2 14.9 1.2 1.2 2.5-2.8" />
    </NavSvg>
  );
}

/** Settings: the two knobs glide to new values when settings open. */
export function SettingsNavIcon(props: NavIconProps) {
  return (
    <NavSvg {...props} open={props.active}>
      <path d="M4 7h9" />
      <path d="M17 7h3" />
      <circle class="ms" style={{ "--tx": "-8.5px" }} cx="15.5" cy="7" r="2" />
      <path d="M4 17h3" />
      <path d="M11 17h9" />
      <circle class="ms" style={{ "--tx": "7px" }} cx="9.5" cy="17" r="2" />
    </NavSvg>
  );
}

/* ---------------- settings rail ---------------- */

/** Model: the outline bolt powers up into a solid, live mark. */
export function ModelNavIcon(props: NavIconProps) {
  const zap = "M13 2 4.5 13.5H11L9.5 22 19 10h-6.5Z";
  return (
    <NavSvg {...props} open={props.active}>
      <path class="mv" d={zap} />
      <path class="mof" d={zap} />
    </NavSvg>
  );
}

/** Chunking: a single document separates into its chunks. */
export function ChunkNavIcon(props: NavIconProps) {
  return (
    <NavSvg {...props} open={props.active}>
      <g class="mv">
        <path d="M14 3H7a1.5 1.5 0 0 0-1.5 1.5v15A1.5 1.5 0 0 0 7 21h10a1.5 1.5 0 0 0 1.5-1.5V8Z" />
        <path d="M14 3v5h5" />
      </g>
      <rect class="mo" pathLength={1} x="6" y="5.6" width="13" height="3" rx="1.4" />
      <rect class="mo mo-2" pathLength={1} x="6" y="11.2" width="13" height="3" rx="1.4" />
      <rect class="mo mo-3" pathLength={1} x="6" y="16.8" width="13" height="3" rx="1.4" />
    </NavSvg>
  );
}

/** Sources: the closed folder opens to show what's inside. */
export function SourcesNavIcon(props: NavIconProps) {
  return (
    <NavSvg {...props} open={props.active}>
      <path class="mv" d="M3 7a2 2 0 0 1 2-2h4l2 2.5h8a2 2 0 0 1 2 2V17a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z" />
      <path class="mo" pathLength={1} d="M3 8.5V7.5a2 2 0 0 1 2-2h4l2 2.5h8a2 2 0 0 1 2 2v1" />
      <path
        class="mo mo-2"
        pathLength={1}
        d="M3.5 10.5 5.2 18a2 2 0 0 0 1.96 1.6h9.7a2 2 0 0 0 1.95-1.6l1.7-7.5a.8.8 0 0 0-.78-.98H4.28a.8.8 0 0 0-.78.98Z"
      />
    </NavSvg>
  );
}

/** Connect: the plug seats and its tip lights up as the link opens. */
export function ConnectNavIcon(props: NavIconProps) {
  const echo = useEcho(props.active);
  return (
    <NavSvg {...props} open={props.active}>
      <path d="M9 2v6" />
      <path d="M15 2v6" />
      <path d="M6 8h12v3a6 6 0 0 1-12 0V8Z" />
      <path d="M12 17v5" />
      <circle class="mof" cx="12" cy="21.2" r="1.7" />
      <Show when={echo()}>
        <g class="morph-echo">
          <circle pathLength={1} cx="12" cy="21.2" r="2.2" />
        </g>
      </Show>
    </NavSvg>
  );
}

/** Cache: the stored layers separate as the section opens. */
export function CacheNavIcon(props: NavIconProps) {
  return (
    <NavSvg {...props} open={props.active}>
      <ellipse class="ms" style={{ "--ty": "3.4px" }} cx="12" cy="6.6" rx="7.2" ry="2.8" />
      <path class="ms" d="M4.8 11.4c0 1.5 3.2 2.8 7.2 2.8s7.2-1.3 7.2-2.8" />
      <path class="ms" style={{ "--ty": "-3.4px" }} d="M4.8 15.4c0 1.5 3.2 2.8 7.2 2.8s7.2-1.3 7.2-2.8" />
    </NavSvg>
  );
}
