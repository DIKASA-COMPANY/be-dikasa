package sales

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newSalesRepositoryMock(t *testing.T) (*GORMRepository, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sql mock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open gorm: %v", err)
	}
	return NewGORMRepository(db), mock
}

func TestGORMRepositoryListsFilteredOrdersWithinOutlet(t *testing.T) {
	repository, mock := newSalesRepositoryMock(t)
	filter := OrderFilter{OutletID: "outlet-id", Search: "Jane", OrderTypeID: "type-id", Status: StatusPaid, Page: 1, PageSize: 30}
	countQuery := `SELECT count\(\*\) FROM transactions AS t.*WHERE t\.outlet_id = \$1.*t\.transaction_number ILIKE \$2 OR t\.customer_name ILIKE \$3.*t\.order_type_id = \$4.*t\.status = \$5`
	mock.ExpectQuery(countQuery).WithArgs("outlet-id", "%Jane%", "%Jane%", "type-id", StatusPaid).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	listQuery := `SELECT .* FROM transactions AS t.*WHERE t\.outlet_id = \$1.*ORDER BY t\.paid_at DESC, t\.transaction_number DESC LIMIT \$6`
	mock.ExpectQuery(listQuery).WithArgs("outlet-id", "%Jane%", "%Jane%", "type-id", StatusPaid, 30).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "receipt_number", "customer_id", "customer_name", "ordered_at", "order_type_id", "order_type_code", "order_type_name",
			"restaurant_table_id", "restaurant_table_name", "total_quantity", "total_amount", "status",
		}).AddRow("order-id", "TRX-001", nil, "Jane Doe", time.Date(2026, time.August, 28, 10, 0, 0, 0, time.UTC),
			"type-id", "table_order", "Dine-In", "table-id", "Meja 5", 7, 120_000, StatusPaid))

	values, total, err := repository.ListOrders(context.Background(), filter)

	if err != nil {
		t.Fatalf("list orders: %v", err)
	}
	if total != 1 || len(values) != 1 || values[0].ReceiptNumber != "TRX-001" || values[0].RestaurantTableName == nil {
		t.Fatalf("unexpected orders: total=%d values=%+v", total, values)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestGORMRepositoryListsActiveCustomersWithinOutlet(t *testing.T) {
	repository, mock := newSalesRepositoryMock(t)
	filter := CustomerFilter{OutletID: "outlet-id", Search: "Jane", GenderID: "gender-id", Page: 2, PageSize: 10}
	countQuery := `SELECT count\(\*\) FROM customers AS c.*WHERE \(c\.outlet_id = \$1 AND c\.is_active = \$2\).*c\.name ILIKE \$3 OR c\.phone_number ILIKE \$4 OR c\.email ILIKE \$5.*c\.gender_id = \$6`
	mock.ExpectQuery(countQuery).WithArgs("outlet-id", true, "%Jane%", "%Jane%", "%Jane%", "gender-id").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	listQuery := `SELECT .* FROM customers AS c.*WHERE \(c\.outlet_id = \$1 AND c\.is_active = \$2\).*ORDER BY c\.name ASC, c\.id ASC LIMIT \$7 OFFSET \$8`
	mock.ExpectQuery(listQuery).WithArgs("outlet-id", true, "%Jane%", "%Jane%", "%Jane%", "gender-id", 10, 10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "address", "points", "gender_id", "gender_name", "phone_number", "email", "created_at", "updated_at"}).
			AddRow("customer-id", "Jane Doe", "Bandung", 2321, "gender-id", "female", "08123456789", "jane@example.com", time.Now(), time.Now()))

	values, total, err := repository.ListCustomers(context.Background(), filter)

	if err != nil {
		t.Fatalf("list customers: %v", err)
	}
	if total != 1 || len(values) != 1 || values[0].Name != "Jane Doe" || values[0].GenderName == nil {
		t.Fatalf("unexpected customers: total=%d values=%+v", total, values)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}
