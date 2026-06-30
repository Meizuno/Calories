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
