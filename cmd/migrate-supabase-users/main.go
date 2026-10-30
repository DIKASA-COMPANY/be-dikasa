// Command migrate-supabase-users imports active Dikasa accounts into Supabase Auth.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"be-dikasa/internal/auth"
	"be-dikasa/internal/config"
	"be-dikasa/internal/database"
	"be-dikasa/internal/shared/supabaseauth"
	"be-dikasa/internal/user"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	if err := config.LoadDotEnv(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("memuat .env: %w", err)
	}
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	db, err := database.OpenPostgres(databaseURL)
	if err != nil {
		return fmt.Errorf("inisialisasi PostgreSQL: %w", err)
	}
	defer func() { _ = database.ClosePostgres(db) }()
	provider, err := supabaseauth.NewClient(
		os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_PUBLISHABLE_KEY"), os.Getenv("SUPABASE_SECRET_KEY"),
	)
	if err != nil {
		return err
	}
	result, err := auth.NewMigrator(user.NewGORMRepository(db), provider).Migrate(ctx, 100)
	if err != nil {
		return err
	}
	log.Printf("migrasi Supabase selesai: imported=%d linked_existing=%d", result.Migrated, result.LinkedExisting)
	return nil
}
