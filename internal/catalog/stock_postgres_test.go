package catalog_test

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
	"be-dikasa/internal/user"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestSharedStockPostgres uses a uniquely named temporary schema; no application data is changed.
func TestSharedStockPostgres(t *testing.T) {
	dsn := os.Getenv("DIKASA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("opt in with DIKASA_TEST_DATABASE_URL")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1") {
		t.Fatal("stock integration requires a loopback PostgreSQL URL")
	}
	// pgx accepts query overrides for the network target; exclude them before opening any connection.
	for key := range parsed.Query() {
		if key != "sslmode" && key != "connect_timeout" {
			t.Fatal("stock test URL may only set sslmode and connect_timeout options")
		}
	}
	base, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	baseSQL, _ := base.DB()
	defer baseSQL.Close()
	schema := "dikasa_stock_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err = base.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := base.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Error(err)
		}
	}()
	query := parsed.Query()
	query.Set("search_path", schema+",public")
	parsed.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: parsed.String(), PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	files, _ := filepath.Glob("../../migrations/*.up.sql")
	sort.Strings(files)
	for _, file := range files {
		content, _ := os.ReadFile(file)
		if err = db.Exec(string(content)).Error; err != nil {
			t.Fatalf("migration %s: %v", filepath.Base(file), err)
		}
	}
	if err = devseed.Run(db, "integration-only-password"); err != nil {
		t.Fatal(err)
	}
	var manager user.User
	if err = db.Where("username = ?", "admin").First(&manager).Error; err != nil {
		t.Fatal(err)
	}
	var product catalog.Product
	if err = db.Where("outlet_id = ?", *manager.OutletID).First(&product).Error; err != nil {
		t.Fatal(err)
	}
	stock := catalog.NewStockService(db, user.NewGORMRepository(db))
	enabled := true
	quantity := int64(3)
	threshold := int64(2)
	command := catalog.StockCommand{RequestID: uuid.NewString(), Reason: "Initial stock test", TrackStock: &enabled, InitialQuantity: &quantity, LowStockThreshold: &threshold}
	first, err := stock.Execute(context.Background(), manager.ID, product.ID, "configure", command)
	if err != nil {
		t.Fatal(err)
	}
	again, err := stock.Execute(context.Background(), manager.ID, product.ID, "configure", command)
	if err != nil || again.ID != first.ID {
		t.Fatalf("dedupe failed: %v", err)
	}
	changed := command
	changed.Reason = "different"
	if _, err = stock.Execute(context.Background(), manager.ID, product.ID, "configure", changed); !errors.Is(err, catalog.ErrConflict) {
		t.Fatal("request mutation accepted", err)
	}
	var movements int64
	db.Model(&catalog.StockMovement{}).Count(&movements)
	if movements != 1 {
		t.Fatal(movements)
	}
	// Normal product edits must not overwrite stock columns from an older snapshot.
	repo := catalog.NewGORMRepository(db)
	stale, err := repo.FindProduct(context.Background(), *manager.OutletID, product.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	adjust := catalog.StockCommand{RequestID: uuid.NewString(), Reason: "Restock", QuantityDelta: 2}
	if _, err = stock.Execute(context.Background(), manager.ID, product.ID, "adjust", adjust); err != nil {
		t.Fatal(err)
	}
	stale.Name = "Updated safely"
	if err = repo.UpdateProduct(context.Background(), stale); err != nil {
		t.Fatal(err)
	}
	db.First(&product, "id = ?", product.ID)
	if product.StockQuantity != 5 {
		t.Fatalf("stale edit reset quantity: %d", product.StockQuantity)
	}
	// Reduce to one last unit before two concurrent receipt attempts.
	adjust.RequestID = uuid.NewString()
	adjust.QuantityDelta = -4
	if _, err = stock.Execute(context.Background(), manager.ID, product.ID, "adjust", adjust); err != nil {
		t.Fatal(err)
	}
	var orderType catalog.OrderType
	db.Where("outlet_id = ?", *manager.OutletID).First(&orderType)
	cashID := uuid.NewString()
	if err = db.Exec("INSERT INTO cash_sessions(id,outlet_id,user_id,opening_cash,status,opened_at) VALUES (?,?,?,0,'open',NOW())", cashID, *manager.OutletID, manager.ID).Error; err != nil {
		t.Fatal(err)
	}
	payRepo := payment.NewGORMRepository(db)
	makeReceipt := func() *payment.Transaction {
		return &payment.Transaction{ID: uuid.NewString(), TransactionNumber: uuid.NewString(), OutletID: *manager.OutletID, CashSessionID: cashID, CreatedBy: manager.ID, OrderTypeID: orderType.ID, PaymentMethod: "cash", Status: "paid", Subtotal: 21000, TotalAmount: 21000, PaidAmount: 21000, TotalQuantity: 1, Items: []payment.TransactionItem{{ID: uuid.NewString(), ProductID: product.ID, ProductName: product.Name, UnitPrice: 21000, Quantity: 1, LineTotal: 21000}}}
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var group sync.WaitGroup
	for i := 0; i < 2; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			results <- payRepo.CreateTransaction(context.Background(), makeReceipt())
		}()
	}
	close(start)
	group.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, catalog.ErrInsufficientStock) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("last unit race: %d success, %d conflicts", successes, conflicts)
	}
	db.First(&product, "id = ?", product.ID)
	if product.StockQuantity != 0 {
		t.Fatal(product.StockQuantity)
	}
	var receipts int64
	db.Model(&payment.Transaction{}).Count(&receipts)
	if receipts != 1 {
		t.Fatalf("failed sale persisted: %d", receipts)
	}
	// Force a post-deduction item constraint failure; the deduction and movement must roll back.
	adjust.RequestID = uuid.NewString()
	adjust.QuantityDelta = 2
	stock.Execute(context.Background(), manager.ID, product.ID, "adjust", adjust)
	failed := makeReceipt()
	failed.Items[0].LineTotal = -1
	if err = payRepo.CreateTransaction(context.Background(), failed); err == nil {
		t.Fatal("invalid item accepted")
	}
	db.First(&product, "id = ?", product.ID)
	if product.StockQuantity != 2 {
		t.Fatalf("failed sale changed stock: %d", product.StockQuantity)
	}
}
