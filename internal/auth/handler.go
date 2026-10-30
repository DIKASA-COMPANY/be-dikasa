package auth

import (
	"context"
	"errors"
	"net/http"

	sharederrors "be-dikasa/internal/shared/errors"
	sharedvalidator "be-dikasa/internal/shared/validator"
)

// Handler melayani endpoint autentikasi melalui Huma.
type Handler struct {
	provider  SignInProvider
	validator *sharedvalidator.Validator
}

// NewHandler membuat handler autentikasi dengan provider dan validator strict.
func NewHandler(provider SignInProvider, validator *sharedvalidator.Validator) *Handler {
	return &Handler{provider: provider, validator: validator}
}

// Login memvalidasi input dan menjalankan use case login.
func (handler *Handler) Login(ctx context.Context, input *LoginInput) (*LoginOutput, error) {
	if details := handler.validator.Validate(input.Body); len(details) > 0 {
		return nil, sharederrors.New(
			http.StatusUnprocessableEntity,
			"VALIDASI_GAGAL",
			"Data yang dikirim tidak memenuhi ketentuan.",
			details,
		)
	}
	session, err := handler.provider.SignIn(ctx, input.Body.Identifier, input.Body.Password)
	if err != nil {
		return nil, loginStatusError(err)
	}
	return &LoginOutput{Body: LoginResponse{Data: LoginResponseData{
		AccessToken: session.AccessToken, RefreshToken: session.RefreshToken,
		TokenType: "Bearer", ExpiresIn: session.ExpiresIn, SupabaseUserID: session.SupabaseUserID,
	}}}, nil
}

func loginStatusError(err error) *sharederrors.StatusError {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		return sharederrors.New(http.StatusUnauthorized, "KREDENSIAL_TIDAK_VALID", "Identifier atau password salah.", nil)
	case errors.Is(err, ErrUserDisabled):
		return sharederrors.New(http.StatusForbidden, "AKUN_DINONAKTIFKAN", "Akun Anda sedang dinonaktifkan.", nil)
	case errors.Is(err, ErrProviderUnavailable):
		return sharederrors.New(http.StatusBadGateway, "SUPABASE_TIDAK_TERSEDIA", "Layanan autentikasi sedang bermasalah.", nil)
	default:
		return sharederrors.Standard(http.StatusInternalServerError, nil)
	}
}
