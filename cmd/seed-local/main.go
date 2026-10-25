// Command seed-local explicitly creates a demo account on a loopback PostgreSQL database.
package main

import (
	"errors"
	"log"
	"net/url"
	"os"

	"be-dikasa/internal/config"
	"be-dikasa/internal/database"
	"be-dikasa/internal/devseed"
)

func main() {
	if err := seed(); err != nil {
		log.Fatal(err)
	}
	log.Print("Local seed finished; existing admin accounts were left unchanged.")
}

// seed requires an explicit local auth mode and loopback URL before touching the database.
func seed() error {
	if err := config.LoadDotEnv(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	if err := validateSeedTarget(cfg.AuthMode, cfg.DatabaseURL); err != nil {
		return err
	}
	db, err := database.OpenPostgres(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer database.ClosePostgres(db)
	password := os.Getenv("LOCAL_SEED_PASSWORD")
	if password == "" {
		password = "dikasa123"
	}
	return devseed.Run(db, password)
}

// validateSeedTarget rejects connection overrides as well as non-loopback hosts.
// pgx query options can override the URL authority, so local seeding accepts only
// sslmode and connect_timeout; other options must never redirect this command.
func validateSeedTarget(mode, dsn string) error {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return err
	}
	if mode != "local" || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") ||
		(parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" && parsed.Hostname() != "::1") {
		return errors.New("seed-local requires AUTH_MODE=local and a loopback PostgreSQL URL")
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return err
	}
	for key := range query {
		if key != "sslmode" && key != "connect_timeout" {
			return errors.New("seed-local URL may only set sslmode and connect_timeout query options")
		}
	}
	return nil
}
