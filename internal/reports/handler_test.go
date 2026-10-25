package reports

import (
	"archive/zip"
	"be-dikasa/internal/user"
	"bytes"
	"context"
	"github.com/danielgtaylor/huma/v2/humatest"
	"io"
	"strings"
	"testing"
	"time"
)

type testAccounts struct {
	role   string
	active bool
}

func (a testAccounts) FindByIDWithRole(context.Context, string) (*user.User, error) {
	out := "outlet"
	return &user.User{ID: "actor", OutletID: &out, Role: user.Role{Name: a.role}, IsActive: a.active}, nil
}

type testStore struct{}

func (testStore) ReadSnapshot(context.Context, string, Window) (Snapshot, error) {
	return fixture(), nil
}
func TestReportsHTTPAndExport(t *testing.T) {
	service := NewService(testStore{}, testAccounts{"cashier", true}, time.Now)
	_, api := humatest.New(t)
	RegisterRoutes(api, NewHandler(service))
	if r := api.Get("/reports/summary"); r.Code != 401 {
		t.Fatal(r.Code)
	}
	ctx := user.WithAuthenticatedUserID(context.Background(), "actor")
	if r := api.DoCtx(ctx, "GET", "/reports/summary?start_date=2026-10-21&end_date=2026-10-21"); r.Code != 200 {
		t.Fatal(r.Code, r.Body.String())
	}
	if r := api.DoCtx(ctx, "GET", "/reports/summary?start_date=2026-10-22&end_date=2026-10-21"); r.Code != 422 {
		t.Fatal(r.Code)
	}
	service.accounts = testAccounts{"admin", true}
	if _, e := service.Read(ctx, "actor", Summary, Filter{}); e != ErrForbidden {
		t.Fatal(e)
	}
	s := fixture()
	s.Receipts[0].Transaction.Items[0].ProductName = "=HYPERLINK(unsafe)"
	f, w, _ := NormalizeFilter(Products, Filter{StartDate: "2026-10-19", EndDate: "2026-10-21"}, time.Now())
	r, e := Calculate(Products, s, f, w)
	if e != nil {
		t.Fatal(e)
	}
	b, e := workbook(Result{Data: r, Meta: Meta{StartDate: w.StartDate, EndDate: w.EndDate, Timezone: w.Timezone, Query: "Tea filter", Sort: "net_desc"}})
	if e != nil {
		t.Fatal(e)
	}
	z, e := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if e != nil {
		t.Fatal(e)
	}
	var sheet string
	for _, file := range z.File {
		if file.Name == "xl/worksheets/sheet1.xml" {
			rd, _ := file.Open()
			raw, _ := io.ReadAll(rd)
			rd.Close()
			sheet = string(raw)
		}
	}
	if !strings.Contains(sheet, "Tea filter") || !strings.Contains(sheet, "net_desc") || !strings.Contains(sheet, "=HYPERLINK(unsafe)") || strings.Contains(sheet, "<f>") || !strings.Contains(sheet, "<v>180</v>") {
		t.Fatal("not literal/numeric workbook")
	}
}

func TestSummaryOmitsUnrelatedGroups(t *testing.T) {
	s := NewService(testStore{}, testAccounts{"cashier", true}, time.Now)
	r, e := s.Read(context.Background(), "actor", Summary, Filter{StartDate: "2026-10-19", EndDate: "2026-10-21"})
	if e != nil || len(r.Data.Rows) != 0 || len(r.Data.Methods) != 0 {
		t.Fatal("unrelated payload arrays", len(r.Data.Rows), len(r.Data.Methods), e)
	}
}
