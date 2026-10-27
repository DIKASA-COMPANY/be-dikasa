package refund

import (
	"be-dikasa/internal/user"
	"context"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/google/uuid"
	"testing"
)

type testAccounts struct{ role string }

func (a testAccounts) FindByIDWithRole(context.Context, string) (*user.User, error) {
	out := uuid.NewString()
	return &user.User{OutletID: &out, Role: user.Role{Name: a.role}}, nil
}

type testRepo struct{ snap ReceiptSnapshot }

func (r testRepo) Detail(context.Context, string, string) (ReceiptSnapshot, error) {
	return r.snap, nil
}
func (r testRepo) History(context.Context, string, string, int, int) ([]Refund, int64, error) {
	return []Refund{}, 0, nil
}
func (r testRepo) FindByRequest(context.Context, string, string) (*Refund, error) {
	return nil, ErrNotFound
}
func (r testRepo) Confirm(context.Context, string, string, string, Command) (*Refund, error) {
	return &Refund{ID: uuid.NewString(), Items: []RefundItem{}}, nil
}
func TestRefundHTTPContract(t *testing.T) {
	snap := receipt()
	s := NewService(testRepo{snap}, testAccounts{"cashier"})
	_, api := humatest.New(t)
	RegisterRoutes(api, NewHandler(s))
	ctx := user.WithAuthenticatedUserID(context.Background(), uuid.NewString())
	sel := selection(snap, 1)
	r := api.DoCtx(ctx, "POST", "/transactions/"+snap.ID+"/refunds/preview", sel)
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	p, _ := BuildPreview(snap, sel)
	c := Command{Selection: sel, RequestID: uuid.NewString(), PreviewHash: p.SelectionHash}
	r = api.DoCtx(ctx, "POST", "/transactions/"+snap.ID+"/refunds", c)
	if r.Code != 201 {
		t.Fatal(r.Code, r.Body.String())
	}
	if r = api.Do("GET", "/transactions/"+snap.ID); r.Code != 401 {
		t.Fatal(r.Code)
	}
	if r = api.DoCtx(ctx, "GET", "/transactions/invalid"); r.Code != 422 {
		t.Fatal(r.Code)
	}
	if _, e := NewService(testRepo{snap}, testAccounts{"customer"}).Detail(ctx, "actor", snap.ID); e != ErrForbidden {
		t.Fatal(e)
	}
}
