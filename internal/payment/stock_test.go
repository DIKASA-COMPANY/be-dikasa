package payment

import (
	"be-dikasa/internal/catalog"
	"context"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func TestCheckoutRollsBackWhenCatalogPriceChanged(t *testing.T) {
	repo, mock := newPaymentRepositoryMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .*FROM "cash_sessions".*FOR SHARE`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("cash-session-id"))
	mock.ExpectExec(`INSERT INTO "transactions"`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(`SELECT .*FROM "products".*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id", "outlet_id", "category_id", "price", "stock_status", "is_active"}).AddRow("p", "o", "c", 99, "available", true))
	mock.ExpectQuery(`SELECT .*FROM "product_categories"`).WillReturnRows(sqlmock.NewRows([]string{"id", "is_active"}).AddRow("c", true))
	mock.ExpectQuery(`SELECT .*FROM "product_variants"`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectRollback()
	err := repo.CreateTransaction(context.Background(), &Transaction{ID: "t", OutletID: "o", Items: []TransactionItem{{ProductID: "p", UnitPrice: 100, Quantity: 1}}})
	if !errors.Is(err, catalog.ErrCatalogChanged) {
		t.Fatalf("expected catalog conflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
