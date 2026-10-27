package savedorder

import (
	"context"
	"errors"
	"testing"

	"be-dikasa/internal/user"

	"github.com/google/uuid"
)

type accountsFake struct{ account user.User }

func (a accountsFake) FindByIDWithRole(context.Context, string) (*user.User, error) {
	return &a.account, nil
}

type repoFake struct{ outlet, actor string }

func (r *repoFake) Execute(_ context.Context, outlet, actor, op, id string, m Mutation) (*Order, error) {
	r.outlet = outlet
	r.actor = actor
	return &Order{ID: id, OutletID: outlet, Revision: 1}, nil
}
func (r *repoFake) Get(_ context.Context, outlet, id string) (*Order, error) {
	r.outlet = outlet
	return nil, ErrNotFound
}
func (r *repoFake) List(_ context.Context, outlet, search string, p, s int) ([]Order, int64, error) {
	r.outlet = outlet
	return []Order{}, 0, nil
}
func TestServiceScopeAndBounds(t *testing.T) {
	outlet := uuid.NewString()
	r := &repoFake{}
	a := accountsFake{user.User{OutletID: &outlet, Role: user.Role{Name: "cashier"}}}
	s := NewService(r, a)
	ctx := context.Background()
	actor := uuid.NewString()
	if _, e := s.Create(ctx, actor, validMutation()); e != nil || r.outlet != outlet || r.actor != actor {
		t.Fatalf("scope %v %#v", e, r)
	}
	if _, e := s.Get(ctx, actor, uuid.NewString()); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
	for _, bounds := range [][2]int{{0, 20}, {1, 101}, {-1, 10}} {
		if _, _, e := s.List(ctx, actor, "", bounds[0], bounds[1]); !errors.Is(e, ErrValidation) {
			t.Fatal("accepted bad bounds")
		}
	}
	a.account.Role.Name = "customer"
	if _, e := NewService(r, a).Create(ctx, actor, validMutation()); !errors.Is(e, ErrForbidden) {
		t.Fatal(e)
	}
	a.account.Role.Name = "manager"
	a.account.OutletID = nil
	if _, e := NewService(r, a).Create(ctx, actor, validMutation()); !errors.Is(e, ErrForbidden) {
		t.Fatal(e)
	}
}
