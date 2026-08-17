package registration

import (
	"context"
	"errors"
	"net/http"
	"testing"

	sharederrors "be-dikasa/internal/shared/errors"
	sharedvalidator "be-dikasa/internal/shared/validator"
)

type stubProvider struct {
	result Result
	err    error
}

func (stub stubProvider) Register(context.Context, Registration) (Result, error) {
	return stub.result, stub.err
}
func (stub stubProvider) ConfirmOTP(context.Context, string, string) error { return stub.err }
func (stub stubProvider) ResendOTP(context.Context, string) (Result, error) {
	return stub.result, stub.err
}

func TestHandlerRejectsMismatchedPasswordConfirmation(t *testing.T) {
	handler := NewHandler(stubProvider{}, sharedvalidator.New())
	request := validRegistrationRequest()
	request.PasswordConfirmation = "berbeda123"

	_, err := handler.Register(context.Background(), &RegisterInput{Body: request})

	statusError, ok := err.(*sharederrors.StatusError)
	if !ok || statusError.GetStatus() != http.StatusUnprocessableEntity || statusError.Body.Details["password_confirmation"] == "" {
		t.Fatalf("unexpected validation error: %#v", err)
	}
}

func TestHandlerMapsRegistrationErrors(t *testing.T) {
	tests := []struct {
		err    error
		status int
		code   string
	}{
		{ErrConflict, http.StatusConflict, "REGISTRASI_SUDAH_ADA"},
		{ErrInvalidRegistration, http.StatusUnprocessableEntity, "REGISTRASI_TIDAK_VALID"},
		{ErrOTPInvalid, http.StatusUnprocessableEntity, "OTP_TIDAK_VALID"},
		{ErrOTPExpired, http.StatusGone, "OTP_KEDALUWARSA"},
		{ErrResendTooSoon, http.StatusTooManyRequests, "KIRIM_ULANG_TERLALU_CEPAT"},
		{ErrDeliveryFailed, http.StatusBadGateway, "EMAIL_GAGAL_DIKIRIM"},
		{errors.New("database secret"), http.StatusInternalServerError, "KESALAHAN_INTERNAL"},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			handler := NewHandler(stubProvider{err: test.err}, sharedvalidator.New())
			_, err := handler.ConfirmOTP(context.Background(), &ConfirmOTPInput{Body: ConfirmOTPRequest{Email: "budi@example.com", OTP: "12345"}})
			statusError, ok := err.(*sharederrors.StatusError)
			if !ok || statusError.GetStatus() != test.status || statusError.Body.Code != test.code {
				t.Fatalf("unexpected mapped error: %#v", err)
			}
		})
	}
}

func validRegistrationRequest() RegisterRequest {
	return RegisterRequest{
		BusinessName: "Warung Dikasa", BusinessAddress: "Jl. Merdeka 10", BusinessPhone: "081234567890",
		OwnerName: "Budi Santoso", OwnerDateOfBirth: "1990-01-02", OwnerPhone: "081298765432",
		OwnerEmail: "budi@example.com", Username: "budisantoso", Password: "rahasia123",
		PasswordConfirmation: "rahasia123", VerificationEmail: "budi@example.com",
	}
}
