package refund

import (
	"be-dikasa/internal/user"
	"context"
	"github.com/google/uuid"
	"strings"
)

// AccountFinder derives refund permission from the authenticated identity.
type AccountFinder interface {
	FindByIDWithRole(context.Context, string) (*user.User, error)
}

// Repository owns consistent snapshots and transactional refund writes.
type Repository interface {
	Detail(context.Context, string, string) (ReceiptSnapshot, error)
	History(context.Context, string, string, int, int) ([]Refund, int64, error)
	FindByRequest(context.Context, string, string) (*Refund, error)
	Confirm(context.Context, string, string, string, Command) (*Refund, error)
}

// Service scopes every refund operation to an outlet cashier or manager.
type Service struct {
	repository Repository
	accounts   AccountFinder
}

// NewService builds the refund boundary from existing account and database adapters.
func NewService(r Repository, a AccountFinder) *Service { return &Service{r, a} }

// scope never accepts an outlet supplied by the request body.
func (s *Service) scope(ctx context.Context, id string) (string, error) {
	a, e := s.accounts.FindByIDWithRole(ctx, id)
	if e != nil || a == nil || a.OutletID == nil {
		return "", ErrForbidden
	}
	role := strings.ToLower(a.Role.Name)
	if role != "cashier" && role != "manager" {
		return "", ErrForbidden
	}
	return *a.OutletID, nil
}
func validID(id string) bool { _, e := uuid.Parse(id); return e == nil }

// Detail loads original amounts and committed refund eligibility.
func (s *Service) Detail(ctx context.Context, userID, id string) (ReceiptSnapshot, error) {
	out, e := s.scope(ctx, userID)
	if e != nil {
		return ReceiptSnapshot{}, e
	}
	if !validID(id) {
		return ReceiptSnapshot{}, ErrValidation
	}
	return s.repository.Detail(ctx, out, canonical(id))
}

// History browses immutable refund audit entries with bounded pagination.
func (s *Service) History(ctx context.Context, userID, id string, page, size int) ([]Refund, int64, error) {
	out, e := s.scope(ctx, userID)
	if e != nil {
		return nil, 0, e
	}
	if !validID(id) || page < 1 || page > 1000000 || size < 1 || size > 100 {
		return nil, 0, ErrValidation
	}
	return s.repository.History(ctx, out, canonical(id), page, size)
}

// Preview calculates original-value amounts without recording a command.
func (s *Service) Preview(ctx context.Context, userID, id string, sel Selection) (Preview, error) {
	r, e := s.Detail(ctx, userID, id)
	if e != nil {
		return Preview{}, e
	}
	return BuildPreview(r, sel)
}

// Confirm writes or replays one exact reviewed refund command.
func (s *Service) Confirm(ctx context.Context, userID, id string, c Command) (*Refund, error) {
	out, e := s.scope(ctx, userID)
	if e != nil {
		return nil, e
	}
	if !validID(id) || !validID(c.RequestID) || len(c.PreviewHash) != 64 {
		return nil, ErrValidation
	}
	return s.repository.Confirm(ctx, out, userID, canonical(id), c)
}

// ByRequest resolves only a committed refund in the authenticated outlet.
func (s *Service) ByRequest(ctx context.Context, userID, id string) (*Refund, error) {
	out, e := s.scope(ctx, userID)
	if e != nil {
		return nil, e
	}
	if !validID(id) {
		return nil, ErrValidation
	}
	return s.repository.FindByRequest(ctx, out, canonical(id))
}
