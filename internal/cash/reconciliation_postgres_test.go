package cash_test

import (
	"be-dikasa/internal/cash"
	"be-dikasa/internal/catalog"
	"be-dikasa/internal/devseed"
	"be-dikasa/internal/payment"
	"be-dikasa/internal/refund"
	"be-dikasa/internal/user"
	"context"
	"encoding/json"
	"errors"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func closingDB(t *testing.T) *gorm.DB {
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
	schema := "dikasa_cash_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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

func TestCashClosingPostgres(t *testing.T) {
	db := closingDB(t)
	ctx := context.Background()
	var actor user.User
	if e := db.Preload("Role").Where("username='admin'").First(&actor).Error; e != nil {
		t.Fatal(e)
	}
	out := *actor.OutletID
	sid := uuid.NewString()
	if e := db.Exec("INSERT INTO cash_sessions(id,outlet_id,user_id,opening_cash,status) VALUES(?,?,?,100000,'open')", sid, out, actor.ID).Error; e != nil {
		t.Fatal(e)
	}
	var product catalog.Product
	db.Where("outlet_id=?", out).First(&product)
	var ot catalog.OrderType
	db.Where("outlet_id=?", out).First(&ot)
	pay := payment.NewService(payment.NewGORMRepository(db), user.NewGORMRepository(db))
	sale, e := pay.CreateTransaction(ctx, actor.ID, payment.TransactionMutation{OrderTypeID: ot.ID, PaymentMethod: "cash", Subtotal: product.Price, TotalAmount: product.Price, PaidAmount: 100000, ChangeAmount: 100000 - product.Price, Items: []payment.TransactionItemMutation{{ProductID: product.ID, ProductName: product.Name, UnitPrice: product.Price, Quantity: 1, LineTotal: product.Price}}})
	if e != nil {
		t.Fatal(e)
	}
	service := cash.NewReconciliationService(cash.NewReconciliationRepository(db), user.NewGORMRepository(db))
	selected := cash.CloseSelection{CountedCash: 100000 + product.Price}
	preview, e := service.Preview(ctx, actor.ID, sid, selected)
	if e != nil || preview.Summary.ExpectedCash != selected.CountedCash {
		t.Fatal(preview, e)
	}
	command := cash.CloseCommand{CloseSelection: selected, RequestID: uuid.NewString(), PreviewHash: preview.PreviewHash}
	var wg sync.WaitGroup
	results := make(chan cash.ClosedReconciliation, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); r, e := service.Close(ctx, actor.ID, sid, command); results <- r; errs <- e }()
	}
	wg.Wait()
	close(results)
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	for r := range results {
		if r.SessionID != sid || r.Difference != 0 {
			t.Fatal(r)
		}
	}
	command.CountedCash--
	if _, e = service.Close(ctx, actor.ID, sid, command); !errors.Is(e, cash.ErrCloseRequestUsed) {
		t.Fatal("altered request", e)
	}
	command.CountedCash++
	record, e := service.Detail(ctx, actor.ID, sid)
	if e != nil || record.Closing == nil {
		t.Fatal(record, e)
	}
	frozen, _ := json.Marshal(record.Closing)
	// A prepared payment may never target a session already closed.
	prepared := *sale
	prepared.ID = uuid.NewString()
	prepared.TransactionNumber = "TEST-" + uuid.NewString()[:8]
	prepared.Items = nil
	if e = payment.NewGORMRepository(db).CreateTransaction(ctx, &prepared); !errors.Is(e, payment.ErrCashSessionNotFound) {
		t.Fatal("prepared payment reached closed till", e)
	}
	next := uuid.NewString()
	if e = db.Exec("INSERT INTO cash_sessions(id,outlet_id,user_id,opening_cash,status) VALUES(?,?,?,0,'open')", next, out, actor.ID).Error; e != nil {
		t.Fatal(e)
	}
	refunds := refund.NewService(refund.NewGORMRepository(db), user.NewGORMRepository(db))
	d, e := refunds.Detail(ctx, actor.ID, sale.ID)
	if e != nil {
		t.Fatal(e)
	}
	sel := refund.Selection{Mode: "full", Method: "cash", Reason: "Previous session sale", Items: []refund.SelectedItem{{TransactionItemID: d.Items[0].ID, Quantity: 1}}}
	rp, e := refunds.Preview(ctx, actor.ID, sale.ID, sel)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = refunds.Confirm(ctx, actor.ID, sale.ID, refund.Command{Selection: sel, RequestID: uuid.NewString(), PreviewHash: rp.SelectionHash}); e != nil {
		t.Fatal(e)
	}
	record, e = service.Detail(ctx, actor.ID, sid)
	after, _ := json.Marshal(record.Closing)
	if e != nil || string(after) != string(frozen) {
		t.Fatal("past summary changed", e)
	}
	if _, e = service.Close(ctx, actor.ID, sid, command); e != nil {
		t.Fatal("replay after new session", e)
	}
	nextRecord, e := service.Detail(ctx, actor.ID, next)
	if e != nil || nextRecord.Summary.ExpectedCash != -product.Price || nextRecord.Status != "open" {
		t.Fatal(nextRecord, e)
	}
	page, count, e := service.List(ctx, actor.ID, 1, 20)
	if e != nil || len(page) != 2 || count != 2 {
		t.Fatal(len(page), count, e)
	}
	if _, e = service.Detail(ctx, actor.ID, uuid.NewString()); !errors.Is(e, cash.ErrNotFound) {
		t.Fatal(e)
	}
	// Preview becomes stale when a new sale commits.
	selection := cash.CloseSelection{CountedCash: 0, Reason: "No cash left"}
	stale, e := service.Preview(ctx, actor.ID, next, selection)
	if e != nil {
		t.Fatal(e)
	}
	_, e = pay.CreateTransaction(ctx, actor.ID, payment.TransactionMutation{OrderTypeID: ot.ID, PaymentMethod: "qris", Subtotal: product.Price, TotalAmount: product.Price, PaidAmount: product.Price, Items: []payment.TransactionItemMutation{{ProductID: product.ID, ProductName: product.Name, UnitPrice: product.Price, Quantity: 1, LineTotal: product.Price}}})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = service.Close(ctx, actor.ID, next, cash.CloseCommand{CloseSelection: selection, RequestID: uuid.NewString(), PreviewHash: stale.PreviewHash}); !errors.Is(e, cash.ErrCloseStale) {
		t.Fatal("stale close", e)
	}
	// An uncommitted real payment holds the session shared lock through outer commit.
	fresh, e := service.Preview(ctx, actor.ID, next, selection)
	if e != nil {
		t.Fatal(e)
	}
	tx := db.Begin()
	pendingPay := payment.NewService(payment.NewGORMRepository(tx), user.NewGORMRepository(tx))
	inFlightSale, e := pendingPay.CreateTransaction(ctx, actor.ID, payment.TransactionMutation{OrderTypeID: ot.ID, PaymentMethod: "cash", Subtotal: product.Price, TotalAmount: product.Price, PaidAmount: product.Price, Items: []payment.TransactionItemMutation{{ProductID: product.ID, ProductName: product.Name, UnitPrice: product.Price, Quantity: 1, LineTotal: product.Price}}})
	if e != nil {
		tx.Rollback()
		t.Fatal(e)
	}
	waiting := make(chan error, 1)
	go func() {
		_, err := service.Close(ctx, actor.ID, next, cash.CloseCommand{CloseSelection: selection, RequestID: uuid.NewString(), PreviewHash: fresh.PreviewHash})
		waiting <- err
	}()
	select {
	case err := <-waiting:
		tx.Rollback()
		t.Fatal("closure bypassed in-flight payment", err)
	case <-time.After(150 * time.Millisecond):
	}
	if e = tx.Commit().Error; e != nil {
		t.Fatal(e)
	}
	select {
	case err := <-waiting:
		if !errors.Is(err, cash.ErrCloseStale) {
			t.Fatal("committed payment omitted", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("close did not resume")
	}
	// Refund transaction also holds the shared session lock until commit.
	beforeRefund, e := service.Preview(ctx, actor.ID, next, selection)
	if e != nil {
		t.Fatal(e)
	}
	refundTx := db.Begin()
	pendingRefund := refund.NewService(refund.NewGORMRepository(refundTx), user.NewGORMRepository(refundTx))
	refundDetail, e := pendingRefund.Detail(ctx, actor.ID, inFlightSale.ID)
	if e != nil {
		refundTx.Rollback()
		t.Fatal(e)
	}
	refundSelection := refund.Selection{Mode: "full", Method: "cash", Reason: "Concurrent refund", Items: []refund.SelectedItem{{TransactionItemID: refundDetail.Items[0].ID, Quantity: 1}}}
	refundPreview, e := pendingRefund.Preview(ctx, actor.ID, inFlightSale.ID, refundSelection)
	if e != nil {
		refundTx.Rollback()
		t.Fatal(e)
	}
	if _, e = pendingRefund.Confirm(ctx, actor.ID, inFlightSale.ID, refund.Command{Selection: refundSelection, RequestID: uuid.NewString(), PreviewHash: refundPreview.SelectionHash}); e != nil {
		refundTx.Rollback()
		t.Fatal(e)
	}
	go func() {
		_, err := service.Close(ctx, actor.ID, next, cash.CloseCommand{CloseSelection: selection, RequestID: uuid.NewString(), PreviewHash: beforeRefund.PreviewHash})
		waiting <- err
	}()
	select {
	case err := <-waiting:
		refundTx.Rollback()
		t.Fatal("closure bypassed in-flight refund", err)
	case <-time.After(150 * time.Millisecond):
	}
	if e = refundTx.Commit().Error; e != nil {
		t.Fatal(e)
	}
	select {
	case err := <-waiting:
		if !errors.Is(err, cash.ErrCloseStale) {
			t.Fatal("refund omitted", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("refund close did not resume")
	}
	// Inject failure on final closure UPDATE; no metadata/status may leak.
	ready, e := service.Preview(ctx, actor.ID, next, selection)
	if e != nil {
		t.Fatal(e)
	}
	if e = db.Exec(`CREATE FUNCTION reject_closure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.close_request_id IS NOT NULL THEN RAISE EXCEPTION 'injected closure failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_cash_close BEFORE UPDATE ON cash_sessions FOR EACH ROW EXECUTE FUNCTION reject_closure();`).Error; e != nil {
		t.Fatal(e)
	}
	failed := cash.CloseCommand{CloseSelection: selection, RequestID: uuid.NewString(), PreviewHash: ready.PreviewHash}
	if _, e = service.Close(ctx, actor.ID, next, failed); e == nil {
		t.Fatal("injected failure ignored")
	}
	if e = db.Exec("DROP TRIGGER reject_cash_close ON cash_sessions; DROP FUNCTION reject_closure();").Error; e != nil {
		t.Fatal(e)
	}
	still, e := service.Detail(ctx, actor.ID, next)
	if e != nil || still.Status != "open" || still.Closing != nil {
		t.Fatal("partial close leaked", still, e)
	}
	// Foreign outlet cannot retrieve either open or closed session.
	if _, e = cash.NewReconciliationRepository(db).Detail(ctx, uuid.NewString(), sid); !errors.Is(e, cash.ErrNotFound) {
		t.Fatal("foreign outlet read", e)
	}
	// Actual PostgreSQL-backed HTTP close and lookup, not a production till.
	_, api := humatest.New(t)
	cash.RegisterReconciliationRoutes(api, cash.NewReconciliationHandler(service))
	httpCtx := user.WithAuthenticatedUserID(ctx, actor.ID)
	response := api.DoCtx(httpCtx, "POST", "/cash-sessions/"+next+"/close/preview", selection)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	var body struct {
		Data cash.ClosePreview `json:"data"`
	}
	if e = json.Unmarshal(response.Body.Bytes(), &body); e != nil {
		t.Fatal(e)
	}
	response = api.DoCtx(httpCtx, "POST", "/cash-sessions/"+next+"/close", cash.CloseCommand{CloseSelection: selection, RequestID: uuid.NewString(), PreviewHash: body.Data.PreviewHash})
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	response = api.DoCtx(httpCtx, "GET", "/cash-sessions/"+next)
	if response.Code != 200 {
		t.Fatal(response.Code)
	}

}
