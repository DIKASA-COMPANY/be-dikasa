package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"be-dikasa/internal/user"
)

type stubUserRepository struct {
	user *user.User
	err  error
}

type stubCustomTokenClient struct {
	token string
	err   error
	uid   string
}

func (stub *stubCustomTokenClient) CustomToken(_ context.Context, uid string) (string, error) {
	stub.uid = uid
	return stub.token, stub.err
}

func (stub stubUserRepository) FindByIdentifier(context.Context, string) (*user.User, error) {
	return stub.user, stub.err
}

func (stub stubUserRepository) FindByID(context.Context, string) (*user.User, error) {
	return stub.user, stub.err
}

type stubTokenIssuer struct {
	session Session
	err     error
	uid     string
}

func (stub *stubTokenIssuer) Issue(_ context.Context, uid string) (Session, error) {
	stub.uid = uid
	return stub.session, stub.err
}

func TestServiceAuthenticatesPostgresUserThenIssuesFirebaseToken(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("rahasia123"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	issuer := &stubTokenIssuer{session: Session{IDToken: "firebase-id-token"}}
	service := NewService(stubUserRepository{user: &user.User{
		ID: "postgres-user-id", PasswordHash: string(hash), IsActive: true,
	}}, issuer)

	session, err := service.SignIn(context.Background(), "kasir", "rahasia123")

	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if session.IDToken != "firebase-id-token" || issuer.uid != "postgres-user-id" {
		t.Fatalf("unexpected session or UID: %+v, %q", session, issuer.uid)
	}
}

func TestServiceRejectsInvalidCredentialsWithoutLeakingUserExistence(t *testing.T) {
	tests := []struct {
		name       string
		repository stubUserRepository
		password   string
	}{
		{"unknown user", stubUserRepository{err: user.ErrNotFound}, "rahasia123"},
		{"wrong password", stubUserRepository{user: &user.User{PasswordHash: "$2a$04$invalid", IsActive: true}}, "salah123"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(test.repository, &stubTokenIssuer{})
			_, err := service.SignIn(context.Background(), "identifier", test.password)
			if !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("expected ErrInvalidCredentials, got %v", err)
			}
		})
	}
}

func TestServiceRejectsInactiveUser(t *testing.T) {
	service := NewService(stubUserRepository{user: &user.User{IsActive: false}}, &stubTokenIssuer{})

	_, err := service.SignIn(context.Background(), "kasir", "rahasia123")

	if !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("expected ErrUserDisabled, got %v", err)
	}
}

func TestServiceMapsTokenIssuerFailure(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("rahasia123"), bcrypt.MinCost)
	service := NewService(
		stubUserRepository{user: &user.User{ID: "uid", PasswordHash: string(hash), IsActive: true}},
		&stubTokenIssuer{err: errors.New("firebase unavailable")},
	)

	_, err := service.SignIn(context.Background(), "kasir", "rahasia123")

	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("expected ErrProviderUnavailable, got %v", err)
	}
}

func TestFirebaseTokenIssuerGeneratesAndExchangesCustomToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("key"); got != "web-api-key" {
			t.Fatalf("expected web API key, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"idToken":"firebase-id-token",
			"refreshToken":"firebase-refresh-token",
			"expiresIn":"3600"
		}`))
	}))
	defer server.Close()

	customTokenClient := &stubCustomTokenClient{token: "firebase-custom-token"}
	issuer := NewFirebaseTokenIssuer(customTokenClient, "web-api-key", WithTokenExchangeURL(server.URL))

	session, err := issuer.Issue(context.Background(), "postgres-user-id")

	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	if customTokenClient.uid != "postgres-user-id" {
		t.Fatalf("expected PostgreSQL UID, got %q", customTokenClient.uid)
	}
	if session.IDToken != "firebase-id-token" || session.RefreshToken != "firebase-refresh-token" || session.ExpiresIn != 3600 {
		t.Fatalf("unexpected session: %+v", session)
	}
}

func TestFirebaseTokenIssuerFailsWhenFirebaseCannotCreateToken(t *testing.T) {
	issuer := NewFirebaseTokenIssuer(&stubCustomTokenClient{err: errors.New("firebase failure")}, "web-api-key")

	_, err := issuer.Issue(context.Background(), "postgres-user-id")

	if !errors.Is(err, ErrProviderUnavailable) {
		t.Fatalf("expected ErrProviderUnavailable, got %v", err)
	}
}
