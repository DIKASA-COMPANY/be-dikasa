package migrations

import (
	"os"
	"strings"
	"testing"
)

func TestStockMigrationPreservesExistingProducts(t *testing.T) {
	content, err := os.ReadFile("000009_add_product_stock.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(content))
	for _, part := range []string{"track_stock boolean not null default false", "stock_quantity bigint not null default 0", "low_stock_threshold", "create table", "stock_movements", "request_id", "foreign key", "check"} {
		if !strings.Contains(sql, part) {
			t.Fatalf("missing %s", part)
		}
	}
	if strings.Contains(sql, "drop table") || strings.Contains(sql, "delete from") {
		t.Fatal("destructive stock migration")
	}
}
