package savedorder

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCheckoutLocksAndRejectsPaid(t *testing.T) {
	sql, mock, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	defer sql.Close()
	db, e := gorm.Open(postgres.New(postgres.Config{Conn: sql}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	mock.ExpectQuery(`SELECT .*FROM "saved_orders".*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id", "outlet_id", "status", "revision", "created_by"}).AddRow("draft", "outlet", "paid", 2, "actor"))
	mock.ExpectQuery(`SELECT.*FROM "saved_order_items"`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`SELECT.*FROM "users"`).WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("Cashier"))
	_, e = LockForCheckout(context.Background(), db, "outlet", "draft", 1, "type", nil)
	if !errors.Is(e, ErrAlreadyPaid) {
		t.Fatal(e)
	}
	if e = mock.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
