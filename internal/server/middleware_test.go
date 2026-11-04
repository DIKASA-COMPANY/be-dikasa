package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"be-dikasa/internal/shared/supabaseauth"
	"be-dikasa/internal/user"
)

type stubTokenVerifier struct {
	token *VerifiedToken
	err   error
}

func (stub stubTokenVerifier) VerifyAccessToken(context.Context, string) (*VerifiedToken, error) {
	return stub.token, stub.err
}

func TestAuthMiddlewareRejectsMissingAndInvalidToken(t *testing.T) {
	tests := []struct {
		name          string
		authorization string
		verifier      stubTokenVerifier
	}{
		{"missing", "", stubTokenVerifier{}},
		{"malformed", "token", stubTokenVerifier{}},
		{"invalid", "Bearer invalid", stubTokenVerifier{err: errors.New("invalid token")}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.Use(AuthMiddleware(test.verifier, func(string) bool { return false }))
			router.GET("/dikasa/v1/health", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			request := httptest.NewRequest(http.MethodGet, "/dikasa/v1/health", nil)
			request.Header.Set("Authorization", test.authorization)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, response.Code)
			}
		})
	}
}

func TestAuthMiddlewareAddsPostgresUserIDToContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(AuthMiddleware(stubTokenVerifier{token: &VerifiedToken{UserID: "postgres-user-id"}}, nil))
	router.GET("/dikasa/v1/health", func(c *gin.Context) {
		identity, ok := IdentityFromContext(c.Request.Context())
		if !ok || identity.UserID != "postgres-user-id" {
			t.Fatalf("unexpected identity: %+v, %v", identity, ok)
		}
		userID, ok := user.AuthenticatedUserIDFromContext(c.Request.Context())
		if !ok || userID != "postgres-user-id" {
			t.Fatalf("unexpected user context: %q, %v", userID, ok)
		}
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "/dikasa/v1/health", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, response.Code)
	}
}

type stubSupabaseIdentityProvider struct {
	identity supabaseauth.Identity
	err      error
}

func (stub stubSupabaseIdentityProvider) VerifyAccessToken(context.Context, string) (supabaseauth.Identity, error) {
	return stub.identity, stub.err
}

type identityUserRepository struct {
	account *user.User
	boundID string
}

func (repository *identityUserRepository) FindByIdentifier(context.Context, string) (*user.User, error) {
	if repository.account == nil {
		return nil, user.ErrNotFound
	}
	return repository.account, nil
}
func (repository *identityUserRepository) FindByID(context.Context, string) (*user.User, error) {
	return nil, user.ErrNotFound
}
func (repository *identityUserRepository) FindBySupabaseUserID(context.Context, string) (*user.User, error) {
	return nil, user.ErrNotFound
}
func (repository *identityUserRepository) BindSupabaseUserID(_ context.Context, _, supabaseUserID string) error {
	repository.boundID = supabaseUserID
	return nil
}

func TestSupabaseTokenVerifierBindsConfirmedOAuthEmailToLocalAccount(t *testing.T) {
	email := "owner@example.com"
	repository := &identityUserRepository{account: &user.User{ID: "postgres-user-id", Email: &email, IsActive: true}}
	verifier := NewSupabaseTokenVerifier(stubSupabaseIdentityProvider{identity: supabaseauth.Identity{
		ID: "supabase-oauth-id", Email: email, EmailConfirmed: true, Claims: map[string]any{"provider": "google"},
	}}, repository)

	verified, err := verifier.VerifyAccessToken(context.Background(), "access-token")
	if err != nil {
		t.Fatal(err)
	}
	if verified.UserID != "postgres-user-id" || repository.boundID != "supabase-oauth-id" || verified.Claims["provider"] != "google" {
		t.Fatalf("unexpected verified identity: %+v bound=%q", verified, repository.boundID)
	}
}

func TestSupabaseTokenVerifierRejectsUnconfirmedUnknownIdentity(t *testing.T) {
	verifier := NewSupabaseTokenVerifier(stubSupabaseIdentityProvider{identity: supabaseauth.Identity{
		ID: "unknown", Email: "unknown@example.com", EmailConfirmed: false,
	}}, &identityUserRepository{})
	if _, err := verifier.VerifyAccessToken(context.Background(), "access-token"); err == nil {
		t.Fatal("expected unknown identity rejection")
	}
}
