package cash

import (
	"context"
	"errors"
	"net/http"

	sharederrors "be-dikasa/internal/shared/errors"
	sharedvalidator "be-dikasa/internal/shared/validator"
	"be-dikasa/internal/user"
)

// Provider mendefinisikan use case sesi kas untuk handler HTTP.
type Provider interface {
	Open(ctx context.Context, userID string, openingCash int64) (*Session, error)
	Current(ctx context.Context, userID string) (*Session, error)
}

// Handler melayani endpoint sesi kas melalui Huma.
type Handler struct {
	provider  Provider
	validator *sharedvalidator.Validator
}

// NewHandler membuat handler sesi kas dengan provider dan validator strict.
func NewHandler(provider Provider, validator *sharedvalidator.Validator) *Handler {
	return &Handler{provider: provider, validator: validator}
}

// Open memvalidasi kas awal dan membuka sesi kas outlet user terautentikasi.
func (handler *Handler) Open(ctx context.Context, input *OpenInput) (*OpenOutput, error) {
	userID, ok := user.AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return nil, sharederrors.Standard(http.StatusUnauthorized, nil)
	}
	if details := handler.validator.Validate(input.Body); len(details) > 0 {
		return nil, sharederrors.New(http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Data yang dikirim tidak memenuhi ketentuan.", details)
	}
	session, err := handler.provider.Open(ctx, userID, input.Body.OpeningCash)
	if err != nil {
		return nil, cashStatusError(err)
	}
	return &OpenOutput{Status: http.StatusCreated, Body: sessionResponse(session)}, nil
}

// Current mengembalikan sesi kas aktif untuk outlet user terautentikasi.
func (handler *Handler) Current(ctx context.Context, _ *struct{}) (*CurrentOutput, error) {
	userID, ok := user.AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return nil, sharederrors.Standard(http.StatusUnauthorized, nil)
	}
	session, err := handler.provider.Current(ctx, userID)
	if err != nil {
		return nil, cashStatusError(err)
	}
	return &CurrentOutput{Body: sessionResponse(session)}, nil
}

func sessionResponse(session *Session) SessionResponse {
	return SessionResponse{Data: SessionData{
		ID: session.ID, OutletID: session.OutletID, UserID: session.UserID,
		OpeningCash: session.OpeningCash, ClosingCash: session.ClosingCash,
		Status: session.Status, OpenedAt: session.OpenedAt, ClosedAt: session.ClosedAt,
	}}
}

// cashStatusError menjaga detail database agar tidak bocor ke response API.
func cashStatusError(err error) *sharederrors.StatusError {
	switch {
	case errors.Is(err, ErrInvalidOpeningCash):
		return sharederrors.New(http.StatusUnprocessableEntity, "KAS_AWAL_TIDAK_VALID", "Kas awal berada di luar batas yang didukung.", map[string]string{"opening_cash": "Kas awal harus antara 0 dan 9007199254740991 Rupiah."})
	case errors.Is(err, ErrUserWithoutOutlet):
		return sharederrors.New(http.StatusForbidden, "USER_TANPA_OUTLET", "User belum terhubung ke outlet.", nil)
	case errors.Is(err, ErrUserNotFound):
		return sharederrors.Standard(http.StatusNotFound, nil)
	case errors.Is(err, ErrActiveSessionExists):
		return sharederrors.New(http.StatusConflict, "SESI_KAS_SUDAH_AKTIF", "Outlet sudah memiliki sesi kas aktif.", nil)
	case errors.Is(err, ErrNotFound):
		return sharederrors.New(http.StatusNotFound, "SESI_KAS_TIDAK_DITEMUKAN", "Outlet belum memiliki sesi kas aktif.", nil)
	default:
		return sharederrors.Standard(http.StatusInternalServerError, nil)
	}
}
