package cash

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	sharederrors "be-dikasa/internal/shared/errors"
	sharedvalidator "be-dikasa/internal/shared/validator"
	"be-dikasa/internal/user"
)

type stubProvider struct {
	session *Session
	err     error
	amount  int64
	userID  string
}

func (provider *stubProvider) Open(_ context.Context, userID string, amount int64) (*Session, error) {
	provider.userID = userID
	provider.amount = amount
	return provider.session, provider.err
}

func (provider *stubProvider) Current(_ context.Context, userID string) (*Session, error) {
	provider.userID = userID
	return provider.session, provider.err
}

func TestHandlerOpensCashSession(t *testing.T) {
	openedAt := time.Date(2026, time.August, 19, 1, 30, 0, 0, time.UTC)
	provider := &stubProvider{session: &Session{
		ID: "session-id", OutletID: "outlet-id", UserID: "user-id",
		OpeningCash: 500_000, Status: StatusOpen, OpenedAt: openedAt,
	}}
	handler := NewHandler(provider, sharedvalidator.New())
	ctx := user.WithAuthenticatedUserID(context.Background(), "user-id")

	output, err := handler.Open(ctx, &OpenInput{Body: OpenRequest{OpeningCash: 500_000}})

	if err != nil {
		t.Fatalf("open cash session: %v", err)
	}
	if output.Status != http.StatusCreated || provider.amount != 500_000 || provider.userID != "user-id" {
		t.Fatalf("unexpected handler result: %+v", output)
	}
	if output.Body.Data.ID != "session-id" || output.Body.Data.OpeningCash != 500_000 {
		t.Fatalf("unexpected response: %+v", output.Body.Data)
	}
}

func TestHandlerOpenValidatesOpeningCash(t *testing.T) {
	provider := &stubProvider{}
	handler := NewHandler(provider, sharedvalidator.New())
	ctx := user.WithAuthenticatedUserID(context.Background(), "user-id")

	_, err := handler.Open(ctx, &OpenInput{Body: OpenRequest{OpeningCash: -1}})

	assertStatusError(t, err, http.StatusUnprocessableEntity, "VALIDASI_GAGAL")
	if provider.userID != "" {
		t.Fatal("invalid payload must not reach service")
	}
}

func TestHandlerOpenRequiresIdentity(t *testing.T) {
	handler := NewHandler(&stubProvider{}, sharedvalidator.New())

	_, err := handler.Open(context.Background(), &OpenInput{Body: OpenRequest{OpeningCash: 0}})

	assertStatusError(t, err, http.StatusUnauthorized, "AUTENTIKASI_DIPERLUKAN")
}

func TestHandlerMapsActiveSessionConflict(t *testing.T) {
	handler := NewHandler(&stubProvider{err: ErrActiveSessionExists}, sharedvalidator.New())
	ctx := user.WithAuthenticatedUserID(context.Background(), "user-id")

	_, err := handler.Open(ctx, &OpenInput{Body: OpenRequest{OpeningCash: 500_000}})

	assertStatusError(t, err, http.StatusConflict, "SESI_KAS_SUDAH_AKTIF")
}

func TestHandlerCurrentMapsMissingSession(t *testing.T) {
	handler := NewHandler(&stubProvider{err: ErrNotFound}, sharedvalidator.New())
	ctx := user.WithAuthenticatedUserID(context.Background(), "user-id")

	_, err := handler.Current(ctx, &struct{}{})

	assertStatusError(t, err, http.StatusNotFound, "SESI_KAS_TIDAK_DITEMUKAN")
}

func TestHandlerMapsUserWithoutOutlet(t *testing.T) {
	handler := NewHandler(&stubProvider{err: ErrUserWithoutOutlet}, sharedvalidator.New())
	ctx := user.WithAuthenticatedUserID(context.Background(), "user-id")

	_, err := handler.Current(ctx, &struct{}{})

	assertStatusError(t, err, http.StatusForbidden, "USER_TANPA_OUTLET")
}

func assertStatusError(t *testing.T, err error, status int, code string) {
	t.Helper()
	var statusError *sharederrors.StatusError
	if !errors.As(err, &statusError) || statusError.GetStatus() != status || statusError.Body.Code != code {
		t.Fatalf("expected status %d code %s, got %T %v", status, code, err, err)
	}
}
