import type { Day, Food, Profile } from "./types";
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

  addMeal: (date: string, name: string) =>
    fetch("/api/meals", { method: "POST", headers: JSON_HEADERS, body: JSON.stringify({ date, name }) }).then((r) => asJSON<Day>(r)),

  updateMeal: (date: string, id: number, name: string) =>
    fetch(`/api/meals/${id}`, { method: "PATCH", headers: JSON_HEADERS, body: JSON.stringify({ date, name }) }).then((r) => asJSON<Day>(r)),

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

  listFoods: () => fetch("/api/foods").then((r) => asJSON<Food[]>(r)),

  createFood: (body: Partial<Food>) =>
    fetch("/api/foods", { method: "POST", headers: JSON_HEADERS, body: JSON.stringify(body) }).then((r) => asJSON<Food[]>(r)),

  deleteFood: (id: number) =>
    fetch(`/api/foods/${id}`, { method: "DELETE" }).then((r) => asJSON<Food[]>(r)),

  getProfile: () => fetch("/api/profile").then((r) => asJSON<Profile>(r)),

  saveProfile: (body: { name: string; kcal: number; carb: number; protein: number; fat: number; shared: boolean }) =>
    fetch("/api/profile", { method: "PUT", headers: JSON_HEADERS, body: JSON.stringify(body) }).then((r) => asJSON<Profile>(r)),

  // public, read-only (shared profiles)
  getShared: (uuid: string) => fetch(`/api/shared/${uuid}`).then((r) => asJSON<Profile>(r)),
  getSharedDay: (uuid: string, date: string) => fetch(`/api/shared/${uuid}/day?date=${date}`).then((r) => asJSON<Day>(r)),
};
