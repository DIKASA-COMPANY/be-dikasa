package user

import (
	"context"
	"errors"
	"net/http"

	sharederrors "be-dikasa/internal/shared/errors"
)

// Handler melayani endpoint user melalui Huma.
type Handler struct {
	service *Service
}

// NewHandler membuat handler user.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Me mengembalikan profil user yang sedang terautentikasi.
func (handler *Handler) Me(ctx context.Context, _ *struct{}) (*ProfileOutput, error) {
	userID, ok := AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return nil, sharederrors.Standard(http.StatusUnauthorized, nil)
	}
	account, err := handler.service.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, sharederrors.Standard(http.StatusNotFound, nil)
		}
		return nil, sharederrors.Standard(http.StatusInternalServerError, nil)
	}
	return &ProfileOutput{Body: ProfileResponse{Data: Profile{
		ID: account.ID, RoleID: account.RoleID, GenderID: account.GenderID, OutletID: account.OutletID,
		Username: account.Username, PhoneNumber: account.PhoneNumber, Email: account.Email,
		FirstName: account.FirstName, LastName: account.LastName, IsActive: account.IsActive,
	}}}, nil
}
