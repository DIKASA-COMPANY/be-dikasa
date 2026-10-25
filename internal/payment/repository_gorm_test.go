package payment

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newPaymentRepositoryMock(t *testing.T) (*GORMRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sql mock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, PreferSimpleProtocol: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open gorm: %v", err)
	}
	return NewGORMRepository(db), mock
}

func TestGORMRepositoryFindsActiveOutletTaxPercentage(t *testing.T) {
	repository, mock := newPaymentRepositoryMock(t)
	mock.ExpectQuery(`SELECT .*tax_percentage.* FROM "outlets" WHERE id = \$1 AND is_active = \$2.*LIMIT \$3`).
		WithArgs("outlet-id", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"tax_percentage"}).AddRow(11.5))

	taxPercentage, err := repository.FindTaxPercentageByOutletID(context.Background(), "outlet-id")

	if err != nil {
		t.Fatalf("find tax percentage: %v", err)
	}
	if taxPercentage != 11.5 {
		t.Fatalf("expected tax percentage 11.5, got %v", taxPercentage)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestGORMRepositoryMapsMissingOrInactiveOutlet(t *testing.T) {
	repository, mock := newPaymentRepositoryMock(t)
	mock.ExpectQuery(`SELECT .*tax_percentage.* FROM "outlets"`).
		WithArgs("outlet-id", true, 1).
		WillReturnRows(sqlmock.NewRows([]string{"tax_percentage"}))

	_, err := repository.FindTaxPercentageByOutletID(context.Background(), "outlet-id")

	if !errors.Is(err, ErrOutletNotFound) {
		t.Fatalf("expected ErrOutletNotFound, got %v", err)
	}
}

func TestGORMRepositoryListsActiveWalletsInDisplayOrder(t *testing.T) {
	repository, mock := newPaymentRepositoryMock(t)
	mock.ExpectQuery(`SELECT \* FROM "payment_wallets" WHERE outlet_id = \$1 AND is_active = \$2 ORDER BY sort_order ASC, name ASC`).
		WithArgs("outlet-id", true).
		WillReturnRows(sqlmock.NewRows([]string{"id", "outlet_id", "code", "name", "icon_key", "sort_order", "is_active"}).
			AddRow("wallet-id", "outlet-id", "gopay", "GoPay", "gopay", 10, true))

	wallets, err := repository.ListWallets(context.Background(), "outlet-id", false)

	if err != nil {
		t.Fatalf("list wallets: %v", err)
	}
	if len(wallets) != 1 || wallets[0].Code != "gopay" || wallets[0].Name != "GoPay" {
		t.Fatalf("unexpected wallets: %+v", wallets)
	}
}

func TestGORMRepositoryCreatesTransactionAndItemsAtomically(t *testing.T) {
	repository, mock := newPaymentRepositoryMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .*FROM "cash_sessions".*FOR SHARE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("cash-session-id"))
	mock.ExpectExec(`INSERT INTO "transactions"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT .*FROM "products".*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id", "outlet_id", "category_id", "price", "stock_status", "is_active"}).AddRow("product-id", "outlet-id", "category-id", 21000, "available", true))
	mock.ExpectQuery(`SELECT .*FROM "product_categories"`).WillReturnRows(sqlmock.NewRows([]string{"id", "is_active"}).AddRow("category-id", true))
	mock.ExpectQuery(`SELECT .*FROM "product_variants"`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT id,category_id FROM "products"`).WillReturnRows(sqlmock.NewRows([]string{"id", "category_id"}).AddRow("product-id", "category-id"))
	mock.ExpectQuery(`SELECT .*FROM "product_categories".*FOR SHARE`).WillReturnRows(sqlmock.NewRows([]string{"id", "outlet_id", "name", "is_active"}).AddRow("category-id", "outlet-id", "Food", true))
	mock.ExpectExec(`INSERT INTO "transaction_items"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	transaction := &Transaction{
		ID: "transaction-id", TransactionNumber: "TRX-001", OutletID: "outlet-id", CashSessionID: "cash-session-id",
		CreatedBy: "user-id", OrderTypeID: "order-type-id", PaymentMethod: PaymentMethodCash,
		Subtotal: 21_000, TotalAmount: 21_000, PaidAmount: 21_000, TotalQuantity: 1, Status: TransactionStatusPaid,
		Items: []TransactionItem{{ID: "item-id", OutletID: "outlet-id", ProductID: "product-id", ProductName: "Nasi Goreng", UnitPrice: 21_000, Quantity: 1, LineTotal: 21_000}},
	}

	err := repository.CreateTransaction(context.Background(), transaction)

	if err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	if transaction.Items[0].TransactionID != "transaction-id" {
		t.Fatalf("item transaction ID not assigned: %+v", transaction.Items[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestGORMRepositoryRollsBackWhenTransactionItemFails(t *testing.T) {
	repository, mock := newPaymentRepositoryMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .*FROM "cash_sessions".*FOR SHARE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("cash-session-id"))
	mock.ExpectExec(`INSERT INTO "transactions"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT .*FROM "products".*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id", "outlet_id", "category_id", "price", "stock_status", "is_active"}).AddRow("product-id", "outlet-id", "category-id", 0, "available", true))
	mock.ExpectQuery(`SELECT .*FROM "product_categories"`).WillReturnRows(sqlmock.NewRows([]string{"id", "is_active"}).AddRow("category-id", true))
	mock.ExpectQuery(`SELECT .*FROM "product_variants"`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT id,category_id FROM "products"`).WillReturnRows(sqlmock.NewRows([]string{"id", "category_id"}).AddRow("product-id", "category-id"))
	mock.ExpectQuery(`SELECT .*FROM "product_categories".*FOR SHARE`).WillReturnRows(sqlmock.NewRows([]string{"id", "outlet_id", "name", "is_active"}).AddRow("category-id", "outlet-id", "Food", true))
	mock.ExpectExec(`INSERT INTO "transaction_items"`).WillReturnError(errors.New("item insert failed"))
	mock.ExpectRollback()
	transaction := &Transaction{
		ID: "transaction-id", TransactionNumber: "TRX-001", OutletID: "outlet-id", CashSessionID: "cash-session-id",
		CreatedBy: "user-id", OrderTypeID: "order-type-id", PaymentMethod: PaymentMethodCash,
		Items: []TransactionItem{{ID: "item-id", OutletID: "outlet-id", ProductID: "product-id", ProductName: "Produk", Quantity: 1}},
	}

	err := repository.CreateTransaction(context.Background(), transaction)

	if err == nil {
		t.Fatal("expected item insert failure")
	}
	if expectationErr := mock.ExpectationsWereMet(); expectationErr != nil {
		t.Fatalf("unmet SQL expectations: %v", expectationErr)
	}
}
