// Package domain holds the core entities and pure rules — no I/O, no DB.
package domain

import "math"

// Macros is the four tracked values: kcal + carbs (Sach.) + protein (Bílk.) + fat (Tuky).
type Macros struct {
	Kcal    float64
	Carb    float64
	Protein float64
	Fat     float64
}

// Scale converts a per-basis macro source to a logged quantity.
// For piece/portion foods basisAmount is 1 and quantity is the count.
func Scale(src Macros, basisAmount, quantity float64) Macros {
	if basisAmount <= 0 {
		return Macros{}
	}
	f := quantity / basisAmount
	return Macros{
		Kcal:    src.Kcal * f,
		Carb:    src.Carb * f,
		Protein: src.Protein * f,
		Fat:     src.Fat * f,
	}
}

// Add returns the element-wise sum.
func (m Macros) Add(o Macros) Macros {
	return Macros{m.Kcal + o.Kcal, m.Carb + o.Carb, m.Protein + o.Protein, m.Fat + o.Fat}
}

// Remaining is target − eaten, per key (can go negative).
func Remaining(target, eaten Macros) Macros {
	return Macros{
		Kcal:    target.Kcal - eaten.Kcal,
		Carb:    target.Carb - eaten.Carb,
		Protein: target.Protein - eaten.Protein,
		Fat:     target.Fat - eaten.Fat,
	}
}

// Round rounds for display: kcal whole, grams to 1 decimal.
func (m Macros) Round() Macros {
	r1 := func(f float64) float64 { return math.Round(f*10) / 10 }
	return Macros{math.Round(m.Kcal), r1(m.Carb), r1(m.Protein), r1(m.Fat)}
}
