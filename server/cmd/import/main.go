// Command import loads the markdown food diary into the app. LOCAL DEV tool — the
// Dockerfile builds only ./cmd/server, so this never ships. It parses the
// weeks/days/meals/food-tables format and writes meals + ad-hoc entries for a
// profile; the per-row macros are stored as-is (they are the consumed portion).
//
//	go run ./cmd/import --dry-run notes.md                 # parse + print, no DB
//	go run ./cmd/import notes.md                           # import into DEV_USER_ID
//	go run ./cmd/import --user <sso-id> --name Yurii notes.md
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Meizuno/calories/config"
	"github.com/Meizuno/calories/internal/service"
	"github.com/Meizuno/calories/internal/store"
	"github.com/Meizuno/calories/internal/store/db"
)

type pEntry struct {
	name, unit                    string
	qty, kcal, carb, protein, fat float64
}
type pMeal struct {
	name    string
	note    string // blockquote (>) lines under the meal heading
	entries []pEntry
}
type pDay struct {
	date  time.Time
	meals []pMeal
}

var (
	dayRe = regexp.MustCompile(`^##\s+(\d{1,2})\.\s*(\d{1,2})\.\s*(\d{4})`)
	numRe = regexp.MustCompile(`^(\d+(?:[.,]\d+)?)(?:\s*/\s*(\d+))?`)
	fracs = map[rune]float64{'½': 0.5, '⅓': 1.0 / 3, '⅔': 2.0 / 3, '¼': 0.25, '¾': 0.75, '⅕': 0.2, '⅖': 0.4, '⅗': 0.6, '⅘': 0.8, '⅙': 1.0 / 6, '⅛': 0.125}
)

func main() {
	dryRun := flag.Bool("dry-run", false, "parse and print only; no DB connection or writes")
	user := flag.String("user", "", "external (SSO) user id to import into; defaults to DEV_USER_ID")
	name := flag.String("name", "Yurii", "profile display name")
	force := flag.Bool("force", false, "import a day even if it already has meals (additive)")
	flag.Parse()

	path := flag.Arg(0)
	if path == "" {
		log.Fatal("usage: import [--dry-run] [--user id] [--name n] [--force] <notes.md>")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("read %s: %v", path, err)
	}

	days, goal, haveGoal := parse(string(data))

	totalMeals, totalEntries := 0, 0
	for _, d := range days {
		totalMeals += len(d.meals)
		for _, m := range d.meals {
			totalEntries += len(m.entries)
		}
	}
	fmt.Printf("parsed: %d days, %d meals, %d entries\n", len(days), totalMeals, totalEntries)
	if haveGoal {
		fmt.Printf("goal (from Cíl): %.0f kcal / %.0f S / %.0f B / %.0f T\n", goal[0], goal[1], goal[2], goal[3])
	}
	if len(days) > 0 {
		d := days[0]
		fmt.Printf("\nsample — %s:\n", d.date.Format("2006-01-02"))
		for _, m := range d.meals {
			fmt.Printf("  %s\n", m.name)
			if m.note != "" {
				fmt.Printf("    > %s\n", strings.ReplaceAll(m.note, "\n", " / "))
			}
			for _, e := range m.entries {
				fmt.Printf("    - %-46s %6.2f %-16s | %4.0f kcal  %3.0f/%3.0f/%3.0f\n",
					e.name, e.qty, e.unit, e.kcal, e.carb, e.protein, e.fat)
			}
		}
	}

	if *dryRun {
		fmt.Println("\n[dry-run] no DB writes.")
		return
	}

	cfg := config.Load()
	uid := *user
	if uid == "" {
		uid = cfg.DevUserID
	}
	if uid == "" {
		log.Fatal("no target user: set DEV_USER_ID in .env or pass --user")
	}

	if err := store.Migrate(cfg.DatabaseURL); err != nil {
		log.Fatalf("migrations: %v", err)
	}
	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer st.Close()

	profiles := service.NewProfiles(st.Queries)
	diary := service.NewDiary(st.Queries)

	prof, err := profiles.Ensure(ctx, uid)
	if err != nil {
		log.Fatalf("profile: %v", err)
	}
	pid := prof.ID

	g := [4]float64{2300, 253, 169, 68}
	if haveGoal {
		g = goal
	}
	if _, err := profiles.Save(ctx, pid, *name, g[0], g[1], g[2], g[3], false); err != nil {
		log.Printf("profile save: %v", err)
	}

	var impDays, impMeals, impEntries, skipped int
	for _, d := range days {
		existing, _ := st.Queries.ListMealsForDay(ctx, db.ListMealsForDayParams{ProfileID: pid, Date: d.date})
		if len(existing) > 0 && !*force {
			skipped++
			log.Printf("skip %s — already has %d meals (use --force to add)", d.date.Format("2006-01-02"), len(existing))
			continue
		}
		impDays++
		for _, m := range d.meals {
			pos, err := st.Queries.MaxMealPosition(ctx, db.MaxMealPositionParams{ProfileID: pid, Date: d.date})
			if err != nil {
				log.Printf("pos %s/%s: %v", d.date.Format("2006-01-02"), m.name, err)
				continue
			}
			var note *string
			if m.note != "" {
				n := m.note
				note = &n
			}
			meal, err := st.Queries.CreateMeal(ctx, db.CreateMealParams{ProfileID: pid, Date: d.date, Name: m.name, Position: pos + 1, Note: note})
			if err != nil {
				log.Printf("meal %s/%s: %v", d.date.Format("2006-01-02"), m.name, err)
				continue
			}
			impMeals++
			for _, e := range m.entries {
				if err := diary.AddAdhocEntry(ctx, pid, meal.ID, e.name, e.unit, e.qty, e.kcal, e.carb, e.protein, e.fat); err != nil {
					log.Printf("entry %s/%s/%s: %v", d.date.Format("2006-01-02"), m.name, e.name, err)
					continue
				}
				impEntries++
			}
		}
	}
	log.Printf("done: profile %d (user %s) — imported %d days, %d meals, %d entries; skipped %d days",
		pid, uid, impDays, impMeals, impEntries, skipped)
}

