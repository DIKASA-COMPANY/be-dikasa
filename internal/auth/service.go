package auth

import (
	"context"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"be-dikasa/internal/shared/supabaseauth"
	"be-dikasa/internal/user"
)

const dummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

var (
	// ErrInvalidCredentials indicates that the submitted local credentials are invalid.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrUserDisabled indicates that the Dikasa account is inactive.
	ErrUserDisabled = errors.New("user disabled")
	// ErrProviderUnavailable indicates that Supabase Auth could not issue a session.
	ErrProviderUnavailable = errors.New("provider unavailable")
)

// SignInProvider defines the login use case used by the HTTP handler.
type SignInProvider interface {
	SignIn(ctx context.Context, identifier, password string) (Session, error)
}

// SupabaseProvider is the Auth surface required for login and rolling user migration.
type SupabaseProvider interface {
	SignIn(ctx context.Context, email, password string) (supabaseauth.Session, error)
	ImportUser(ctx context.Context, value supabaseauth.UserImport) error
	UpdateUserCredentials(ctx context.Context, userID, email, passwordHash string) error
	FindUserByEmail(ctx context.Context, email string) (string, error)
}

// Service validates the existing Dikasa credential before issuing a Supabase session.
type Service struct {
	users    user.Repository
	provider SupabaseProvider
}

// NewService creates an authentication service backed by Supabase Auth.
func NewService(users user.Repository, provider SupabaseProvider) *Service {
	return &Service{users: users, provider: provider}
}

// SignIn validates the local bcrypt credential and lazily imports it into Supabase Auth.
func (service *Service) SignIn(ctx context.Context, identifier, password string) (Session, error) {
	account, err := service.users.FindByIdentifier(ctx, identifier)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			_ = bcrypt.CompareHashAndPassword([]byte(dummyPasswordHash), []byte(password))
			return Session{}, ErrInvalidCredentials
		}
		return Session{}, err
	}
	if !account.IsActive {
		return Session{}, ErrUserDisabled
	}
	if err = bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(password)); err != nil {
		return Session{}, ErrInvalidCredentials
	}
	if account.Email == nil || strings.TrimSpace(*account.Email) == "" {
		return Session{}, ErrInvalidCredentials
	}

	providerSession, err := service.provider.SignIn(ctx, *account.Email, password)
	if errors.Is(err, supabaseauth.ErrInvalidCredentials) {
		if migrationErr := service.synchronizeAccount(ctx, account); migrationErr != nil {
			return Session{}, ErrProviderUnavailable
		}
		providerSession, err = service.provider.SignIn(ctx, *account.Email, password)
	}
	if err != nil || providerSession.AccessToken == "" || providerSession.UserID == "" {
		return Session{}, ErrProviderUnavailable
	}
	if account.SupabaseUserID != nil && *account.SupabaseUserID != providerSession.UserID {
		return Session{}, ErrProviderUnavailable
	}
	if identities, ok := service.users.(interface {
		BindSupabaseUserID(context.Context, string, string) error
	}); ok {
		if err = identities.BindSupabaseUserID(ctx, account.ID, providerSession.UserID); err != nil {
			return Session{}, err
		}
	}
	return Session{
		AccessToken: providerSession.AccessToken, RefreshToken: providerSession.RefreshToken,
		ExpiresIn: providerSession.ExpiresIn, SupabaseUserID: providerSession.UserID,
	}, nil
}

// synchronizeAccount preserves a stable UUID and bcrypt hash during rolling migration.
func (service *Service) synchronizeAccount(ctx context.Context, account *user.User) error {
	if account.SupabaseUserID != nil && *account.SupabaseUserID != "" {
		return service.provider.UpdateUserCredentials(ctx, *account.SupabaseUserID, *account.Email, account.PasswordHash)
	}
	err := service.provider.ImportUser(ctx, supabaseauth.UserImport{
		ID: account.ID, Email: *account.Email, PasswordHash: account.PasswordHash, EmailConfirmed: true,
	})
	if errors.Is(err, supabaseauth.ErrUserExists) {
		existingID, findErr := service.provider.FindUserByEmail(ctx, *account.Email)
		if findErr != nil {
			return findErr
		}
		return service.provider.UpdateUserCredentials(ctx, existingID, *account.Email, account.PasswordHash)
	}
	return err
}
