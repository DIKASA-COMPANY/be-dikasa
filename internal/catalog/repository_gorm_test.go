package catalog

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestRepositoryProductSearchEscapesSQLWildcardsAndOrders(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db, PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	repository := NewGORMRepository(gormDB)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT count(*) FROM "products" JOIN product_categories ON product_categories.id = products.category_id JOIN product_types ON product_types.id = product_categories.product_type_id WHERE products.outlet_id = $1 AND products.is_active = $2 AND product_types.code = $3 AND LOWER(products.name) LIKE $4 ESCAPE '\'`)).
		WithArgs("outlet-a", true, "food", `%50\%\_off%`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	_, _, err = repository.ListProducts(context.Background(), ProductFilter{OutletID: "outlet-a", ProductType: "food", SearchByProductName: `50%_OFF`, Page: 1, PageSize: 30})
	if err != nil {
		t.Fatalf("list products: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestEscapeLikePreservesLiteralSearchMeaning(t *testing.T) {
	tests := map[string]string{
		"Nasi":    "Nasi",
		`50%_OFF`: `50\%\_OFF`,
		`a\b%c_d`: `a\\b\%c\_d`,
		"":        "",
	}
	for input, expected := range tests {
		if actual := escapeLike(input); actual != expected {
			t.Errorf("escapeLike(%q) = %q, want %q", input, actual, expected)
		}
	}
}
