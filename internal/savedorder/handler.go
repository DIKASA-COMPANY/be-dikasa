package savedorder

import (
	"context"
	"errors"
	"net/http"

	sharederrors "be-dikasa/internal/shared/errors"
	"be-dikasa/internal/user"
)

// Handler exposes saved-cart operations through authenticated Huma transport.
type Handler struct{ service *Service }

// NewHandler constructs saved-order HTTP handlers.
func NewHandler(s *Service) *Handler { return &Handler{s} }
func identity(ctx context.Context) (string, error) {
	id, ok := user.AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return "", sharederrors.Standard(401, nil)
	}
	return id, nil
}
func output(o *Order, e error) (*Output, error) {
	if e != nil {
		return nil, StatusError(e)
	}
	out := &Output{}
	out.Body.Data = o
	return out, nil
}

// Create stores an unpaid cart without changing inventory.
func (h *Handler) Create(ctx context.Context, in *CreateInput) (*Output, error) {
	id, e := identity(ctx)
	if e != nil {
		return nil, e
	}
	return output(h.service.Create(ctx, id, in.Body))
}

// Update replaces an open saved cart at its expected revision.
func (h *Handler) Update(ctx context.Context, in *UpdateInput) (*Output, error) {
	id, e := identity(ctx)
	if e != nil {
		return nil, e
	}
	return output(h.service.Update(ctx, id, in.ID, in.Body))
}

// Delete retains an audit snapshot while removing the cart from open browsing.
func (h *Handler) Delete(ctx context.Context, in *DeleteInput) (*Output, error) {
	id, e := identity(ctx)
	if e != nil {
		return nil, e
	}
	return output(h.service.Delete(ctx, id, in.ID, in.RequestID, in.ExpectedRevision))
}

// Get retrieves a current or completed draft for recovery.
func (h *Handler) Get(ctx context.Context, in *DetailInput) (*Output, error) {
	id, e := identity(ctx)
	if e != nil {
		return nil, e
	}
	return output(h.service.Get(ctx, id, in.ID))
}

// List returns open drafts visible at the authenticated outlet.
func (h *Handler) List(ctx context.Context, in *ListInput) (*ListOutput, error) {
	id, e := identity(ctx)
	if e != nil {
		return nil, e
	}
	rows, n, e := h.service.List(ctx, id, in.Search, in.Page, in.PageSize)
	if e != nil {
		return nil, StatusError(e)
	}
	out := &ListOutput{}
	out.Body.Data = rows
	out.Body.Meta.Page = in.Page
	out.Body.Meta.PageSize = in.PageSize
	out.Body.Meta.Total = n
	return out, nil
}

// StatusError maps draft failures to stable client recovery codes.
func StatusError(e error) *sharederrors.StatusError {
	switch {
	case errors.Is(e, ErrAlreadyPaid):
		return sharederrors.New(409, "PESANAN_SUDAH_DIBAYAR", "Pesanan sudah dibayar. Muat ulang untuk melihat struk.", nil)
	case errors.Is(e, ErrRequestUsed):
		return sharederrors.New(409, "REQUEST_ID_DIGUNAKAN", "ID permintaan sudah digunakan untuk perubahan lain.", nil)
	case errors.Is(e, ErrConflict):
		return sharederrors.New(409, "PESANAN_TERSIMPAN_BERUBAH", "Pesanan berubah. Muat ulang sebelum melanjutkan.", nil)
	case errors.Is(e, ErrNotFound):
		return sharederrors.Standard(404, nil)
	case errors.Is(e, ErrForbidden):
		return sharederrors.Standard(403, nil)
	case errors.Is(e, ErrValidation):
		return sharederrors.Standard(422, nil)
	default:
		return sharederrors.Standard(http.StatusInternalServerError, nil)
	}
}
