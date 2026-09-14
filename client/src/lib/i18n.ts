import { reactive } from "vue";
import cs from "../locales/cs";
import en from "../locales/en";

// A deliberately small i18n layer instead of a library: nothing here needs
// plural rules (every count in the UI is followed by a non-inflecting unit —
// "g", "kcal"), so lookup + interpolation is the whole job. Dates and numbers
// go through Intl, which the browser already ships.

export const MESSAGES = { cs, en };
export type Locale = keyof typeof MESSAGES;

export const LOCALE_NAMES: Record<Locale, string> = { cs: "Čeština", en: "English" };

// Flag icons, registered locally in lib/flags.ts (no CDN fetch). The British
// flag stands in for English — flags belong to countries, not languages, so
// this is a convention rather than a fact.
export const LOCALE_FLAGS: Record<Locale, string> = { cs: "i-locale-cs", en: "i-locale-en" };

const COOKIE = "locale";
const ONE_YEAR = 60 * 60 * 24 * 365;

function isLocale(v: string | null | undefined): v is Locale {
  return !!v && v in MESSAGES;
}

function readCookie(): Locale | null {
  const hit = document.cookie.split("; ").find((c) => c.startsWith(`${COOKIE}=`));
  const value = hit?.slice(COOKIE.length + 1);
  return isLocale(value) ? value : null;
}

// Not HttpOnly — the page itself has to read this to pick a language. It holds
// no secret, and being a cookie (rather than localStorage) means the server can
// read it too if anything ever needs to render in the right language.
function writeCookie(locale: Locale) {
  document.cookie = `${COOKIE}=${locale}; Path=/; Max-Age=${ONE_YEAR}; SameSite=Lax`;
}

/** Saved choice, else the browser's preference, else English. */
function detect(): Locale {
  const saved = readCookie();
  if (saved) return saved;
  for (const tag of navigator.languages ?? [navigator.language]) {
    const base = tag.toLowerCase().split("-")[0];
    if (isLocale(base)) return base;
  }
  return "en";
}

const state = reactive({ locale: detect() });

export function currentLocale(): Locale {
  return state.locale;
}

export function setLocale(locale: Locale) {
  state.locale = locale;
  writeCookie(locale);
  document.documentElement.lang = locale;
}

/** Apply the detected locale on boot (keeps <html lang> honest for a11y). */
export function initLocale() {
  document.documentElement.lang = state.locale;
}

function lookup(locale: Locale, key: string): string | undefined {
  let node: unknown = MESSAGES[locale];
  for (const part of key.split(".")) {
    if (typeof node !== "object" || node === null) return undefined;
    node = (node as Record<string, unknown>)[part];
  }
  return typeof node === "string" ? node : undefined;
}

/**
 * Translate a dotted key, filling {placeholders}. Reading state.locale here is
 * what makes every component using t() re-render on a language switch.
 *
 * A missing key falls back to English and finally to the key itself, so a gap
 * shows up as visible text rather than a blank space.
 */
export function t(key: string, vars?: Record<string, string | number>): string {
  const raw = lookup(state.locale, key) ?? lookup("en", key) ?? key;
  if (!vars) return raw;
  return raw.replace(/\{(\w+)\}/g, (whole, name: string) => (name in vars ? String(vars[name]) : whole));
}

// ── dates & numbers ─────────────────────────────────────────────────────────

const intlTag: Record<Locale, string> = { cs: "cs-CZ", en: "en-GB" };

/** Parse a YYYY-MM-DD as UTC midnight — the app stores plain dates, not instants. */
function asUTCDate(iso: string): Date {
  return new Date(`${iso}T00:00:00Z`);
}

export function formatDate(iso: string, opts: Intl.DateTimeFormatOptions): string {
  return new Intl.DateTimeFormat(intlTag[state.locale], { timeZone: "UTC", ...opts }).format(asUTCDate(iso));
}

/** Short weekday ("po" / "Mon") for chart axes and the day strip. */
export function weekdayShort(iso: string): string {
  return formatDate(iso, { weekday: "short" });
}

/** Long-form day heading, e.g. "pondělí 14. září" / "Monday 14 September". */
export function formatDayLong(iso: string): string {
  return formatDate(iso, { weekday: "long", day: "numeric", month: "long" });
}

export function formatMonth(iso: string): string {
  return formatDate(iso, { month: "long", year: "numeric" });
}
