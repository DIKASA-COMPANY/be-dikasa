package reports

import (
	"be-dikasa/internal/user"
	"context"
	"time"
)

// Repository supplies one consistent original-money snapshot.
type Repository interface {
	ReadSnapshot(context.Context, string, Window) (Snapshot, error)
}

// Accounts resolves the authenticated outlet and current report role.
type Accounts interface {
	FindByIDWithRole(context.Context, string) (*user.User, error)
}

// Service derives all report authority from authenticated accounts.
type Service struct {
	repo     Repository
	accounts Accounts
	now      func() time.Time
}

// NewService supports deterministic defaults without changing ordinary POS clients.
func NewService(r Repository, a Accounts, clock func() time.Time) *Service {
	if clock == nil {
		clock = time.Now
	}
	return &Service{r, a, clock}
}
func (s *Service) read(ctx context.Context, actor string, k Kind, f Filter, paginate bool) (Result, error) {
	var result Result
	a, e := s.accounts.FindByIDWithRole(ctx, actor)
	if e != nil || a == nil || !a.IsActive || a.OutletID == nil || (a.Role.Name != "cashier" && a.Role.Name != "manager") {
		return result, ErrForbidden
	}
	f, w, e := NormalizeFilter(k, f, s.now())
	if e != nil {
		return result, e
	}
	snapshot, e := s.repo.ReadSnapshot(ctx, *a.OutletID, w)
	if e != nil {
		return result, e
	}
	r, e := Calculate(k, snapshot, f, w)
	if e != nil {
		return result, e
	}
	n := len(r.Rows)
	if k == Summary {
		n = len(r.Daily)
	}
	if k == Methods {
		n = len(r.Methods)
	}
	result = Result{Data: r, Meta: Meta{Query: f.Query, Sort: f.Sort, StartDate: w.StartDate, EndDate: w.EndDate, Timezone: w.Timezone, GeneratedAt: snapshot.GeneratedAt, Page: f.Page, PageSize: f.PageSize, Total: int64(n)}}
	if paginate && (k == Products || k == Categories) {
		start := (f.Page - 1) * f.PageSize
		if start > len(r.Rows) {
			start = len(r.Rows)
		}
		end := start + f.PageSize
		if end > len(r.Rows) {
			end = len(r.Rows)
		}
		result.Data.Rows = r.Rows[start:end]
	}
	switch k {
	case Summary:
		result.Data.Rows = []GroupRow{}
		result.Data.Methods = []MethodRow{}
	case Methods:
		result.Data.Rows = []GroupRow{}
		result.Data.Daily = []DailyPoint{}
	default:
		result.Data.Daily = []DailyPoint{}
		result.Data.Methods = []MethodRow{}
	}
	return result, nil
}

// Read applies pagination only after computing all matching group totals.
func (s *Service) Read(ctx context.Context, actor string, k Kind, f Filter) (Result, error) {
	return s.read(ctx, actor, k, f, true)
}
