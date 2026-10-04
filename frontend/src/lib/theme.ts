/**
 * Theme preference: "system" follows the OS, "light" / "dark" pin it.
 * Stored in localStorage under `vectile.theme` and applied as a single
 * `data-theme="light|dark"` attribute on <html>, which the token block in
 * index.css keys off. Kept dependency-free so `index.html` can repeat the
 * same resolution inline before first paint (no flash of the wrong theme).
 */

export type ThemePref = "system" | "light" | "dark";
export type ResolvedTheme = "light" | "dark";

export const THEME_KEY = "vectile.theme";

const isPref = (v: unknown): v is ThemePref => v === "system" || v === "light" || v === "dark";

export function getThemePref(): ThemePref {
  try {
    const v = localStorage.getItem(THEME_KEY);
    return isPref(v) ? v : "system";
  } catch {
    return "system";
  }
}

export function prefersDark(): boolean {
  return window.matchMedia("(prefers-color-scheme: dark)").matches;
}

export function resolveTheme(pref: ThemePref): ResolvedTheme {
  if (pref === "system") return prefersDark() ? "dark" : "light";
  return pref;
}

export function applyTheme(pref: ThemePref): void {
  document.documentElement.setAttribute("data-theme", resolveTheme(pref));
}

export function setThemePref(pref: ThemePref): void {
  try {
    localStorage.setItem(THEME_KEY, pref);
  } catch {
    /* private mode / storage disabled: still apply for this session */
  }
  applyTheme(pref);
}
