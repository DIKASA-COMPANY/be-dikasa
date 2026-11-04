package auth

import (
	"context"
	"errors"
	"fmt"

	"be-dikasa/internal/shared/supabaseauth"
	"be-dikasa/internal/user"
)

// MigrationRepository lists active accounts that are not yet bound to Supabase Auth.
type MigrationRepository interface {
	ListUnboundSupabaseUsers(ctx context.Context, limit int) ([]user.User, error)
	BindSupabaseUserID(ctx context.Context, userID, supabaseUserID string) error
}

// MigrationResult summarizes one complete account migration run.
type MigrationResult struct {
	Migrated       int
	LinkedExisting int
}

// Migrator imports active PostgreSQL accounts into Supabase Auth in bounded batches.
type Migrator struct {
	users    MigrationRepository
	provider SupabaseProvider
}

// NewMigrator creates the one-off Supabase account migrator.
func NewMigrator(users MigrationRepository, provider SupabaseProvider) *Migrator {
	return &Migrator{users: users, provider: provider}
}

// Migrate preserves local UUIDs and bcrypt hashes, or links an existing exact-email OAuth user.
func (migrator *Migrator) Migrate(ctx context.Context, batchSize int) (MigrationResult, error) {
	if batchSize < 1 || batchSize > 1000 {
		return MigrationResult{}, fmt.Errorf("batch size must be between 1 and 1000")
	}
	var result MigrationResult
	for {
		accounts, err := migrator.users.ListUnboundSupabaseUsers(ctx, batchSize)
		if err != nil {
			return result, err
		}
		if len(accounts) == 0 {
			return result, nil
		}
		for index := range accounts {
			account := &accounts[index]
			if account.Email == nil {
				return result, fmt.Errorf("user %s has no email", account.ID)
			}
			supabaseUserID := account.ID
			err = migrator.provider.ImportUser(ctx, supabaseauth.UserImport{
				ID: account.ID, Email: *account.Email, PasswordHash: account.PasswordHash, EmailConfirmed: true,
			})
			if errors.Is(err, supabaseauth.ErrUserExists) {
				supabaseUserID, err = migrator.provider.FindUserByEmail(ctx, *account.Email)
				if err == nil {
					err = migrator.provider.UpdateUserCredentials(ctx, supabaseUserID, *account.Email, account.PasswordHash)
				}
				if err == nil {
					result.LinkedExisting++
				}
			} else if err == nil {
				result.Migrated++
			}
			if err != nil {
				return result, fmt.Errorf("migrate user %s: %w", account.ID, err)
			}
			if err = migrator.users.BindSupabaseUserID(ctx, account.ID, supabaseUserID); err != nil {
				return result, fmt.Errorf("bind user %s: %w", account.ID, err)
			}
		}
	}
}
