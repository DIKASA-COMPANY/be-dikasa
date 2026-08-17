package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"

	sharederrors "be-dikasa/internal/shared/errors"
	sharedvalidator "be-dikasa/internal/shared/validator"
)

type stubSignInProvider struct {
	session Session
	err     error
}

func (stub stubSignInProvider) SignIn(_ context.Context, _, _ string) (Session, error) {
	return stub.session, stub.err
}

func TestLoginReturnsFirebaseSession(t *testing.T) {
	handler := NewHandler(stubSignInProvider{session: Session{
		IDToken: "firebase-id-token", RefreshToken: "firebase-refresh-token",
		ExpiresIn: 3600, FirebaseUID: "firebase-user-id",
	}}, sharedvalidator.New())

	output, err := handler.Login(context.Background(), &LoginInput{Body: LoginRequest{
		Identifier: "kasir@example.com", Password: "rahasia123",
	}})

	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if output.Body.Data.AccessToken != "firebase-id-token" || output.Body.Data.TokenType != "Bearer" {
		t.Fatalf("unexpected login output: %+v", output.Body.Data)
	}
}

func TestLoginUsesStrictGoValidator(t *testing.T) {
	handler := NewHandler(stubSignInProvider{}, sharedvalidator.New())

	_, err := handler.Login(context.Background(), &LoginInput{Body: LoginRequest{
		Identifier: "a", Password: "pendek",
	}})

	statusError, ok := err.(*sharederrors.StatusError)
	if !ok {
		t.Fatalf("expected StatusResponse, got %T", err)
	}
	if statusError.GetStatus() != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, statusError.GetStatus())
	}
	if statusError.Body.Details["identifier"] == "" || statusError.Body.Details["password"] == "" {
		t.Fatalf("unexpected validation details: %+v", statusError.Body.Details)
	}
}

func TestLoginMapsProviderErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		statusCode int
		code       string
	}{
		{"invalid credentials", ErrInvalidCredentials, http.StatusUnauthorized, "KREDENSIAL_TIDAK_VALID"},
		{"disabled user", ErrUserDisabled, http.StatusForbidden, "AKUN_DINONAKTIFKAN"},
		{"provider unavailable", ErrProviderUnavailable, http.StatusBadGateway, "FIREBASE_TIDAK_TERSEDIA"},
		{"unexpected", errors.New("secret internal error"), http.StatusInternalServerError, "KESALAHAN_INTERNAL"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewHandler(stubSignInProvider{err: test.err}, sharedvalidator.New())
			_, err := handler.Login(context.Background(), &LoginInput{Body: LoginRequest{
				Identifier: "kasir@example.com", Password: "rahasia123",
			}})

			statusError, ok := err.(*sharederrors.StatusError)
			if !ok {
				t.Fatalf("expected StatusResponse, got %T", err)
			}
			if statusError.GetStatus() != test.statusCode || statusError.Body.Code != test.code {
				t.Fatalf("unexpected error: %+v", statusError)
			}
		})
	}
}
