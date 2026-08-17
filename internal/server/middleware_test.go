package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	firebaseauth "firebase.google.com/go/v4/auth"
	"github.com/gin-gonic/gin"

	"be-dikasa/internal/user"
)

type stubTokenVerifier struct {
	token *firebaseauth.Token
	err   error
}

func (stub stubTokenVerifier) VerifyIDToken(context.Context, string) (*firebaseauth.Token, error) {
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
	router.Use(AuthMiddleware(stubTokenVerifier{token: &firebaseauth.Token{UID: "postgres-user-id"}}, nil))
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
