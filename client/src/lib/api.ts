import type { Day, Food, Profile, Stats } from "./types";
import { apiFetch, JSON_HEADERS } from "./http";

// Every call goes through apiFetch, which transparently refreshes an expired
// access token once and replays the request before giving up on the session.
//
// The API answers with an object naming what it returns — {"day": …},
// {"foods": …} — so a response can gain a field without breaking a caller that
// only reads the one it knows. This module unwraps that envelope, so the views
// keep seeing plain Day / Stats / Food values.

const V1 = "/api/v1";

const post = (body: unknown): RequestInit => ({ method: "POST", headers: JSON_HEADERS, body: JSON.stringify(body) });
const patch = (body: unknown): RequestInit => ({ method: "PATCH", headers: JSON_HEADERS, body: JSON.stringify(body) });
const del: RequestInit = { method: "DELETE" };

const unwrap = <T>(key: string, p: Promise<Record<string, T>>): Promise<T> => p.then((r) => r[key]);
/** Most mutations answer with the day they changed. */
const day = (path: string, init?: RequestInit) => unwrap<Day>("day", apiFetch(path, init));

export const api = {
  getDay: (date: string) => day(`${V1}/day?date=${date}`),

  // dates (YYYY-MM-DD) that have logged data — used to enable calendar days
  getDays: () => unwrap<string[]>("days", apiFetch(`${V1}/days`)),

  // per-day macro totals for an inclusive [from, to] range (YYYY-MM-DD)
  getStats: (from: string, to: string) => unwrap<Stats>("stats", apiFetch(`${V1}/stats?from=${from}&to=${to}`)),

  // A meal may be created complete with its entries; the diary sends none and
  // fills them in afterwards.
  addMeal: (date: string, name: string) => day(`${V1}/meals`, post({ date, name })),

  updateMeal: (date: string, id: number, name: string, note: string) =>
    day(`${V1}/meals/${id}`, patch({ date, name, note })),

  deleteMeal: (date: string, id: number) => day(`${V1}/meals/${id}?date=${date}`, del),

  // Duplicate a meal (entries and all) onto `toDate`. Resolves with THAT day,
  // not the one the meal came from.
  copyMeal: (id: number, toDate: string) => day(`${V1}/meals/${id}/copy`, post({ date: toDate })),

  // The meal is an address rather than a body field: an item is created against
  // the meal it belongs to.
  addEntry: (
    date: string,
    mealId: number,
    body: { name: string; quantity: number; unit: string; kcal: number; carb: number; protein: number; fat: number },
  ) => day(`${V1}/meals/${mealId}/entries`, post({ date, ...body })),

  updateEntry: (
    date: string,
    id: number,
    body: { name: string; quantity: number; unit: string; kcal: number; carb: number; protein: number; fat: number },
  ) => day(`${V1}/entries/${id}`, patch({ date, ...body })),

  deleteEntry: (date: string, id: number) => day(`${V1}/entries/${id}?date=${date}`, del),

  // Foods the app has remembered from what has been logged. Small enough to
  // fetch whole and filter in the view, so typing a name needs no round-trip.
  getFoods: () => unwrap<Food[]>("foods", apiFetch(`${V1}/foods`)),

  // Responds with the refreshed list.
  forgetFood: (id: number) => unwrap<Food[]>("foods", apiFetch(`${V1}/foods/${id}`, del)),

  getProfile: () => unwrap<Profile>("profile", apiFetch(`${V1}/profile`)),

  saveProfile: (body: { name: string; kcal: number; carb: number; protein: number; fat: number; shared: boolean }) =>
    unwrap<Profile>(
      "profile",
      apiFetch(`${V1}/profile`, { method: "PUT", headers: JSON_HEADERS, body: JSON.stringify(body) }),
    ),

  // Public, read-only view of a profile that opted into sharing. These are the
  // same endpoints the owner reads, addressed by the profile's public id
  // instead of by a session.
  getShared: (uuid: string) => unwrap<Profile>("profile", apiFetch(`${V1}/shared/${uuid}`)),
  getSharedDay: (uuid: string, date: string) => day(`${V1}/shared/${uuid}/day?date=${date}`),
  getSharedStats: (uuid: string, from: string, to: string) =>
    unwrap<Stats>("stats", apiFetch(`${V1}/shared/${uuid}/stats?from=${from}&to=${to}`)),
  getSharedDays: (uuid: string) => unwrap<string[]>("days", apiFetch(`${V1}/shared/${uuid}/days`)),
};
