export interface Macros {
  kcal: number;
  carb: number;
  protein: number;
  fat: number;
}

export interface Entry {
  id: number;
  name: string;
  quantity: number;
  unit: string;
  kcal: number;
  carb: number;
  protein: number;
  fat: number;
}

export interface Meal {
  id: number;
  name: string;
  note: string;
  entries: Entry[];
  total: Macros;
}

export interface Day {
  date: string;
  target: Macros;
  eaten: Macros;
  remaining: Macros;
  meals: Meal[];
}

export interface Profile {
  publicId: string;
  name: string;
  shared: boolean;
  onboarded: boolean;
  goal: Macros;
}

// One day's summed macros inside a stats range. `date` is YYYY-MM-DD.
export interface DayTotal extends Macros {
  date: string;
}

// Per-day totals for a period plus the daily goal, from GET /api/stats.
// Only days with logged entries are present; the view fills the gaps.
export interface Stats {
  from: string;
  to: string;
  goal: Macros;
  days: DayTotal[];
}
