import type { Day, Profile, Stats } from "./types";
import { redirectToLogin } from "./session";

async function asJSON<T>(res: Response): Promise<T> {
  if (res.status === 401) {
    redirectToLogin();
    throw new Error("unauthorized");
  }
  if (!res.ok) throw new Error(await res.text());
  return res.json() as Promise<T>;
}

const JSON_HEADERS = { "Content-Type": "application/json" };

export const api = {
  getDay: (date: string) => fetch(`/api/day?date=${date}`).then((r) => asJSON<Day>(r)),

  // dates (YYYY-MM-DD) that have logged data — used to enable calendar days
  getDays: () => fetch("/api/days").then((r) => asJSON<string[]>(r)),

  // per-day macro totals for an inclusive [from, to] range (YYYY-MM-DD)
  getStats: (from: string, to: string) =>
    fetch(`/api/stats?from=${from}&to=${to}`).then((r) => asJSON<Stats>(r)),

  addMeal: (date: string, name: string) =>
    fetch("/api/meals", { method: "POST", headers: JSON_HEADERS, body: JSON.stringify({ date, name }) }).then((r) => asJSON<Day>(r)),

  updateMeal: (date: string, id: number, name: string, note: string) =>
    fetch(`/api/meals/${id}`, { method: "PATCH", headers: JSON_HEADERS, body: JSON.stringify({ date, name, note }) }).then((r) => asJSON<Day>(r)),

  deleteMeal: (date: string, id: number) =>
    fetch(`/api/meals/${id}?date=${date}`, { method: "DELETE" }).then((r) => asJSON<Day>(r)),

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
  }) => fetch("/api/entries", { method: "POST", headers: JSON_HEADERS, body: JSON.stringify(body) }).then((r) => asJSON<Day>(r)),

  updateEntry: (
    date: string,
    id: number,
    body: { name: string; quantity: number; unit: string; kcal: number; carb: number; protein: number; fat: number },
  ) => fetch(`/api/entries/${id}`, { method: "PATCH", headers: JSON_HEADERS, body: JSON.stringify({ date, ...body }) }).then((r) => asJSON<Day>(r)),

  deleteEntry: (date: string, id: number) =>
    fetch(`/api/entries/${id}?date=${date}`, { method: "DELETE" }).then((r) => asJSON<Day>(r)),

  getProfile: () => fetch("/api/profile").then((r) => asJSON<Profile>(r)),

  saveProfile: (body: { name: string; kcal: number; carb: number; protein: number; fat: number; shared: boolean }) =>
    fetch("/api/profile", { method: "PUT", headers: JSON_HEADERS, body: JSON.stringify(body) }).then((r) => asJSON<Profile>(r)),

  // public, read-only (shared profiles)
  getShared: (uuid: string) => fetch(`/api/shared/${uuid}`).then((r) => asJSON<Profile>(r)),
  getSharedDay: (uuid: string, date: string) => fetch(`/api/shared/${uuid}/day?date=${date}`).then((r) => asJSON<Day>(r)),
  getSharedStats: (uuid: string, from: string, to: string) =>
    fetch(`/api/shared/${uuid}/stats?from=${from}&to=${to}`).then((r) => asJSON<Stats>(r)),
  getSharedDays: (uuid: string) => fetch(`/api/shared/${uuid}/days`).then((r) => asJSON<string[]>(r)),
};
