package refund_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/danielgtaylor/huma/v2/humatest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"be-dikasa/internal/catalog"
	"be-dikasa/internal/devseed"
	"be-dikasa/internal/payment"
	"be-dikasa/internal/refund"
	"be-dikasa/internal/sales"
	"be-dikasa/internal/user"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testDB(t *testing.T) *gorm.DB {
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
	schema := "dikasa_refund_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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

func TestRefundsPostgres(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	var actor user.User
	db.Preload("Role").Where("username = 'admin'").First(&actor)
	out := *actor.OutletID
	var p catalog.Product
	db.Where("outlet_id = ?", out).First(&p)
	var ot catalog.OrderType
	db.Where("outlet_id = ?", out).First(&ot)
	cash := uuid.NewString()
	if e := db.Exec("INSERT INTO cash_sessions(id,outlet_id,user_id,opening_cash,status,opened_at) VALUES (?,?,?,0,'open',NOW())", cash, out, actor.ID).Error; e != nil {
		t.Fatal(e)
	}
	db.Model(&catalog.Product{}).Where("id = ?", p.ID).Updates(map[string]any{"track_stock": true, "stock_quantity": 20})
	pay := payment.NewService(payment.NewGORMRepository(db), user.NewGORMRepository(db))
	sale, e := pay.CreateTransaction(ctx, actor.ID, payment.TransactionMutation{OrderTypeID: ot.ID, PaymentMethod: "cash", Subtotal: p.Price * 3, TaxAmount: 3, TotalAmount: p.Price*3 + 3, PaidAmount: p.Price*3 + 3, Items: []payment.TransactionItemMutation{{ProductID: p.ID, ProductName: p.Name, UnitPrice: p.Price, Quantity: 3, LineTotal: p.Price * 3}}})
	if e != nil {
		t.Fatal(e)
	}
	repo := refund.NewGORMRepository(db)
	s := refund.NewService(repo, user.NewGORMRepository(db))
	detail, e := s.Detail(ctx, actor.ID, sale.ID)
	if e != nil || detail.Items[0].RestockBudget != 3 {
		t.Fatal(detail, e)
	}
	sel := refund.Selection{Mode: "items", Method: "cash", Reason: "Test return", Items: []refund.SelectedItem{{TransactionItemID: detail.Items[0].ID, Quantity: 1, RestockQuantity: 1}}}
	preview, e := s.Preview(ctx, actor.ID, sale.ID, sel)
	if e != nil {
		t.Fatal(e)
	}
	cmd := refund.Command{Selection: sel, RequestID: uuid.NewString(), PreviewHash: preview.SelectionHash}
	var wg sync.WaitGroup
	ch := make(chan *refund.Refund, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); r, e := s.Confirm(ctx, actor.ID, sale.ID, cmd); ch <- r; errs <- e }()
	}
	wg.Wait()
	for i := 0; i < 2; i++ {
		if e := <-errs; e != nil {
			t.Fatal(e)
		}
	}
	a, b := <-ch, <-ch
	if a.ID != b.ID {
		t.Fatal("duplicate refund")
	}
	db.First(&p, "id = ?", p.ID)
	if p.StockQuantity != 18 {
		t.Fatal("return applied twice", p.StockQuantity)
	}
	d, e := s.Detail(ctx, actor.ID, sale.ID)
	if e != nil || d.Status != "partially_refunded" || d.Items[0].RefundedQuantity != 1 {
		t.Fatal(d, e)
	}
	full := sel
	full.Mode = "full"
	full.ExpectedRevision = 1
	full.Items = append([]refund.SelectedItem(nil), sel.Items...)
	full.Items[0].Quantity = 2
	full.Items[0].RestockQuantity = 0
	pv, e := s.Preview(ctx, actor.ID, sale.ID, full)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.Confirm(ctx, actor.ID, sale.ID, refund.Command{Selection: full, RequestID: uuid.NewString(), PreviewHash: pv.SelectionHash})
	if e != nil || r.TotalAmount+a.TotalAmount != sale.TotalAmount {
		t.Fatal(r, e)
	}
	replay, e := s.Confirm(ctx, actor.ID, sale.ID, cmd)
	if e != nil || replay.ID != a.ID {
		t.Fatal("late replay", e)
	}
	changed := cmd
	changed.Reason = "changed"
	if _, e = s.Confirm(ctx, actor.ID, sale.ID, changed); !errors.Is(e, refund.ErrRequestUsed) {
		t.Fatal("changed replay accepted", e)
	}
	if _, e = repo.Detail(ctx, uuid.NewString(), sale.ID); !errors.Is(e, refund.ErrNotFound) {
		t.Fatal("outlet leak", e)
	}
	d, e = s.Detail(ctx, actor.ID, sale.ID)
	if e != nil || d.Status != "refunded" || d.RefundedAmount != sale.TotalAmount {
		t.Fatal(d, e)
	}
	makeSale := func() *payment.Transaction {
		t.Helper()
		value, e := pay.CreateTransaction(ctx, actor.ID, payment.TransactionMutation{OrderTypeID: ot.ID, PaymentMethod: "cash", Subtotal: p.Price * 2, TotalAmount: p.Price * 2, PaidAmount: p.Price * 2, Items: []payment.TransactionItemMutation{{ProductID: p.ID, ProductName: p.Name, UnitPrice: p.Price, Quantity: 2, LineTotal: p.Price * 2}}})
		if e != nil {
			t.Fatal(e)
		}
		return value
	}
	commandFor := func(id string, restock int) refund.Command {
		t.Helper()
		d, e := s.Detail(ctx, actor.ID, id)
		if e != nil {
			t.Fatal(e)
		}
		selection := refund.Selection{Mode: "items", Method: "cash", Reason: "Extra tests", ExpectedRevision: d.RefundRevision, Items: []refund.SelectedItem{{TransactionItemID: d.Items[0].ID, Quantity: 1, RestockQuantity: restock}}}
		pv, e := s.Preview(ctx, actor.ID, id, selection)
		if e != nil {
			t.Fatal(e)
		}
		return refund.Command{Selection: selection, RequestID: uuid.NewString(), PreviewHash: pv.SelectionHash}
	}
	race := makeSale()
	one := commandFor(race.ID, 0)
	two := one
	two.RequestID = uuid.NewString()
	wg = sync.WaitGroup{}
	raceErrors := make(chan error, 2)
	for _, c := range []refund.Command{one, two} {
		wg.Add(1)
		go func(c refund.Command) { defer wg.Done(); _, e := s.Confirm(ctx, actor.ID, race.ID, c); raceErrors <- e }(c)
	}
	wg.Wait()
	wins := 0
	for i := 0; i < 2; i++ {
		e := <-raceErrors
		if e == nil {
			wins++
		} else if !errors.Is(e, refund.ErrRevision) {
			t.Fatal(e)
		}
	}
	if wins != 1 {
		t.Fatal("distinct command race winners", wins)
	}
	// Refund/stock/session changes cannot invalidate the immutable replay of an earlier success.
	rollbackSale := makeSale()
	rollbackCmd := commandFor(rollbackSale.ID, 1)
	db.First(&p, "id = ?", p.ID)
	beforeStock := p.StockQuantity
	var beforeRefunds, beforeMovements int64
	db.Table("transaction_refunds").Count(&beforeRefunds)
	db.Table("stock_movements").Count(&beforeMovements)
	if e = db.Exec(`CREATE FUNCTION reject_refund_return() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.kind='refund_return' THEN RAISE EXCEPTION 'injected rollback failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_return BEFORE INSERT ON stock_movements FOR EACH ROW EXECUTE FUNCTION reject_refund_return();`).Error; e != nil {
		t.Fatal(e)
	}
	if _, e = s.Confirm(ctx, actor.ID, rollbackSale.ID, rollbackCmd); e == nil {
		t.Fatal("injected failure ignored")
	}
	if e = db.Exec("DROP TRIGGER reject_return ON stock_movements; DROP FUNCTION reject_refund_return();").Error; e != nil {
		t.Fatal(e)
	}
	var afterRefunds, afterMovements int64
	db.Table("transaction_refunds").Count(&afterRefunds)
	db.Table("stock_movements").Count(&afterMovements)
	db.First(&p, "id = ?", p.ID)
	rolled, e := s.Detail(ctx, actor.ID, rollbackSale.ID)
	if e != nil || p.StockQuantity != beforeStock || beforeRefunds != afterRefunds || beforeMovements != afterMovements || rolled.RefundRevision != 0 || rolled.Status != "paid" {
		t.Fatal("refund failure leaked writes", e)
	}
	// A sale without original tracked deduction cannot manufacture returned inventory.
	db.Model(&catalog.Product{}).Where("id = ?", p.ID).Update("track_stock", false)
	untracked := makeSale()
	db.Model(&catalog.Product{}).Where("id = ?", p.ID).Update("track_stock", true)
	u, e := s.Detail(ctx, actor.ID, untracked.ID)
	if e != nil {
		t.Fatal(e)
	}
	untrackedSel := refund.Selection{Mode: "items", Method: "cash", Reason: "Untracked", Items: []refund.SelectedItem{{TransactionItemID: u.Items[0].ID, Quantity: 1, RestockQuantity: 1}}}
	if _, e = s.Preview(ctx, actor.ID, untracked.ID, untrackedSel); !errors.Is(e, refund.ErrRestock) {
		t.Fatal("manufactured stock", e)
	}
	disabled := makeSale()
	disabledCommand := commandFor(disabled.ID, 1)
	db.Model(&catalog.Product{}).Where("id = ?", p.ID).Update("track_stock", false)
	if _, e = s.Confirm(ctx, actor.ID, disabled.ID, disabledCommand); !errors.Is(e, refund.ErrRestock) {
		t.Fatal("disabled tracking accepted", e)
	}
	db.Model(&catalog.Product{}).Where("id = ?", p.ID).Update("track_stock", true)
	db.Model(&catalog.Product{}).Where("id = ?", p.ID).Update("stock_quantity", 1000000000)
	if _, e = s.Confirm(ctx, actor.ID, disabled.ID, disabledCommand); !errors.Is(e, refund.ErrRestock) {
		t.Fatal("stock overflow accepted", e)
	}
	db.Model(&catalog.Product{}).Where("id = ?", p.ID).Update("stock_quantity", 10)

	// Exercise actual Huma preview/confirmation/detail with PostgreSQL-backed handlers.
	apiSale := makeSale()
	apiDetail, e := s.Detail(ctx, actor.ID, apiSale.ID)
	if e != nil {
		t.Fatal(e)
	}
	_, api := humatest.New(t)
	refund.RegisterRoutes(api, refund.NewHandler(s))
	httpCtx := user.WithAuthenticatedUserID(ctx, actor.ID)
	apiSel := refund.Selection{Mode: "full", Method: "transfer", Reason: "API fixture", Items: []refund.SelectedItem{{TransactionItemID: apiDetail.Items[0].ID, Quantity: 2}}}
	response := api.DoCtx(httpCtx, "POST", "/transactions/"+apiSale.ID+"/refunds/preview", apiSel)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	var quote struct {
		Data refund.Preview `json:"data"`
	}
	if e = json.Unmarshal(response.Body.Bytes(), &quote); e != nil {
		t.Fatal(e)
	}
	apiCommand := refund.Command{Selection: apiSel, RequestID: uuid.NewString(), PreviewHash: quote.Data.SelectionHash}
	response = api.DoCtx(httpCtx, "POST", "/transactions/"+apiSale.ID+"/refunds", apiCommand)
	if response.Code != 201 {
		t.Fatal(response.Code, response.Body.String())
	}
	response = api.DoCtx(httpCtx, "GET", "/refunds/by-request/"+apiCommand.RequestID)
	if response.Code != 200 {
		t.Fatal(response.Code)
	}
	// Listing uses grouped header totals and preserves the original receipt value.
	orders, count, e := sales.NewGORMRepository(db).ListOrders(ctx, sales.OrderFilter{OutletID: out, Search: apiSale.TransactionNumber, Status: "refunded", Page: 1, PageSize: 20})
	if e != nil || count != 1 || len(orders) != 1 {
		t.Fatal("refund listing count", count, e)
	}
	listed := orders[0]
	if listed.TotalAmount != apiSale.TotalAmount || listed.RefundedAmount == nil || *listed.RefundedAmount != apiSale.TotalAmount || listed.NetAfterRefund == nil || *listed.NetAfterRefund != 0 || !listed.RefundsSupported {
		t.Fatal("refund listing changed original or net", listed)
	}
	response = api.DoCtx(httpCtx, "GET", "/transactions/"+apiSale.ID)
	if response.Code != 200 {
		t.Fatal(response.Code)
	}
	if e = db.Table("cash_sessions").Where("id = ?", cash).Updates(map[string]any{"status": "closed", "closing_cash": 0, "closed_at": time.Now()}).Error; e != nil {
		t.Fatal(e)
	}
	if replay, e = s.Confirm(ctx, actor.ID, sale.ID, cmd); e != nil || replay.ID != a.ID {
		t.Fatal("closed-session replay", e)
	}
	fresh := commandFor(untracked.ID, 0)
	if _, e = s.Confirm(ctx, actor.ID, untracked.ID, fresh); !errors.Is(e, refund.ErrCashSession) {
		t.Fatal("new refund without cash session", e)
	}

}
