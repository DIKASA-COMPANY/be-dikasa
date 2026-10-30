package auth

import (
	"context"
	"testing"

	"be-dikasa/internal/shared/supabaseauth"
	"be-dikasa/internal/user"
)

type migrationRepository struct {
	users   []user.User
	bound   map[string]string
	queries int
}

func (repository *migrationRepository) ListUnboundSupabaseUsers(context.Context, int) ([]user.User, error) {
	repository.queries++
	if repository.queries > 1 {
		return nil, nil
	}
	return repository.users, nil
}
func (repository *migrationRepository) BindSupabaseUserID(_ context.Context, userID, supabaseUserID string) error {
	if repository.bound == nil {
		repository.bound = map[string]string{}
	}
	repository.bound[userID] = supabaseUserID
	return nil
}

func TestMigratorImportsUnboundAccountsAndPreservesUUID(t *testing.T) {
	email := "owner@example.com"
	repository := &migrationRepository{users: []user.User{{ID: "local-id", Email: &email, PasswordHash: "$2a$10$hash", IsActive: true}}}
	provider := &stubSupabaseProvider{}
	result, err := NewMigrator(repository, provider).Migrate(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.Migrated != 1 || provider.imported.ID != "local-id" || repository.bound["local-id"] != "local-id" {
		t.Fatalf("unexpected migration: result=%+v import=%+v bound=%v", result, provider.imported, repository.bound)
	}
}

func TestMigratorBindsExistingOAuthEmail(t *testing.T) {
	email := "owner@example.com"
	repository := &migrationRepository{users: []user.User{{ID: "local-id", Email: &email, PasswordHash: "$2a$10$hash", IsActive: true}}}
	provider := &stubSupabaseProvider{importErr: supabaseauth.ErrUserExists, findID: "oauth-id"}
	result, err := NewMigrator(repository, provider).Migrate(context.Background(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.LinkedExisting != 1 || repository.bound["local-id"] != "oauth-id" {
		t.Fatalf("unexpected OAuth binding: result=%+v bound=%v", result, repository.bound)
	}
}
