package catalog

import (
	"context"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sort"
)

// SaleCategory is the immutable category identity/name captured at checkout.
type SaleCategory struct{ ID, Name string }

// CaptureSaleCategories runs after DeductSaleStock, whose parent locks remain held.
// Category shared locks serialize names/activation with concurrent catalog edits.
func CaptureSaleCategories(ctx context.Context, tx *gorm.DB, outlet string, items []StockSaleItem) (map[string]SaleCategory, error) {
	ids := map[string]bool{}
	for _, i := range items {
		ids[i.ProductID] = true
	}
	keys := []string{}
	for id := range ids {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	result := map[string]SaleCategory{}
	if len(keys) == 0 {
		return result, nil
	}
	var parents []Product
	if e := tx.WithContext(ctx).Select("id,category_id").Where("outlet_id=? AND id IN ?", outlet, keys).Find(&parents).Error; e != nil {
		return nil, e
	}
	if len(parents) != len(keys) {
		return nil, ErrCatalogChanged
	}
	categories := map[string]bool{}
	for _, p := range parents {
		categories[p.CategoryID] = true
	}
	keys = nil
	for id := range categories {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	found := map[string]SaleCategory{}
	for _, id := range keys {
		var c ProductCategory
		if e := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "SHARE"}).Where("id=? AND outlet_id=? AND is_active=true", id, outlet).First(&c).Error; e != nil {
			if e == gorm.ErrRecordNotFound {
				return nil, ErrCatalogChanged
			}
			return nil, e
		}
		found[id] = SaleCategory{c.ID, c.Name}
	}
	for _, p := range parents {
		result[p.ID] = found[p.CategoryID]
	}
	return result, nil
}
