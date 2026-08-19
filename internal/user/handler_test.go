package user

import (
	"context"
	"net/http"
	"testing"

	sharederrors "be-dikasa/internal/shared/errors"
	sharedvalidator "be-dikasa/internal/shared/validator"
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

func TestHandlerChangePasswordRejectsMismatchedConfirmation(t *testing.T) {
	repository := &settingsRepository{account: &User{ID: "user-id", PasswordHash: "hash:rahasia123"}}
	handler := NewHandler(NewService(repository, WithCredentialHasher(plainCredentialHasher{})), sharedvalidator.New())
	ctx := WithAuthenticatedUserID(context.Background(), "user-id")

	_, err := handler.ChangePassword(ctx, &ChangePasswordInput{Body: ChangePasswordRequest{
		CurrentPassword: "rahasia123", NewPassword: "password-baru", NewPasswordConfirmation: "berbeda123",
	}})
	statusError, ok := err.(*sharederrors.StatusError)
	if !ok || statusError.GetStatus() != http.StatusUnprocessableEntity {
		t.Fatalf("expected validation status, got %T %v", err, err)
	}
}

func TestHandlerChangePINReturnsNoContent(t *testing.T) {
	repository := &settingsRepository{account: &User{ID: "user-id"}}
	handler := NewHandler(NewService(repository, WithCredentialHasher(plainCredentialHasher{})), sharedvalidator.New())
	ctx := WithAuthenticatedUserID(context.Background(), "user-id")

	output, err := handler.ChangePIN(ctx, &ChangePINInput{Body: ChangePINRequest{NewPIN: "123456", NewPINConfirmation: "123456"}})
	if err != nil || output.Status != http.StatusNoContent || repository.pinHash != "hash:123456" {
		t.Fatalf("change PIN: output=%+v hash=%q err=%v", output, repository.pinHash, err)
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
