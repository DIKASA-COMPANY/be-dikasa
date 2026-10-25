package cash

import (
	"be-dikasa/internal/user"
	"context"
	"github.com/google/uuid"
	"strings"
)

// ReconciliationAccounts resolves the current authenticated role and outlet.
type ReconciliationAccounts interface {
	FindByIDWithRole(context.Context, string) (*user.User, error)
}

// ReconciliationStore separates closure behavior from existing opening interfaces.
type ReconciliationStore interface {
	Detail(context.Context, string, string) (SessionRecord, error)
	List(context.Context, string, int, int) ([]SessionRecord, int64, error)
	PreviewSnapshot(context.Context, string, string) (SessionSnapshot, error)
	Close(context.Context, string, string, string, CloseCommand) (ClosedReconciliation, error)
}

// ReconciliationService enforces outlet and opening-cashier/manager permissions.
type ReconciliationService struct {
	repo     ReconciliationStore
	accounts ReconciliationAccounts
}

// NewReconciliationService uses existing authenticated users and a scoped store.
func NewReconciliationService(r ReconciliationStore, a ReconciliationAccounts) *ReconciliationService {
	return &ReconciliationService{r, a}
}
func (s *ReconciliationService) account(ctx context.Context, id string) (*user.User, error) {
	a, e := s.accounts.FindByIDWithRole(ctx, id)
	if e != nil || a == nil || !a.IsActive || a.OutletID == nil {
		return nil, ErrCloseForbidden
	}
	if a.Role.Name != "cashier" && a.Role.Name != "manager" {
		return nil, ErrCloseForbidden
	}
	return a, nil
}

// mayClose implements the user-selected opening-cashier or manager policy.
func mayClose(a *user.User, opener string) bool {
	return a != nil && a.IsActive && (a.Role.Name == "manager" || (a.Role.Name == "cashier" && a.ID == opener))
}

// Detail returns one scoped session with caller-specific close eligibility.
func (s *ReconciliationService) Detail(ctx context.Context, actor, id string) (SessionRecord, error) {
	a, e := s.account(ctx, actor)
	if e != nil {
		return SessionRecord{}, e
	}
	u, e := uuid.Parse(id)
	if e != nil {
		return SessionRecord{}, ErrCloseInvalid
	}
	r, e := s.repo.Detail(ctx, *a.OutletID, u.String())
	if e == nil {
		r.CanClose = r.Status == StatusOpen && r.Supported && mayClose(a, r.UserID)
	}
	return r, e
}

// List provides stable session history with per-caller closure eligibility.
func (s *ReconciliationService) List(ctx context.Context, actor string, page, size int) ([]SessionRecord, int64, error) {
	a, e := s.account(ctx, actor)
	if e != nil {
		return nil, 0, e
	}
	if page < 1 || page > 1000000 || size < 1 || size > 100 {
		return nil, 0, ErrCloseInvalid
	}
	rows, n, e := s.repo.List(ctx, *a.OutletID, page, size)
	for i := range rows {
		rows[i].CanClose = rows[i].Status == StatusOpen && rows[i].Supported && mayClose(a, rows[i].UserID)
	}
	return rows, n, e
}

// Preview validates a cashier count against current, consistent activity.
func (s *ReconciliationService) Preview(ctx context.Context, actor, id string, c CloseSelection) (ClosePreview, error) {
	a, e := s.account(ctx, actor)
	if e != nil {
		return ClosePreview{}, e
	}
	u, e := uuid.Parse(id)
	if e != nil {
		return ClosePreview{}, ErrCloseInvalid
	}
	snap, e := s.repo.PreviewSnapshot(ctx, *a.OutletID, u.String())
	if e != nil {
		return ClosePreview{}, e
	}
	if !mayClose(a, snap.OpeningUserID) {
		return ClosePreview{}, ErrCloseForbidden
	}
	return BuildClosePreview(snap, c)
}

// Close accepts only an exact reviewed command; database replay precedes mutable state.
func (s *ReconciliationService) Close(ctx context.Context, actor, id string, c CloseCommand) (ClosedReconciliation, error) {
	a, e := s.account(ctx, actor)
	if e != nil {
		return ClosedReconciliation{}, e
	}
	u, e := uuid.Parse(id)
	if e != nil {
		return ClosedReconciliation{}, ErrCloseInvalid
	}
	request, e := uuid.Parse(c.RequestID)
	if e != nil || len(c.PreviewHash) != 64 || c.CountedCash < 0 || c.CountedCash > maxOpeningCash || len([]rune(c.Reason)) > 500 {
		return ClosedReconciliation{}, ErrCloseInvalid
	}
	c.RequestID = request.String()
	c.Reason = strings.TrimSpace(c.Reason)
	return s.repo.Close(ctx, *a.OutletID, actor, u.String(), c)
}
