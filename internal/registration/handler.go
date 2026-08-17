package registration

import (
	"context"
	"errors"
	"net/http"

	sharederrors "be-dikasa/internal/shared/errors"
	sharedvalidator "be-dikasa/internal/shared/validator"
)

// Provider mendefinisikan use case registrasi untuk handler HTTP.
type Provider interface {
	// Register membuat registrasi tertunda dan mengirim OTP awal.
	Register(ctx context.Context, registration Registration) (Result, error)
	// ConfirmOTP mengonfirmasi kode dan mengaktifkan registrasi.
	ConfirmOTP(ctx context.Context, email, code string) error
	// ResendOTP mengganti dan mengirim ulang OTP.
	ResendOTP(ctx context.Context, email string) (Result, error)
}

// Handler melayani endpoint registrasi dan OTP melalui Huma.
type Handler struct {
	provider  Provider
	validator *sharedvalidator.Validator
}

// NewHandler membuat handler registrasi dengan provider dan validator strict.
func NewHandler(provider Provider, validator *sharedvalidator.Validator) *Handler {
	return &Handler{provider: provider, validator: validator}
}

// Register memvalidasi seluruh form registrasi lalu mengirim OTP.
func (handler *Handler) Register(ctx context.Context, input *RegisterInput) (*RegisterOutput, error) {
	if details := handler.validator.Validate(input.Body); len(details) > 0 {
		return nil, validationError(details)
	}
	request := input.Body
	result, err := handler.provider.Register(ctx, Registration{
		BusinessName: request.BusinessName, BusinessAddress: request.BusinessAddress, BusinessPhone: request.BusinessPhone,
		BusinessEmail: request.BusinessEmail, OwnerName: request.OwnerName, OwnerDateOfBirth: request.OwnerDateOfBirth,
		OwnerPhone: request.OwnerPhone, OwnerEmail: request.OwnerEmail, Username: request.Username, Password: request.Password,
		VerificationEmail: request.VerificationEmail,
	})
	if err != nil {
		return nil, registrationStatusError(err)
	}
	return &RegisterOutput{Status: http.StatusCreated, Body: otpResponse(result)}, nil
}

// ConfirmOTP memvalidasi kode lalu mengaktifkan akun dan outlet.
func (handler *Handler) ConfirmOTP(ctx context.Context, input *ConfirmOTPInput) (*ConfirmOTPOutput, error) {
	if details := handler.validator.Validate(input.Body); len(details) > 0 {
		return nil, validationError(details)
	}
	if err := handler.provider.ConfirmOTP(ctx, input.Body.Email, input.Body.OTP); err != nil {
		return nil, registrationStatusError(err)
	}
	return &ConfirmOTPOutput{Body: ConfirmOTPResponse{Data: ConfirmOTPResponseData{Verified: true}}}, nil
}

// ResendOTP merotasi dan mengirim ulang OTP untuk registrasi tertunda.
func (handler *Handler) ResendOTP(ctx context.Context, input *ResendOTPInput) (*ResendOTPOutput, error) {
	if details := handler.validator.Validate(input.Body); len(details) > 0 {
		return nil, validationError(details)
	}
	result, err := handler.provider.ResendOTP(ctx, input.Body.Email)
	if err != nil {
		return nil, registrationStatusError(err)
	}
	return &ResendOTPOutput{Body: otpResponse(result)}, nil
}

func otpResponse(result Result) OTPResponse {
	return OTPResponse{Data: OTPResponseData{Email: result.Email, OTPExpiresIn: result.OTPExpiresIn, ResendAvailableIn: result.ResendAvailableIn}}
}

func validationError(details map[string]string) *sharederrors.StatusError {
	return sharederrors.New(http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Data yang dikirim tidak memenuhi ketentuan.", details)
}

// registrationStatusError mencegah detail database, hash, atau SMTP bocor ke respons API.
func registrationStatusError(err error) *sharederrors.StatusError {
	switch {
	case errors.Is(err, ErrConflict):
		return sharederrors.New(http.StatusConflict, "REGISTRASI_SUDAH_ADA", "Username, nomor telepon, atau email sudah terdaftar.", nil)
	case errors.Is(err, ErrVerificationEmailInvalid):
		return sharederrors.New(http.StatusUnprocessableEntity, "EMAIL_VERIFIKASI_TIDAK_VALID", "Email verifikasi harus menggunakan email pemilik atau email usaha.", map[string]string{"verification_email": "Pilih salah satu email yang didaftarkan."})
	case errors.Is(err, ErrInvalidRegistration):
		return sharederrors.New(http.StatusUnprocessableEntity, "REGISTRASI_TIDAK_VALID", "Data registrasi tidak dapat diproses.", nil)
	case errors.Is(err, ErrOTPInvalid):
		return sharederrors.New(http.StatusUnprocessableEntity, "OTP_TIDAK_VALID", "Kode OTP tidak valid.", map[string]string{"otp": "Kode OTP tidak valid."})
	case errors.Is(err, ErrOTPExpired):
		return sharederrors.New(http.StatusGone, "OTP_KEDALUWARSA", "Kode OTP telah kedaluwarsa. Silakan kirim ulang.", nil)
	case errors.Is(err, ErrOTPAttemptsExceeded):
		return sharederrors.New(http.StatusTooManyRequests, "BATAS_PERCOBAAN_OTP", "Batas percobaan OTP telah habis. Silakan kirim ulang kode.", nil)
	case errors.Is(err, ErrResendTooSoon):
		return sharederrors.New(http.StatusTooManyRequests, "KIRIM_ULANG_TERLALU_CEPAT", "Tunggu sebelum meminta kode OTP baru.", nil)
	case errors.Is(err, ErrAlreadyVerified):
		return sharederrors.New(http.StatusConflict, "EMAIL_SUDAH_TERVERIFIKASI", "Email sudah terverifikasi.", nil)
	case errors.Is(err, ErrNotFound):
		return sharederrors.Standard(http.StatusNotFound, nil)
	case errors.Is(err, ErrDeliveryFailed):
		return sharederrors.New(http.StatusBadGateway, "EMAIL_GAGAL_DIKIRIM", "Kode OTP belum dapat dikirim. Silakan coba lagi.", nil)
	default:
		return sharederrors.Standard(http.StatusInternalServerError, nil)
	}
}
