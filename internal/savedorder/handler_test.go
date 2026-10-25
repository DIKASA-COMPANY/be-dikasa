package savedorder

import (
	"context"
	"net/http"
	"testing"

	sharederrors "be-dikasa/internal/shared/errors"
	"be-dikasa/internal/user"

	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/google/uuid"
)

func TestHandlerUnauthenticated(t *testing.T) {
	h := NewHandler(nil)
	_, e := h.Create(context.Background(), &CreateInput{})
	se, ok := e.(*sharederrors.StatusError)
	if !ok || se.GetStatus() != http.StatusUnauthorized {
		t.Fatal(e)
	}
}

func TestHTTPContractAcceptsCashierPayload(t *testing.T) {
	outlet := uuid.NewString()
	repo := &repoFake{}
	s := NewService(repo, accountsFake{user.User{OutletID: &outlet, Role: user.Role{Name: "cashier"}}})
	_, api := humatest.New(t)
	RegisterRoutes(api, NewHandler(s))
	ctx := user.WithAuthenticatedUserID(context.Background(), uuid.NewString())
	m := validMutation()
	body := map[string]any{"request_id": m.RequestID, "label": "A", "order_type_id": m.OrderTypeID, "items": []any{map[string]any{"product_id": m.Items[0].ProductID, "product_name": "Tea", "unit_price": 100, "quantity": 1, "notes": ""}}}
	r := api.DoCtx(ctx, "POST", "/saved-orders", body)
	if r.Code != 200 {
		t.Fatalf("create %d %s", r.Code, r.Body.String())
	}
	r = api.DoCtx(ctx, "PUT", "/saved-orders/"+uuid.NewString(), map[string]any{"request_id": uuid.NewString(), "expected_revision": 1, "label": "A", "order_type_id": m.OrderTypeID, "items": body["items"]})
	if r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	if r = api.DoCtx(ctx, "GET", "/saved-orders?page_size=101"); r.Code != 422 {
		t.Fatal("bad page size", r.Code)
	}
	if r = api.DoCtx(ctx, "GET", "/saved-orders/not-uuid"); r.Code != 422 {
		t.Fatal("bad path", r.Code)
	}
	if r = api.Do("POST", "/saved-orders", body); r.Code != 401 {
		t.Fatal("unauthenticated", r.Code)
	}
}
func TestStatusErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{ErrNotFound, 404}, {ErrConflict, 409}, {ErrRequestUsed, 409}, {ErrAlreadyPaid, 409}, {ErrValidation, 422}, {ErrForbidden, 403}} {
		if StatusError(tc.err).GetStatus() != tc.status {
			t.Fatal(tc.err)
		}
	}
}
