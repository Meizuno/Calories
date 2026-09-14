package domain

import (
	"math"
	"testing"
)

func eq(t *testing.T, got, want Macros) {
	t.Helper()
	const tol = 1e-9
	if math.Abs(got.Kcal-want.Kcal) > tol || math.Abs(got.Carb-want.Carb) > tol ||
		math.Abs(got.Protein-want.Protein) > tol || math.Abs(got.Fat-want.Fat) > tol {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestScale(t *testing.T) {
	per100g := Macros{Kcal: 165, Carb: 0, Protein: 31, Fat: 3.6}

	t.Run("proportional to quantity", func(t *testing.T) {
		eq(t, Scale(per100g, 100, 150), Macros{Kcal: 247.5, Carb: 0, Protein: 46.5, Fat: 5.4})
	})

	t.Run("per-piece foods use basis 1", func(t *testing.T) {
		egg := Macros{Kcal: 78, Carb: 0.6, Protein: 6, Fat: 5}
		eq(t, Scale(egg, 1, 3), Macros{Kcal: 234, Carb: 1.8, Protein: 18, Fat: 15})
	})

	t.Run("zero quantity contributes nothing", func(t *testing.T) {
		eq(t, Scale(per100g, 100, 0), Macros{})
	})

	// A zero or negative basis would divide by zero and poison every downstream
	// total with Inf/NaN, so it must yield an empty contribution instead.
	t.Run("non-positive basis is not a division", func(t *testing.T) {
		for _, basis := range []float64{0, -100} {
			got := Scale(per100g, basis, 150)
			eq(t, got, Macros{})
		}
	})
}

func TestAdd(t *testing.T) {
	a := Macros{Kcal: 100, Carb: 10, Protein: 5, Fat: 2}
	b := Macros{Kcal: 50, Carb: 1.5, Protein: 20, Fat: 0.5}
	eq(t, a.Add(b), Macros{Kcal: 150, Carb: 11.5, Protein: 25, Fat: 2.5})

	t.Run("adding zero is identity", func(t *testing.T) {
		eq(t, a.Add(Macros{}), a)
	})
}

func TestRemaining(t *testing.T) {
	target := Macros{Kcal: 2400, Carb: 250, Protein: 180, Fat: 70}

	t.Run("subtracts per key", func(t *testing.T) {
		eaten := Macros{Kcal: 1624, Carb: 187.8, Protein: 120, Fat: 40}
		eq(t, Remaining(target, eaten), Macros{Kcal: 776, Carb: 62.2, Protein: 60, Fat: 30})
	})

	// Going over the goal is normal and the UI colours it differently, so the
	// value has to stay negative rather than clamp at zero.
	t.Run("goes negative when over the goal", func(t *testing.T) {
		over := Macros{Kcal: 2500, Carb: 260, Protein: 190, Fat: 80}
		eq(t, Remaining(target, over), Macros{Kcal: -100, Carb: -10, Protein: -10, Fat: -10})
	})
}

func TestRound(t *testing.T) {
	// kcal whole, grams to one decimal.
	got := Macros{Kcal: 247.49, Carb: 12.34, Protein: 46.55, Fat: 5.44}.Round()
	eq(t, got, Macros{Kcal: 247, Carb: 12.3, Protein: 46.6, Fat: 5.4})
}
