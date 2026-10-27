package savedorder_test

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"be-dikasa/internal/catalog"
	"be-dikasa/internal/devseed"
	"be-dikasa/internal/payment"
	"be-dikasa/internal/savedorder"
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
	schema := "dikasa_draft_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
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

func TestSavedOrderPaymentPostgres(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	var actor user.User
	db.Where("username = 'admin'").First(&actor)
	out := *actor.OutletID
	var p catalog.Product
	db.Where("outlet_id = ?", out).First(&p)
	var ot catalog.OrderType
	db.Where("outlet_id = ?", out).First(&ot)
	cash := uuid.NewString()
	if e := db.Exec("INSERT INTO cash_sessions(id,outlet_id,user_id,opening_cash,status,opened_at) VALUES (?,?,?,0,'open',NOW())", cash, out, actor.ID).Error; e != nil {
		t.Fatal(e)
	}
	enabled := true
	initial := int64(20)
	stock := catalog.NewStockService(db, user.NewGORMRepository(db))
	if _, e := stock.Execute(ctx, actor.ID, p.ID, "configure", catalog.StockCommand{RequestID: uuid.NewString(), Reason: "Test", TrackStock: &enabled, InitialQuantity: &initial}); e != nil {
		t.Fatal(e)
	}
	s := savedorder.NewService(savedorder.NewGORMRepository(db), user.NewGORMRepository(db))
	pay := payment.NewService(payment.NewGORMRepository(db), user.NewGORMRepository(db))
	create := func() *savedorder.Order {
		t.Helper()
		o, e := s.Create(ctx, actor.ID, savedorder.Mutation{RequestID: uuid.NewString(), OrderTypeID: ot.ID, Items: []savedorder.Item{{ProductID: p.ID, ProductName: p.Name, UnitPrice: p.Price, Quantity: 1}}})
		if e != nil {
			t.Fatal(e)
		}
		return o
	}
	mutation := func(o *savedorder.Order) payment.TransactionMutation {
		return payment.TransactionMutation{SavedOrderID: &o.ID, SavedOrderRevision: &o.Revision, OrderTypeID: ot.ID, PaymentMethod: "cash", Subtotal: p.Price, TotalAmount: p.Price, PaidAmount: p.Price, Items: []payment.TransactionItemMutation{{ProductID: p.ID, ProductName: p.Name, UnitPrice: p.Price, Quantity: 1, LineTotal: p.Price}}}
	}
	o := create()
	m := mutation(o)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := pay.CreateTransaction(ctx, actor.ID, m); errs <- e }()
	}
	wg.Wait()
	close(errs)
	wins := 0
	for e := range errs {
		if e == nil {
			wins++
		} else if !errors.Is(e, savedorder.ErrAlreadyPaid) {
			t.Fatal(e)
		}
	}
	if wins != 1 {
		t.Fatal("double payment", wins)
	}
	paid, e := s.Get(ctx, actor.ID, o.ID)
	if e != nil || paid.Status != "paid" || paid.TransactionID == nil {
		t.Fatal("receipt link missing", e)
	}
	var receipts int64
	db.Table("transactions").Count(&receipts)
	if receipts != 1 {
		t.Fatal(receipts)
	}
	db.First(&p, "id = ?", p.ID)
	if p.StockQuantity != 19 {
		t.Fatal("wrong stock", p.StockQuantity)
	}
	for _, op := range []string{"update", "delete"} {
		draft := create()
		mm := mutation(draft)
		ch := make(chan error, 2)
		wg = sync.WaitGroup{}
		wg.Add(2)
		go func() { defer wg.Done(); _, e := pay.CreateTransaction(ctx, actor.ID, mm); ch <- e }()
		go func() {
			defer wg.Done()
			var e error
			if op == "delete" {
				_, e = s.Delete(ctx, actor.ID, draft.ID, uuid.NewString(), 1)
			} else {
				_, e = s.Update(ctx, actor.ID, draft.ID, savedorder.Mutation{RequestID: uuid.NewString(), ExpectedRevision: 1, Label: "Changed", OrderTypeID: ot.ID, Items: draft.Items})
			}
			ch <- e
		}()
		wg.Wait()
		close(ch)
		n := 0
		for e := range ch {
			if e == nil {
				n++
			} else if !errors.Is(e, savedorder.ErrConflict) && !errors.Is(e, savedorder.ErrAlreadyPaid) {
				t.Fatal(e)
			}
		}
		if n != 1 {
			t.Fatalf("%s/payment winners=%d", op, n)
		}
	}
	// A later item with insufficient balance rolls back the earlier deduction and receipt.
	var second catalog.Product
	db.Where("outlet_id = ? AND id <> ?", out, p.ID).First(&second)
	if second.ID == "" {
		second = p
		second.ID = "ffffffff-ffff-ffff-ffff-ffffffffffff" // Locks after the first product.
		second.Name = "Rollback fixture"
		second.TrackStock = false
		second.StockQuantity = 0
		if e := db.Omit("Category", "Variants").Create(&second).Error; e != nil {
			t.Fatal(e)
		}
	}
	db.Model(&catalog.Product{}).Where("id = ?", second.ID).Updates(map[string]any{"track_stock": true, "stock_quantity": 0})
	bad, e := s.Create(ctx, actor.ID, savedorder.Mutation{RequestID: uuid.NewString(), OrderTypeID: ot.ID, Items: []savedorder.Item{{ProductID: p.ID, ProductName: p.Name, UnitPrice: p.Price, Quantity: 1}, {ProductID: second.ID, ProductName: second.Name, UnitPrice: second.Price, Quantity: 1}}})
	if e != nil {
		t.Fatal(e)
	}
	mm := mutation(bad)
	mm.Items = append(mm.Items, payment.TransactionItemMutation{ProductID: second.ID, ProductName: second.Name, UnitPrice: second.Price, Quantity: 1, LineTotal: second.Price})
	mm.Subtotal += second.Price
	mm.TotalAmount = mm.Subtotal
	mm.PaidAmount = mm.TotalAmount
	var before catalog.Product
	db.First(&before, "id = ?", p.ID)
	var countBefore int64
	db.Table("transactions").Count(&countBefore)
	var movementsBefore, movementsAfter int64
	db.Table("stock_movements").Count(&movementsBefore)
	if _, e = pay.CreateTransaction(ctx, actor.ID, mm); !errors.Is(e, catalog.ErrInsufficientStock) {
		t.Fatal("expected stock conflict", e)
	}
	after, _ := s.Get(ctx, actor.ID, bad.ID)
	db.First(&p, "id = ?", p.ID)
	db.Table("transactions").Count(&receipts)
	db.Table("stock_movements").Count(&movementsAfter)
	if after.Status != "open" || p.StockQuantity != before.StockQuantity || receipts != countBefore || movementsBefore != movementsAfter {
		t.Fatal("failed payment leaked writes")
	}
	priceDraft := create()
	if e = db.Model(&catalog.Product{}).Where("id = ?", p.ID).Update("price", p.Price+1).Error; e != nil {
		t.Fatal(e)
	}
	if _, e = pay.CreateTransaction(ctx, actor.ID, mutation(priceDraft)); !errors.Is(e, catalog.ErrCatalogChanged) {
		t.Fatal("stale price accepted", e)
	}
	priceState, e := s.Get(ctx, actor.ID, priceDraft.ID)
	if e != nil || priceState.Status != "open" {
		t.Fatal("price conflict closed draft", e)
	}
}
func TestSavedOrdersPostgres(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	var actor user.User
	db.Where("username = 'admin'").First(&actor)
	var p catalog.Product
	db.Where("outlet_id = ?", *actor.OutletID).First(&p)
	var ot catalog.OrderType
	db.Where("outlet_id = ?", *actor.OutletID).First(&ot)
	repo := savedorder.NewGORMRepository(db)
	s := savedorder.NewService(repo, user.NewGORMRepository(db))
	m := savedorder.Mutation{RequestID: uuid.NewString(), OrderTypeID: ot.ID, Label: "Parked", Items: []savedorder.Item{{ProductID: p.ID, ProductName: p.Name, UnitPrice: p.Price, Quantity: 1}}}
	var wg sync.WaitGroup
	results := make(chan *savedorder.Order, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			copy := m
			copy.Items = append([]savedorder.Item(nil), m.Items...)
			o, e := s.Create(ctx, actor.ID, copy)
			results <- o
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	first := <-results
	second := <-results
	if first.ID != second.ID {
		t.Fatal("duplicate create")
	}
	update := m
	update.RequestID = uuid.NewString()
	update.ExpectedRevision = 1
	update.Label = "Edited"
	updated, e := s.Update(ctx, actor.ID, first.ID, update)
	if e != nil || updated.Revision != 2 {
		t.Fatal("update", e)
	}
	replay, e := s.Create(ctx, actor.ID, m)
	if e != nil || replay.Revision != 1 || replay.Label != "Parked" {
		t.Fatal("immutable replay", e, replay)
	}
	changed := m
	changed.Label = "different"
	if _, e = s.Create(ctx, actor.ID, changed); !errors.Is(e, savedorder.ErrRequestUsed) {
		t.Fatal("changed UUID accepted", e)
	}
	if _, e = s.Update(ctx, actor.ID, first.ID, update); e != nil {
		t.Fatal("update replay", e)
	}
	stale := update
	stale.RequestID = uuid.NewString()
	if _, e = s.Update(ctx, actor.ID, first.ID, stale); !errors.Is(e, savedorder.ErrConflict) {
		t.Fatal("stale revision", e)
	}
	if _, e = repo.Get(ctx, uuid.NewString(), first.ID); !errors.Is(e, savedorder.ErrNotFound) {
		t.Fatal("scope leak", e)
	}
	for _, op := range []string{"update", "delete"} {
		wrong := m
		wrong.RequestID = uuid.NewString()
		wrong.ExpectedRevision = 2
		if _, e = repo.Execute(ctx, uuid.NewString(), actor.ID, op, first.ID, wrong); !errors.Is(e, savedorder.ErrNotFound) {
			t.Fatal("cross-outlet write", op, e)
		}
	}
	invalid := m
	invalid.RequestID = uuid.NewString()
	invalid.Items = append([]savedorder.Item(nil), m.Items...)
	missing := uuid.NewString()
	invalid.Items[0].ProductVariantID = &missing
	if _, e = s.Create(ctx, actor.ID, invalid); !errors.Is(e, savedorder.ErrValidation) {
		t.Fatal("missing variation accepted", e)
	}
	count := 0
	var mu sync.Mutex
	wg = sync.WaitGroup{}
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var e error
			if i == 0 {
				x := update
				x.RequestID = uuid.NewString()
				x.ExpectedRevision = 2
				_, e = s.Update(ctx, actor.ID, first.ID, x)
			} else {
				_, e = s.Delete(ctx, actor.ID, first.ID, uuid.NewString(), 2)
			}
			mu.Lock()
			defer mu.Unlock()
			if e == nil {
				count++
			} else if !errors.Is(e, savedorder.ErrConflict) {
				t.Error(e)
			}
		}(i)
	}
	wg.Wait()
	if count != 1 {
		t.Fatal("concurrent winners", count)
	}
	var receipts, movements int64
	db.Table("transactions").Count(&receipts)
	db.Table("stock_movements").Count(&movements)
	if receipts != 0 || movements != 0 {
		t.Fatal("save mutated paid inventory")
	}
}
