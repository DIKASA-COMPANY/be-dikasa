package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// TestMigrationsApplyToPostgres catches SQL errors invisible to textual migration checks.
// It uses an isolated schema in a rolled-back transaction; existing data is untouched.
func TestMigrationsApplyToPostgres(t *testing.T) {
	dsn := os.Getenv("DIKASA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set DIKASA_TEST_DATABASE_URL to run PostgreSQL migration integration")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	schema := fmt.Sprintf("dikasa_migration_test_%d", time.Now().UnixNano())
	if _, err := tx.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec("SET LOCAL search_path TO " + schema + ", public"); err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob("*.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(string(content)); err != nil {
			t.Fatalf("apply %s: %v", path, err)
		}
	}
	var exists bool
	if err := tx.QueryRow("SELECT to_regclass('customers') IS NOT NULL AND to_regclass('transaction_items') IS NOT NULL").Scan(&exists); err != nil || !exists {
		t.Fatalf("final schema missing: %v", err)
	}
}
