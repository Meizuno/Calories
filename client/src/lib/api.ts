import type { Day, Profile, Stats } from "./types";
import { apiFetch, JSON_HEADERS } from "./http";

// Every call goes through apiFetch, which transparently refreshes an expired
// access token once and replays the request before giving up on the session.

export const api = {
  getDay: (date: string) => apiFetch<Day>(`/api/day?date=${date}`),

  // dates (YYYY-MM-DD) that have logged data — used to enable calendar days
  getDays: () => apiFetch<string[]>("/api/days"),

  // per-day macro totals for an inclusive [from, to] range (YYYY-MM-DD)
  getStats: (from: string, to: string) => apiFetch<Stats>(`/api/stats?from=${from}&to=${to}`),

  addMeal: (date: string, name: string) =>
    apiFetch<Day>("/api/meals", { method: "POST", headers: JSON_HEADERS, body: JSON.stringify({ date, name }) }),

  updateMeal: (date: string, id: number, name: string, note: string) =>
    apiFetch<Day>(`/api/meals/${id}`, { method: "PATCH", headers: JSON_HEADERS, body: JSON.stringify({ date, name, note }) }),

  deleteMeal: (date: string, id: number) => apiFetch<Day>(`/api/meals/${id}?date=${date}`, { method: "DELETE" }),

  // Duplicate a meal (entries and all) onto `toDate`. Resolves with THAT day,
  // not the one the meal came from.
  copyMeal: (id: number, toDate: string) =>
    apiFetch<Day>(`/api/meals/${id}/copy`, { method: "POST", headers: JSON_HEADERS, body: JSON.stringify({ date: toDate }) }),

  addEntry: (body: {
    date: string;
    mealId: number;
    name: string;
    quantity: number;
    unit: string;
    kcal: number;
    carb: number;
    protein: number;
    fat: number;
  }) => apiFetch<Day>("/api/entries", { method: "POST", headers: JSON_HEADERS, body: JSON.stringify(body) }),

  updateEntry: (
    date: string,
    id: number,
    body: { name: string; quantity: number; unit: string; kcal: number; carb: number; protein: number; fat: number },
  ) => apiFetch<Day>(`/api/entries/${id}`, { method: "PATCH", headers: JSON_HEADERS, body: JSON.stringify({ date, ...body }) }),

  deleteEntry: (date: string, id: number) => apiFetch<Day>(`/api/entries/${id}?date=${date}`, { method: "DELETE" }),

  getProfile: () => apiFetch<Profile>("/api/profile"),

  saveProfile: (body: { name: string; kcal: number; carb: number; protein: number; fat: number; shared: boolean }) =>
    apiFetch<Profile>("/api/profile", { method: "PUT", headers: JSON_HEADERS, body: JSON.stringify(body) }),

  // public, read-only (shared profiles)
  getShared: (uuid: string) => apiFetch<Profile>(`/api/shared/${uuid}`),
  getSharedDay: (uuid: string, date: string) => apiFetch<Day>(`/api/shared/${uuid}/day?date=${date}`),
  getSharedStats: (uuid: string, from: string, to: string) => apiFetch<Stats>(`/api/shared/${uuid}/stats?from=${from}&to=${to}`),
  getSharedDays: (uuid: string) => apiFetch<string[]>(`/api/shared/${uuid}/days`),
};
