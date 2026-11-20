export type ThemePref = "dark" | "light" | "system";
export type DensityPref = "comfortable" | "compact";
export type LayoutPref = "grid" | "list";

export interface Prefs {
  theme: ThemePref;
  accent: string;
  density: DensityPref;
  layout: LayoutPref;
  pinned: string[];
  recents: string[];
  showDocker: boolean;
  immichBackground: boolean;
}

const KEY = "sdash:prefs:v1";
const RECENT_LIMIT = 8;

export const ACCENTS = ["lime", "emerald", "cyan", "sky", "amber", "orange", "rose"] as const;

export function defaultPrefs(seed?: {
  theme?: ThemePref;
  accent?: string;
  density?: DensityPref;
}): Prefs {
  return {
    theme: seed?.theme ?? "dark",
    accent: ACCENTS.find((accent) => accent === seed?.accent) ?? "lime",
    density: seed?.density ?? "comfortable",
    layout: "grid",
    pinned: [],
    recents: [],
    showDocker: true,
    immichBackground: false,
  };
}

type Listener = (message: string) => void;
let onSaveError: Listener = () => undefined;

/** Register a UI callback for storage failures so they are never silent. */
export function setPrefsErrorListener(listener: Listener): void {
  onSaveError = listener;
}

/** Validates an untrusted preferences document (e.g. from the server sync). */
export function sanitizePrefs(raw: unknown, fallback: Prefs): Prefs {
  if (raw === null || typeof raw !== "object") return fallback;
  const parsed = raw as Partial<Prefs>;
  const strings = (value: unknown): string[] =>
    Array.isArray(value) ? value.filter((v): v is string => typeof v === "string") : [];
  return {
    theme: parsed.theme === "dark" || parsed.theme === "light" || parsed.theme === "system" ? parsed.theme : fallback.theme,
    accent: ACCENTS.find((accent) => accent === parsed.accent) ?? fallback.accent,
    density: parsed.density === "compact" || parsed.density === "comfortable" ? parsed.density : fallback.density,
    layout: parsed.layout === "list" || parsed.layout === "grid" ? parsed.layout : fallback.layout,
    pinned: strings(parsed.pinned),
    recents: strings(parsed.recents),
    showDocker: typeof parsed.showDocker === "boolean" ? parsed.showDocker : fallback.showDocker,
    immichBackground:
      typeof parsed.immichBackground === "boolean" ? parsed.immichBackground : fallback.immichBackground,
  };
}

export function loadPrefs(seed?: Parameters<typeof defaultPrefs>[0]): Prefs {
  const fallback = defaultPrefs(seed);
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return fallback;
    const parsed: unknown = JSON.parse(raw);
    return sanitizePrefs(parsed, fallback);
  } catch {
    return fallback;
  }
}

export function savePrefs(prefs: Prefs): void {
  try {
    localStorage.setItem(KEY, JSON.stringify(prefs));
  } catch {
    onSaveError("Preferences could not be saved in this browser (storage unavailable).");
  }
}

export function pushRecent(recents: string[], key: string): string[] {
  return [key, ...recents.filter((k) => k !== key)].slice(0, RECENT_LIMIT);
}

export function resolveTheme(theme: ThemePref): "dark" | "light" {
  if (theme === "system") {
    return window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
  }
  return theme;
}

export function applyTheme(prefs: Prefs): void {
  const root = document.documentElement;
  const resolved = resolveTheme(prefs.theme);
  root.dataset.theme = resolved;
  root.dataset.accent = prefs.accent;
  root.dataset.density = prefs.density;
  document
    .querySelector('meta[name="theme-color"]')
    ?.setAttribute("content", resolved === "light" ? "#e9e9e6" : "#101012");
}

export function isMacPlatform(): boolean {
  if (typeof navigator === "undefined") return true;
  const platform = navigator.platform ?? "";
  return /Mac|iPhone|iPad/.test(platform) || navigator.userAgent.includes("Mac OS");
}

export function searchShortcutLabel(): string {
  return isMacPlatform() ? "⌘K" : "Ctrl K";
}
