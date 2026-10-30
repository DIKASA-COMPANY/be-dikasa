package auth

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"be-dikasa/internal/shared/supabaseauth"
	"be-dikasa/internal/user"
)

type stubUserRepository struct {
	user    *user.User
	err     error
	boundID string
}

func (stub *stubUserRepository) FindByIdentifier(context.Context, string) (*user.User, error) {
	return stub.user, stub.err
}
func (stub *stubUserRepository) FindByID(context.Context, string) (*user.User, error) {
	return stub.user, stub.err
}
func (stub *stubUserRepository) BindSupabaseUserID(_ context.Context, _, supabaseUserID string) error {
	stub.boundID = supabaseUserID
	return stub.err
}

type stubSupabaseProvider struct {
	sessions     []supabaseauth.Session
	signInErrs   []error
	imported     supabaseauth.UserImport
	updatedID    string
	updatedEmail string
	updatedHash  string
	email        string
	findID       string
	importErr    error
}

func (stub *stubSupabaseProvider) SignIn(_ context.Context, email, _ string) (supabaseauth.Session, error) {
	stub.email = email
	var session supabaseauth.Session
	if len(stub.sessions) > 0 {
		session = stub.sessions[0]
		stub.sessions = stub.sessions[1:]
	}
	var err error
	if len(stub.signInErrs) > 0 {
		err = stub.signInErrs[0]
		stub.signInErrs = stub.signInErrs[1:]
	}
	return session, err
}
func (stub *stubSupabaseProvider) ImportUser(_ context.Context, value supabaseauth.UserImport) error {
	stub.imported = value
	return stub.importErr
}
func (stub *stubSupabaseProvider) FindUserByEmail(context.Context, string) (string, error) {
	if stub.findID == "" {
		return "", supabaseauth.ErrUserNotFound
	}
	return stub.findID, nil
}
func (stub *stubSupabaseProvider) UpdateUserCredentials(_ context.Context, id, email, hash string) error {
	stub.updatedID, stub.updatedEmail, stub.updatedHash = id, email, hash
	return nil
}

func activeAccount(t *testing.T) *user.User {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("rahasia123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	email := "owner@example.com"
	return &user.User{ID: "11111111-1111-4111-8111-111111111111", Email: &email, PasswordHash: string(hash), IsActive: true}
}

func TestServiceAuthenticatesLocalAccountThenReturnsSupabaseSession(t *testing.T) {
	repository := &stubUserRepository{user: activeAccount(t)}
	provider := &stubSupabaseProvider{sessions: []supabaseauth.Session{{AccessToken: "supabase-access", UserID: repository.user.ID}}}
	service := NewService(repository, provider)

	session, err := service.SignIn(context.Background(), "kasir", "rahasia123")
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if session.AccessToken != "supabase-access" || session.SupabaseUserID != repository.user.ID || provider.email != "owner@example.com" || repository.boundID != repository.user.ID {
		t.Fatalf("unexpected session/provider state: %+v email=%q bound=%q", session, provider.email, repository.boundID)
	}
}

func TestServiceImportsExistingBcryptAccountOnFirstSupabaseLogin(t *testing.T) {
	repository := &stubUserRepository{user: activeAccount(t)}
	provider := &stubSupabaseProvider{
		sessions:   []supabaseauth.Session{{}, {AccessToken: "access", UserID: repository.user.ID}},
		signInErrs: []error{supabaseauth.ErrInvalidCredentials, nil},
	}
	service := NewService(repository, provider)

	session, err := service.SignIn(context.Background(), "owner@example.com", "rahasia123")
	if err != nil {
		t.Fatal(err)
	}
	if session.AccessToken != "access" || provider.imported.ID != repository.user.ID || provider.imported.PasswordHash != repository.user.PasswordHash || !provider.imported.EmailConfirmed {
		t.Fatalf("account was not imported correctly: session=%+v import=%+v", session, provider.imported)
	}
}

func TestServiceResynchronizesLinkedSupabasePasswordHash(t *testing.T) {
	account := activeAccount(t)
	linked := "22222222-2222-4222-8222-222222222222"
	account.SupabaseUserID = &linked
	repository := &stubUserRepository{user: account}
	provider := &stubSupabaseProvider{
		sessions:   []supabaseauth.Session{{}, {AccessToken: "access", UserID: linked}},
		signInErrs: []error{supabaseauth.ErrInvalidCredentials, nil},
	}
	service := NewService(repository, provider)

	if _, err := service.SignIn(context.Background(), "owner@example.com", "rahasia123"); err != nil {
		t.Fatal(err)
	}
	if provider.updatedID != linked || provider.updatedEmail != *account.Email || provider.updatedHash != account.PasswordHash {
		t.Fatalf("credentials were not synchronized: id=%q email=%q hash=%q", provider.updatedID, provider.updatedEmail, provider.updatedHash)
	}
}

func TestServiceLinksPreexistingOAuthUserWhenEmailAlreadyExists(t *testing.T) {
	account := activeAccount(t)
	repository := &stubUserRepository{user: account}
	oauthID := "33333333-3333-4333-8333-333333333333"
	provider := &stubSupabaseProvider{
		sessions: []supabaseauth.Session{{}, {AccessToken: "access", UserID: oauthID}}, signInErrs: []error{supabaseauth.ErrInvalidCredentials, nil},
		importErr: supabaseauth.ErrUserExists, findID: oauthID,
	}
	if _, err := NewService(repository, provider).SignIn(context.Background(), "owner@example.com", "rahasia123"); err != nil {
		t.Fatal(err)
	}
	if provider.updatedID != oauthID || repository.boundID != oauthID {
		t.Fatalf("existing OAuth identity was not linked: updated=%q bound=%q", provider.updatedID, repository.boundID)
	}
}

func TestServiceRejectsInvalidCredentialsWithoutCallingSupabase(t *testing.T) {
	tests := []struct {
		name       string
		repository *stubUserRepository
		password   string
	}{
		{"unknown user", &stubUserRepository{err: user.ErrNotFound}, "rahasia123"},
		{"wrong password", &stubUserRepository{user: activeAccount(t)}, "salah123"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := &stubSupabaseProvider{}
			_, err := NewService(test.repository, provider).SignIn(context.Background(), "identifier", test.password)
			if !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("expected ErrInvalidCredentials, got %v", err)
			}
			if provider.email != "" {
				t.Fatalf("Supabase was called for rejected local credentials: %q", provider.email)
			}
		})
	}
}

func TestServiceRejectsInactiveUserAndMapsProviderFailure(t *testing.T) {
	account := activeAccount(t)
	account.IsActive = false
	if _, err := NewService(&stubUserRepository{user: account}, &stubSupabaseProvider{}).SignIn(context.Background(), "kasir", "rahasia123"); !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("expected ErrUserDisabled, got %v", err)
	}
	account.IsActive = true
	provider := &stubSupabaseProvider{signInErrs: []error{supabaseauth.ErrUnavailable}}
	if _, err := NewService(&stubUserRepository{user: account}, provider).SignIn(context.Background(), "kasir", "rahasia123"); !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("expected ErrProviderUnavailable, got %v", err)
	}
}
