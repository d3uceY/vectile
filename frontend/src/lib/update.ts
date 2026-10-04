/* Update check + external links. Modular on purpose: the badge, the startup
   check, and the dialog all read from here, so the GitHub URL and the
   "stable release only" rule live in one place. */

import { Browser } from "@wailsio/runtime";

export const REPO = "d3uceY/vectile";
export const HOME_URL = `https://github.com/${REPO}`;
/** Opens the latest release page directly, so the download prompt lands on
    the current version's assets instead of the whole releases list. */
export const DOWNLOAD_URL = `https://github.com/${REPO}/releases/latest`;

/** True when running inside the Wails desktop webview. The browser dev-stub
    also sets window._wails, so require a non-localhost origin too; that keeps
    the update check (and its dialog) out of the screenshot pipeline. */
export const isDesktop =
  typeof window !== "undefined" &&
  Boolean((window as unknown as { _wails?: unknown })._wails) &&
  !/^https?:\/\/(localhost|127\.0\.0\.1)/.test(window.location.origin);

/** Opens a URL in the system browser; falls back to a new tab (browser dev). */
export function openExternal(url: string): void {
  if (isDesktop) Browser.OpenURL(url);
  else window.open(url, "_blank", "noopener");
}

/** Parses "v1.2.3" → [1,2,3]. Returns null for anything with a suffix
    (beta, rc, alpha, ...); only clean stable releases count. */
export function parseVersion(v: string): [number, number, number] | null {
  const m = /^v?(\d+)\.(\d+)\.(\d+)$/.exec(v.trim());
  return m ? [Number(m[1]), Number(m[2]), Number(m[3])] : null;
}

/** True when `latest` is a stable release strictly newer than `current`. */
export function isNewer(latest: string, current: string): boolean {
  const a = parseVersion(latest);
  const b = parseVersion(current);
  if (!a || !b) return false;
  return a[0] > b[0] || (a[0] === b[0] && (a[1] > b[1] || (a[1] === b[1] && a[2] > b[2])));
}

/** Latest release tag_name, or null when none yet / unstable / offline. */
export async function fetchLatestRelease(): Promise<string | null> {
  try {
    const res = await fetch(`https://api.github.com/repos/${REPO}/releases/latest`, {
      headers: { Accept: "application/vnd.github+json" },
    });
    if (!res.ok) return null;
    const data = (await res.json()) as { tag_name?: string };
    return typeof data.tag_name === "string" ? data.tag_name : null;
  } catch {
    return null;
  }
}
