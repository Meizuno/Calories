// Command claim attaches a pre-cutover profile to a local account.
//
// Before auth moved in-house, a profile was keyed by an external SSO user id.
// Those ids carry no email, and with the auth service gone nothing can resolve
// them, so migration 000004 parked them in profiles.legacy_user_id and left the
// profiles unowned. This tool hands one to a real account:
//
//	go run ./cmd/claim                                 # list unclaimed profiles
//	go run ./cmd/claim -profile 3 -email me@example.com # hand profile 3 over
//
// The target account must already exist (register in the app first). Its own
// empty profile is deleted in the process, since a user owns exactly one.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Meizuno/calories/config"
	"github.com/Meizuno/calories/internal/service"
	"github.com/Meizuno/calories/internal/store"
	"github.com/jackc/pgx/v5"
)

func main() {
	profileID := flag.Int64("profile", 0, "id of the unclaimed profile to hand over")
	email := flag.String("email", "", "email of the account that should own it")
	flag.Parse()

	cfg := config.Load()
	ctx := context.Background()
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer st.Close()

	profiles := service.NewProfiles(st.Queries)

	if *profileID == 0 || *email == "" {
		if err := list(ctx, profiles); err != nil {
			log.Fatalf("list: %v", err)
		}
		return
	}

	user, err := st.Queries.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(*email)))
	if errors.Is(err, pgx.ErrNoRows) {
		log.Fatalf("no account for %s — register it in the app first, then re-run", *email)
	}
	if err != nil {
		log.Fatalf("account: %v", err)
	}

	prof, err := profiles.Claim(ctx, *profileID, user.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		log.Fatalf("profile %d is not unclaimed (it may already belong to someone)", *profileID)
	}
	if err != nil {
		log.Fatalf("claim: %v", err)
	}
	fmt.Printf("profile %d (%q) now belongs to %s\n", prof.ID, prof.Name, user.Email)
}

func list(ctx context.Context, profiles *service.Profiles) error {
	rows, err := profiles.Unclaimed(ctx)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		fmt.Println("no unclaimed profiles — nothing to migrate")
		return nil
	}
	fmt.Println("unclaimed profiles (from the external-SSO era):")
	fmt.Printf("  %-6s %-24s %-38s %s\n", "ID", "NAME", "LEGACY USER ID", "CREATED")
	for _, p := range rows {
		name := p.Name
		if name == "" {
			name = "(unnamed)"
		}
		legacy := ""
		if p.LegacyUserID != nil {
			legacy = *p.LegacyUserID
		}
		fmt.Printf("  %-6d %-24s %-38s %s\n", p.ID, name, legacy, p.CreatedAt.Format("2006-01-02"))
	}
	fmt.Fprintln(os.Stderr, "\nhand one over with: go run ./cmd/claim -profile <id> -email <account email>")
	return nil
}
