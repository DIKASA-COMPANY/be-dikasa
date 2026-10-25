package cash

import (
	"be-dikasa/internal/user"
	"context"
	"errors"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/google/uuid"
	"testing"
)

type closingAccounts struct{ account *user.User }

func (a closingAccounts) FindByIDWithRole(context.Context, string) (*user.User, error) {
	return a.account, nil
}

type closingStore struct{ snap SessionSnapshot }

func (r closingStore) Detail(context.Context, string, string) (SessionRecord, error) {
	return SessionRecord{SessionData: SessionData{ID: r.snap.ID, UserID: r.snap.OpeningUserID, Status: "open"}, Supported: true}, nil
}
func (r closingStore) List(context.Context, string, int, int) ([]SessionRecord, int64, error) {
	return []SessionRecord{}, 0, nil
}
func (r closingStore) PreviewSnapshot(context.Context, string, string) (SessionSnapshot, error) {
	return r.snap, nil
}
func (r closingStore) Close(context.Context, string, string, string, CloseCommand) (ClosedReconciliation, error) {
	return ClosedReconciliation{}, nil
}
func TestClosingPermissionAndHTTP(t *testing.T) {
	out := uuid.NewString()
	actor := uuid.NewString()
	snap := snapshot()
	snap.ID = uuid.NewString()
	snap.OpeningUserID = actor
	a := &user.User{ID: actor, IsActive: true, OutletID: &out, Role: user.Role{Name: "cashier"}}
	service := NewReconciliationService(closingStore{snap}, closingAccounts{a})
	if _, e := service.Preview(context.Background(), actor, snap.ID, CloseSelection{CountedCash: 170000}); e != nil {
		t.Fatal(e)
	}
	a.ID = uuid.NewString()
	if _, e := service.Preview(context.Background(), a.ID, snap.ID, CloseSelection{CountedCash: 170000}); !errors.Is(e, ErrCloseForbidden) {
		t.Fatal(e)
	}
	a.Role.Name = "manager"
	if _, e := service.Preview(context.Background(), a.ID, snap.ID, CloseSelection{CountedCash: 170000}); e != nil {
		t.Fatal(e)
	}
	_, api := humatest.New(t)
	RegisterReconciliationRoutes(api, NewReconciliationHandler(service))
	path := "/cash-sessions/" + snap.ID + "/close/preview"
	if res := api.Post(path, map[string]any{"counted_cash": 170000, "reason": ""}); res.Code != 401 {
		t.Fatal(res.Code)
	}
	ctx := user.WithAuthenticatedUserID(context.Background(), a.ID)
	if res := api.DoCtx(ctx, "POST", path, map[string]any{"counted_cash": 170000, "reason": ""}); res.Code != 200 {
		t.Fatal(res.Code, res.Body.String())
	}
	a.IsActive = false
	if _, e := service.Detail(ctx, a.ID, snap.ID); !errors.Is(e, ErrCloseForbidden) {
		t.Fatal(e)
	}
}
