// Command seed populates a dev user with a sample food catalog and a few days of
// meals + entries. LOCAL DEV tool — the Dockerfile builds only ./cmd/server, so
// this never ships in production. Run: `go run ./cmd/seed` (uses DATABASE_URL).
package main

import (
	"context"
	"log"
	"time"

	"github.com/Meizuno/calories/config"
	"github.com/Meizuno/calories/internal/service"
	"github.com/Meizuno/calories/internal/store"
)

type foodSpec struct {
	name, unit                   string
	basis, kcal, carb, prot, fat float64
}

type entrySpec struct {
	meal, food string
	qty        float64
}

const seedDays = 3

func main() {
	cfg := config.Load()
	if err := store.Migrate(cfg.DatabaseURL); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer st.Close()

	catalog := service.NewCatalog(st.Queries)
	diary := service.NewDiary(st.Queries)
	profiles := service.NewProfiles(st.Queries)

	// Everything hangs off a profile; ensure one exists for the dev user.
	prof, err := profiles.Ensure(ctx, cfg.DevUserID)
	if err != nil {
		log.Fatalf("profile: %v", err)
	}
	pid := prof.ID

	// The profile owns name + daily target — seed them and mark it onboarded so the
	// dev user skips the onboarding form.
	if _, err := profiles.Save(ctx, pid, "Dev", 2400, 250, 180, 70, false); err != nil {
		log.Printf("profile save: %v", err)
	}

	if existing, _ := catalog.List(ctx, pid); len(existing) > 0 {
		log.Printf("seed: profile %d (user %s) already has %d foods — nothing to do (use a fresh DB to reseed)", pid, cfg.DevUserID, len(existing))
		return
	}

	// Macros are PER BASIS (basis_amount of basis_unit): per 100 g, per 1 ks, …
	foods := []foodSpec{
		{"Ovesné vločky (suché)", "g", 100, 372, 60, 13, 7},
		{"Rýže basmati (vařená)", "g", 100, 130, 28, 2.7, 0.3},
		{"Těstoviny (vařené)", "g", 100, 158, 31, 6, 1},
		{"Brambory (vařené)", "g", 100, 87, 20, 2, 0.1},
		{"Chléb žitný", "g", 100, 250, 48, 8, 3},
		{"Kuřecí prsa (vařená)", "g", 100, 165, 0, 31, 3.6},
		{"Losos (pečený)", "g", 100, 208, 0, 20, 13},
		{"Vejce", "ks", 1, 78, 0.6, 6, 5},
		{"Tvaroh polotučný", "g", 100, 130, 3.5, 18, 5},
		{"Jogurt bílý", "g", 100, 60, 4, 4, 3},
		{"Sýr eidam", "g", 100, 330, 1, 25, 25},
		{"Mléko polotučné", "ml", 100, 48, 5, 3, 2},
		{"Máslo", "g", 100, 717, 0, 1, 81},
		{"Olivový olej", "ml", 100, 884, 0, 0, 100},
		{"Mandle", "g", 100, 579, 22, 21, 50},
		{"Banán", "ks", 1, 107, 27, 1, 0},
		{"Jablko", "ks", 1, 78, 21, 0.4, 0.3},
		{"Syrovátkový protein (jahoda)", "g", 30, 114, 2, 23, 2},
	}
	for _, f := range foods {
		if err := catalog.Create(ctx, pid, f.name, f.unit, f.basis, f.kcal, f.carb, f.prot, f.fat); err != nil {
			log.Printf("food %q: %v", f.name, err)
		}
	}

	all, _ := catalog.List(ctx, pid)
	foodID := make(map[string]int64, len(all))
	for _, f := range all {
		foodID[f.Name] = f.ID
	}

	mealNames := []string{"Snídaně", "Svačina", "Oběd", "Večeře"}
	entries := []entrySpec{
		{"Snídaně", "Ovesné vločky (suché)", 60},
		{"Snídaně", "Mléko polotučné", 200},
		{"Snídaně", "Banán", 1},
		{"Svačina", "Syrovátkový protein (jahoda)", 30},
		{"Svačina", "Jablko", 1},
		{"Oběd", "Rýže basmati (vařená)", 200},
		{"Oběd", "Kuřecí prsa (vařená)", 150},
		{"Oběd", "Olivový olej", 10},
		{"Večeře", "Tvaroh polotučný", 200},
		{"Večeře", "Chléb žitný", 60},
	}

	now := time.Now().UTC()
	base := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	for d := 0; d < seedDays; d++ {
		date := base.AddDate(0, 0, -d)
		for _, n := range mealNames {
			if err := diary.AddMeal(ctx, pid, date, n, ""); err != nil {
				log.Printf("meal %q (%s): %v", n, date.Format("2006-01-02"), err)
			}
		}
		dv, err := diary.GetDayView(ctx, pid, date)
		if err != nil {
			log.Printf("dayview %s: %v", date.Format("2006-01-02"), err)
			continue
		}
		mealID := map[string]int64{}
		for _, m := range dv.Meals {
			if _, ok := mealID[m.Meal.Name]; !ok {
				mealID[m.Meal.Name] = m.Meal.ID
			}
		}
		for _, e := range entries {
			mid, ok := mealID[e.meal]
			fid, ok2 := foodID[e.food]
			if !ok || !ok2 {
				continue
			}
			if err := diary.AddFoodEntry(ctx, pid, mid, fid, e.qty); err != nil {
				log.Printf("entry %s/%s (%s): %v", e.meal, e.food, date.Format("2006-01-02"), err)
			}
		}
	}

	log.Printf("seed: done for profile %d (user %s) — name Dev, goal 2400/250/180/70, %d foods, %d days × %d meals with entries",
		pid, cfg.DevUserID, len(foods), seedDays, len(mealNames))
}
