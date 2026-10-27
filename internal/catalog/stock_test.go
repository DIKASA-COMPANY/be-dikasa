package catalog

import (
	"context"
	"errors"
	"testing"
)

func TestInventoryOperationsRequireManagerBeforeDatabaseAccess(t *testing.T) {
	service := NewStockService(nil, fakeAccountFinder{account: account("cashier")})
	if _, err := service.Execute(context.Background(), "cashier", "product", "adjust", StockCommand{}); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	if _, _, err := service.Movements(context.Background(), "cashier", "product", 1, 20); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
}

func TestDerivedStockAndSharedSaleQuantities(t *testing.T) {
	p := Product{StockStatus: "available", TrackStock: true, StockQuantity: 3, LowStockThreshold: 3}
	if p.EffectiveStockStatus() != "low_stock" {
		t.Fatal(p.EffectiveStockStatus())
	}
	p.StockQuantity = 0
	if p.EffectiveStockStatus() != "out_of_stock" {
		t.Fatal("zero available")
	}
	p.TrackStock = false
	if p.EffectiveStockStatus() != "available" {
		t.Fatal("untracked zero blocked")
	}
	p.StockStatus = "out_of_stock"
	if p.EffectiveStockStatus() != "out_of_stock" {
		t.Fatal("manual pause ignored")
	}
	p = Product{ID: "p", Price: 100, StockStatus: "available", IsActive: true, TrackStock: true, StockQuantity: 3, Category: ProductCategory{IsActive: true}, Variants: []ProductVariant{{ID: "v", ProductID: "p", Price: 200, StockStatus: "available", IsActive: true}}}
	variant := "v"
	items := []StockSaleItem{{ProductID: "p", UnitPrice: 100, Quantity: 1}, {ProductID: "p", VariantID: &variant, UnitPrice: 200, Quantity: 2}}
	if quantity, err := validateStockSale(p, items); err != nil || quantity != 3 {
		t.Fatalf("%d %v", quantity, err)
	}
	items[1].Quantity = 3
	if _, err := validateStockSale(p, items); !errors.Is(err, ErrInsufficientStock) {
		t.Fatal(err)
	}
	items[1].Quantity = 1
	items[1].UnitPrice = 199
	if _, err := validateStockSale(p, items); !errors.Is(err, ErrCatalogChanged) {
		t.Fatal(err)
	}
	items[1].UnitPrice = 200
	wrong := "foreign"
	items[1].VariantID = &wrong
	if _, err := validateStockSale(p, items); !errors.Is(err, ErrCatalogChanged) {
		t.Fatal(err)
	}
}

func TestStockConfigurationCannotResetLiveBalance(t *testing.T) {
	enabled := true
	initial := int64(9)
	threshold := int64(2)
	p := Product{}
	if err := applyStockCommand(&p, "configure", StockCommand{TrackStock: &enabled, InitialQuantity: &initial, LowStockThreshold: &threshold}); err != nil {
		t.Fatal(err)
	}
	if p.StockQuantity != 9 || !p.TrackStock {
		t.Fatal(p)
	}
	if err := applyStockCommand(&p, "configure", StockCommand{TrackStock: &enabled, InitialQuantity: &initial}); !errors.Is(err, ErrConflict) {
		t.Fatal("stale configuration reset stock")
	}
	if err := applyStockCommand(&p, "adjust", StockCommand{QuantityDelta: -10}); !errors.Is(err, ErrInsufficientStock) {
		t.Fatal(err)
	}
	if err := applyStockCommand(&p, "adjust", StockCommand{QuantityDelta: -3}); err != nil || p.StockQuantity != 6 {
		t.Fatal(err)
	}
}