// parse walks the markdown line by line. A `### heading` becomes a meal only when
// a food table (header contains "Potravina") follows; the daily "Souhrn dne" table
// (no "Potravina") is mined once for the "Cíl" goal row and otherwise ignored. A
// blockquote (`>`) line under a meal heading becomes that meal's comment/note.
func parse(text string) ([]pDay, [4]float64, bool) {
	lines := strings.Split(text, "\n")
	var days []pDay
	var goal [4]float64
	haveGoal := false
	ci := -1
	curMeal := ""
	curMealIdx := -1  // index in days[ci].meals of the meal being annotated
	pendingNote := "" // > lines seen before the meal's table

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if m := dayRe.FindStringSubmatch(line); m != nil {
			d, _ := strconv.Atoi(m[1])
			mo, _ := strconv.Atoi(m[2])
			y, _ := strconv.Atoi(m[3])
			days = append(days, pDay{date: time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC)})
			ci = len(days) - 1
			curMeal, curMealIdx, pendingNote = "", -1, ""
			continue
		}
		if strings.HasPrefix(line, "### ") {
			curMeal = cleanHeading(strings.TrimPrefix(line, "###"))
			curMealIdx, pendingNote = -1, ""
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		// A blockquote line is a comment for the current meal (works before or after
		// its food table).
		if strings.HasPrefix(line, ">") {
			t := strings.TrimSpace(strings.TrimPrefix(line, ">"))
			if t != "" && ci >= 0 {
				if curMealIdx >= 0 {
					if days[ci].meals[curMealIdx].note != "" {
						days[ci].meals[curMealIdx].note += "\n"
					}
					days[ci].meals[curMealIdx].note += t
				} else {
					if pendingNote != "" {
						pendingNote += "\n"
					}
					pendingNote += t
				}
			}
			continue
		}
		if strings.HasPrefix(line, "|") && strings.Contains(strings.ToLower(line), "kcal") {
			isMeal := strings.Contains(line, "Potravina")
			var rows [][]string
			j := i + 1
			for j < len(lines) {
				l := strings.TrimSpace(lines[j])
				if !strings.HasPrefix(l, "|") {
					break
				}
				cells := splitRow(l)
				j++
				if !isSeparator(cells) {
					rows = append(rows, cells)
				}
			}
			i = j - 1

			if isMeal {
				if ci < 0 || curMeal == "" {
					continue
				}
				meal := pMeal{name: curMeal, note: pendingNote}
				for _, c := range rows {
					if len(c) < 6 {
						continue
					}
					nm := stripBold(c[0])
					if nm == "" || strings.EqualFold(nm, "Celkem") {
						continue
					}
					qty, unit := parseQty(c[1])
					meal.entries = append(meal.entries, pEntry{
						name: nm, unit: unit, qty: qty,
						kcal: num(c[2]), carb: num(c[3]), protein: num(c[4]), fat: num(c[5]),
					})
				}
				if len(meal.entries) > 0 {
					days[ci].meals = append(days[ci].meals, meal)
					curMealIdx = len(days[ci].meals) - 1
					pendingNote = ""
				}
			} else if !haveGoal {
				for _, c := range rows {
					if len(c) >= 5 && strings.EqualFold(stripBold(c[0]), "Cíl") {
						goal = [4]float64{num(c[1]), num(c[2]), num(c[3]), num(c[4])}
						haveGoal = true
						break
					}
				}
			}
		}
	}
	return days, goal, haveGoal
}

// parseQty pulls a leading amount out of the free-text "Množství" (e.g. "40 g",
// "~10 g", "2 ks (~240 g)", "½ ks", "1,5 ks", "7/8 ks", "~1 porce"); the rest is
// the unit label. Macros are absolute per row, so the amount is purely cosmetic.
func parseQty(s string) (float64, string) {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "~"))
	if s == "" {
		return 1, "porce"
	}
	r := []rune(s)
	if v, ok := fracs[r[0]]; ok {
		return v, fallbackUnit(strings.TrimSpace(string(r[1:])))
	}
	m := numRe.FindStringSubmatch(s)
	if m == nil {
		return 1, s // no number — keep the whole label (e.g. "trochu", "porce")
	}
	val, _ := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", "."), 64)
	if m[2] != "" {
		if den, _ := strconv.ParseFloat(m[2], 64); den != 0 {
			val /= den
		}
	}
	unit := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s[len(m[0]):]), "~"))
	return val, fallbackUnit(unit)
}

func fallbackUnit(u string) string {
	if u == "" {
		return "porce"
	}
	return u
}

func cleanHeading(s string) string {
	if idx := strings.Index(s, "<!--"); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(strings.ReplaceAll(s, "*", ""))
}

func splitRow(line string) []string {
	line = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "|"), "|")
	parts := strings.Split(line, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func isSeparator(cells []string) bool {
	for _, c := range cells {
		if c != "" && strings.Trim(c, ":-") != "" {
			return false
		}
	}
	return true
}

func stripBold(s string) string { return strings.TrimSpace(strings.ReplaceAll(s, "*", "")) }

func num(s string) float64 {
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "−", "-") // unicode minus → ascii
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", ".")
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
