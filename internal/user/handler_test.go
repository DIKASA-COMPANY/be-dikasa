package user

import (
	"context"
	"net/http"
	"testing"

	sharederrors "be-dikasa/internal/shared/errors"
)

func TestHandlerMeReturnsPublicUserWithoutCredentialHashes(t *testing.T) {
	service := NewService(stubRepository{user: &User{
		ID: "user-id", Username: "kasir", PhoneNumber: "08123456789",
		PasswordHash: "secret-hash", FirstName: "Budi", LastName: "Kasir", IsActive: true,
	}})
	handler := NewHandler(service)
	ctx := WithAuthenticatedUserID(context.Background(), "user-id")

	output, err := handler.Me(ctx, &struct{}{})

	if err != nil {
		t.Fatalf("get profile: %v", err)
	}
	if output.Body.Data.ID != "user-id" || output.Body.Data.Username != "kasir" {
		t.Fatalf("unexpected output: %+v", output.Body.Data)
	}
}

func TestHandlerMeRequiresIdentity(t *testing.T) {
	handler := NewHandler(NewService(stubRepository{}))

	_, err := handler.Me(context.Background(), &struct{}{})

	statusError, ok := err.(*sharederrors.StatusError)
	if !ok || statusError.GetStatus() != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized StatusError, got %T %v", err, err)
	}
}
