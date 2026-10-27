package reports_test

import (
	"be-dikasa/internal/catalog"
	"be-dikasa/internal/devseed"
	"be-dikasa/internal/payment"
	"be-dikasa/internal/refund"
	"be-dikasa/internal/reports"
	"be-dikasa/internal/user"
	"context"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

func reportsDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DIKASA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("opt in with DIKASA_TEST_DATABASE_URL")
	}
	u, e := url.Parse(dsn)
	if e != nil || (u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1") {
		t.Fatal("requires loopback PostgreSQL")
	}
	for k := range u.Query() {
		if k != "sslmode" && k != "connect_timeout" {
			t.Fatal("unsupported database URL option")
		}
	}
	base, e := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	schema := "dikasa_reports_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if e = base.Exec("CREATE SCHEMA " + schema).Error; e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := base.Exec("DROP SCHEMA " + schema + " CASCADE").Error; e != nil {
			t.Error(e)
		}
		sql, _ := base.DB()
		sql.Close()
	})
	q := u.Query()
	q.Set("search_path", schema+",public")
	u.RawQuery = q.Encode()
	db, e := gorm.Open(postgres.New(postgres.Config{DSN: u.String(), PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { sql, _ := db.DB(); sql.Close() })
	files, _ := filepath.Glob("../../migrations/*.up.sql")
	sort.Strings(files)
	for _, f := range files {
		b, e := os.ReadFile(f)
		if e != nil {
			t.Fatal(e)
		}
		if e = db.Exec(string(b)).Error; e != nil {
			t.Fatalf("%s: %v", filepath.Base(f), e)
		}
	}
	if e = devseed.Run(db, "integration-only-password"); e != nil {
		t.Fatal(e)
	}
	return db
}

func TestReportsPostgres(t *testing.T) {
	db := reportsDB(t)
	ctx := context.Background()
	var actor user.User
	db.Preload("Role").Where("username='admin'").First(&actor)
	out := *actor.OutletID
	var p catalog.Product
	db.Preload("Category").Where("outlet_id=?", out).First(&p)
	var ot catalog.OrderType
	db.Where("outlet_id=?", out).First(&ot)
	sid := uuid.NewString()
	if e := db.Exec("INSERT INTO cash_sessions(id,outlet_id,user_id,opening_cash,status) VALUES(?,?,?,0,'open')", sid, out, actor.ID).Error; e != nil {
		t.Fatal(e)
	}
	pay := payment.NewService(payment.NewGORMRepository(db), user.NewGORMRepository(db))
	sale, e := pay.CreateTransaction(ctx, actor.ID, payment.TransactionMutation{OrderTypeID: ot.ID, PaymentMethod: "qris", Subtotal: p.Price * 3, DiscountAmount: 30, TotalAmount: p.Price*3 - 30, PaidAmount: p.Price*3 - 30, Items: []payment.TransactionItemMutation{{ProductID: p.ID, ProductName: p.Name, UnitPrice: p.Price, Quantity: 3, LineTotal: p.Price * 3}}})
	if e != nil {
		t.Fatal(e)
	}
	var line payment.TransactionItem
	db.Where("transaction_id=?", sale.ID).First(&line)
	if line.CategoryIDSnapshot == nil || line.CategoryNameSnapshot == nil || *line.CategoryNameSnapshot != p.Category.Name {
		t.Fatal("missing original category", line)
	}
	if e = db.Exec("UPDATE transactions SET paid_at='2026-10-19T03:00:00Z' WHERE id=?", sale.ID).Error; e != nil {
		t.Fatal(e)
	}
	refunds := refund.NewService(refund.NewGORMRepository(db), user.NewGORMRepository(db))
	detail, e := refunds.Detail(ctx, actor.ID, sale.ID)
	if e != nil {
		t.Fatal(e)
	}
	selection := refund.Selection{Mode: "items", Method: "cash", Reason: "Report fixture", Items: []refund.SelectedItem{{TransactionItemID: detail.Items[0].ID, Quantity: 1}}}
	quote, e := refunds.Preview(ctx, actor.ID, sale.ID, selection)
	if e != nil {
		t.Fatal(e)
	}
	ret, e := refunds.Confirm(ctx, actor.ID, sale.ID, refund.Command{Selection: selection, RequestID: uuid.NewString(), PreviewHash: quote.SelectionHash})
	if e != nil {
		t.Fatal(e)
	}
	if e = db.Exec("UPDATE transaction_refunds SET created_at='2026-10-21T03:00:00Z' WHERE id=?", ret.ID).Error; e != nil {
		t.Fatal(e)
	}
	s := reports.NewService(reports.NewGORMRepository(db), user.NewGORMRepository(db), time.Now)
	r, e := s.Read(ctx, actor.ID, reports.Summary, reports.Filter{StartDate: "2026-10-21", EndDate: "2026-10-21"})
	if e != nil || r.Data.Totals.SaleCount != 0 || r.Data.Totals.NetSales != -(p.Price-10) {
		t.Fatal(r, e)
	}
	if e = db.Exec("UPDATE product_categories SET name='Changed later' WHERE id=?", p.CategoryID).Error; e != nil {
		t.Fatal(e)
	}
	r, e = s.Read(ctx, actor.ID, reports.Categories, reports.Filter{StartDate: "2026-10-19", EndDate: "2026-10-21"})
	if e != nil || len(r.Data.Rows) != 1 || r.Data.Rows[0].Name != p.Category.Name || r.Data.Totals.NetSales != (p.Price-10)*2 {
		t.Fatal("history changed", r, e)
	}
	if e = db.Exec("UPDATE transaction_items SET category_id_snapshot=NULL,category_name_snapshot=NULL WHERE id=?", line.ID).Error; e != nil {
		t.Fatal(e)
	}
	r, e = s.Read(ctx, actor.ID, reports.Categories, reports.Filter{StartDate: "2026-10-19", EndDate: "2026-10-21"})
	if e != nil || r.Data.Rows[0].Key != "unknown" {
		t.Fatal(r, e)
	}
	snap, e := reports.NewGORMRepository(db).ReadSnapshot(ctx, uuid.NewString(), reports.Window{Start: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), EndExclusive: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)})
	if e != nil || len(snap.Receipts) != 0 || len(snap.Refunds) != 0 {
		t.Fatal("outlet leak", snap, e)
	}
	// Category name snapshots serialize with a concurrent rename until sale commit.
	tx := db.Begin()
	payTx := payment.NewService(payment.NewGORMRepository(tx), user.NewGORMRepository(tx))
	concurrent, e := payTx.CreateTransaction(ctx, actor.ID, payment.TransactionMutation{OrderTypeID: ot.ID, PaymentMethod: "qris", Subtotal: p.Price, TotalAmount: p.Price, PaidAmount: p.Price, Items: []payment.TransactionItemMutation{{ProductID: p.ID, ProductName: p.Name, UnitPrice: p.Price, Quantity: 1, LineTotal: p.Price}}})
	if e != nil {
		tx.Rollback()
		t.Fatal(e)
	}
	renamed := make(chan error, 1)
	go func() {
		renamed <- db.Exec("UPDATE product_categories SET name='After concurrent sale' WHERE id=?", p.CategoryID).Error
	}()
	select {
	case e := <-renamed:
		tx.Rollback()
		t.Fatal("rename bypassed original category lock", e)
	case <-time.After(150 * time.Millisecond):
	}
	if e = tx.Commit().Error; e != nil {
		t.Fatal(e)
	}
	select {
	case e := <-renamed:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("rename did not resume")
	}
	var captured payment.TransactionItem
	db.Where("transaction_id=?", concurrent.ID).First(&captured)
	if captured.CategoryNameSnapshot == nil || *captured.CategoryNameSnapshot != "Changed later" {
		t.Fatal("mixed category snapshot", captured)
	}

}
