package savedorder

import (
	"context"
	"strings"

	"be-dikasa/internal/user"

	"github.com/google/uuid"
)

// AccountFinder obtains the authenticated account's outlet and role.
type AccountFinder interface {
	FindByIDWithRole(context.Context, string) (*user.User, error)
}

// Repository persists commands and reads only within an explicit outlet scope.
type Repository interface {
	Execute(context.Context, string, string, string, string, Mutation) (*Order, error)
	Get(context.Context, string, string) (*Order, error)
	List(context.Context, string, string, int, int) ([]Order, int64, error)
}

// Service derives authorization scope before executing draft operations.
type Service struct {
	repository Repository
	accounts   AccountFinder
}

// NewService constructs the saved-cart application boundary.
func NewService(r Repository, a AccountFinder) *Service { return &Service{r, a} }

// scope derives access from the authenticated account, never caller-supplied outlet fields.
func (s *Service) scope(ctx context.Context, id string) (string, error) {
	a, e := s.accounts.FindByIDWithRole(ctx, id)
	if e != nil {
		return "", ErrForbidden
	}
	role := strings.ToLower(a.Role.Name)
	if a.OutletID == nil || *a.OutletID == "" || (role != "manager" && role != "cashier") {
		return "", ErrForbidden
	}
	return *a.OutletID, nil
}

// Create stores a new draft or replays its successful creation.
func (s *Service) Create(ctx context.Context, userID string, m Mutation) (*Order, error) {
	return s.execute(ctx, userID, "create", "", m)
}

// Update replaces an open draft at the expected revision.
func (s *Service) Update(ctx context.Context, userID, id string, m Mutation) (*Order, error) {
	return s.execute(ctx, userID, "update", id, m)
}

// Delete soft-deletes a draft while retaining its audit and replay records.
func (s *Service) Delete(ctx context.Context, userID, id, requestID string, revision int64) (*Order, error) {
	return s.execute(ctx, userID, "delete", id, Mutation{RequestID: requestID, ExpectedRevision: revision})
}

// execute validates command shape before the repository obtains locks or writes state.
func (s *Service) execute(ctx context.Context, userID, op, id string, m Mutation) (*Order, error) {
	out, e := s.scope(ctx, userID)
	if e != nil {
		return nil, e
	}
	if op != "create" {
		if _, e = uuid.Parse(id); e != nil {
			return nil, ErrValidation
		}
		if m.ExpectedRevision < 1 {
			return nil, ErrValidation
		}
	}
	if _, e = uuid.Parse(m.RequestID); e != nil {
		return nil, ErrValidation
	}
	if op != "delete" {
		if e = ValidateMutation(m); e != nil {
			return nil, e
		}
	}
	return s.repository.Execute(ctx, out, userID, op, id, m)
}

// Get returns a draft including paid/deleted state for outcome recovery.
func (s *Service) Get(ctx context.Context, userID, id string) (*Order, error) {
	out, e := s.scope(ctx, userID)
	if e != nil {
		return nil, e
	}
	if _, e = uuid.Parse(id); e != nil {
		return nil, ErrValidation
	}
	return s.repository.Get(ctx, out, id)
}

// List browses only open outlet drafts.
func (s *Service) List(ctx context.Context, userID, search string, page, size int) ([]Order, int64, error) {
	out, e := s.scope(ctx, userID)
	if e != nil {
		return nil, 0, e
	}
	if page < 1 || size < 1 || size > 100 || page > 1000000 || len([]rune(search)) > 100 {
		return nil, 0, ErrValidation
	}
	return s.repository.List(ctx, out, strings.TrimSpace(search), page, size)
}
