package catalog

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// GORMRepository menyimpan katalog menggunakan PostgreSQL/GORM.
type GORMRepository struct{ db *gorm.DB }

// NewGORMRepository membuat repository katalog.
func NewGORMRepository(db *gorm.DB) *GORMRepository { return &GORMRepository{db: db} }

// ListProductTypes mengambil master tipe produk aktif dengan urutan stabil.
func (repository *GORMRepository) ListProductTypes(ctx context.Context) ([]ProductType, error) {
	var values []ProductType
	err := repository.db.WithContext(ctx).Where("is_active = ?", true).Order("sort_order ASC, name ASC").Find(&values).Error
	return values, err
}

// FindProductType mengambil satu tipe produk global aktif berdasarkan code.
func (repository *GORMRepository) FindProductType(ctx context.Context, code string) (*ProductType, error) {
	var value ProductType
	err := repository.db.WithContext(ctx).Where("LOWER(code) = LOWER(?) AND is_active = ?", strings.TrimSpace(code), true).First(&value).Error
	return &value, normalizeRepositoryError(err)
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}

func productQuery(db *gorm.DB, filter ProductFilter) *gorm.DB {
	query := db.Model(&Product{}).
		Joins("JOIN product_categories ON product_categories.id = products.category_id").
		Joins("JOIN product_types ON product_types.id = product_categories.product_type_id").
		Where("products.outlet_id = ?", filter.OutletID)
	if !filter.IncludeInactive {
		query = query.Where("products.is_active = ?", true)
	}
	if filter.ProductType != "" {
		query = query.Where("product_types.code = ?", strings.ToLower(strings.TrimSpace(filter.ProductType)))
	}
	if filter.CategoryID != "" {
		query = query.Where("products.category_id = ?", filter.CategoryID)
	}
	if filter.SearchByProductName != "" {
		query = query.Where(`LOWER(products.name) LIKE ? ESCAPE '\'`, "%"+strings.ToLower(escapeLike(filter.SearchByProductName))+"%")
	}
	return query
}

// ListProducts mengambil satu halaman produk dan total hasil dalam scope outlet.
func (repository *GORMRepository) ListProducts(ctx context.Context, filter ProductFilter) ([]Product, int64, error) {
	query := productQuery(repository.db.WithContext(ctx), filter)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []Product{}, 0, nil
	}
	var values []Product
	err := query.Preload("Category.ProductType").Preload("Variants", "is_active = ?", true).Order("products.sort_order ASC, products.name ASC").
		Offset((filter.Page - 1) * filter.PageSize).Limit(filter.PageSize).Find(&values).Error
	return values, total, err
}

// FindProduct mengambil produk hanya dari outlet yang diminta.
func (repository *GORMRepository) FindProduct(ctx context.Context, outletID, id string, includeInactive bool) (*Product, error) {
	var value Product
	query := repository.db.WithContext(ctx).Preload("Category.ProductType").Preload("Variants", func(db *gorm.DB) *gorm.DB {
		return db.Order("sort_order ASC, name ASC")
	}).Where("products.outlet_id = ? AND products.id = ?", outletID, id)
	if !includeInactive {
		query = query.Where("products.is_active = ?", true)
	}
	err := query.First(&value).Error
	return &value, normalizeRepositoryError(err)
}

// CreateProduct menyimpan produk tanpa menulis ulang struct relasi.
func (repository *GORMRepository) CreateProduct(ctx context.Context, value *Product) error {
	err := repository.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		if err := transaction.Omit("Category", "Variants").Create(value).Error; err != nil {
			return err
		}
		if len(value.Variants) > 0 {
			return transaction.Create(&value.Variants).Error
		}
		return nil
	})
	return normalizeRepositoryError(err)
}

// UpdateProduct memperbarui field produk yang dikelola manager.
func (repository *GORMRepository) UpdateProduct(ctx context.Context, value *Product) error {
	err := repository.db.WithContext(ctx).Transaction(func(transaction *gorm.DB) error {
		result := transaction.Model(&Product{}).Where("id = ? AND outlet_id = ?", value.ID, value.OutletID).Updates(map[string]any{
			"category_id": value.CategoryID, "name": value.Name, "price": value.Price, "online_selling_price": value.OnlineSellingPrice,
			"cost_price": value.CostPrice, "online_cost_price": value.OnlineCostPrice, "stock_status": value.StockStatus,
			"description": value.Description, "image_url": value.ImageURL, "image_object_name": value.ImageObjectName,
			"is_highlighted": value.IsHighlighted, "sort_order": value.SortOrder, "is_active": value.IsActive,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		if value.ReplaceVariants {
			if err := transaction.Model(&ProductVariant{}).Where("product_id = ? AND outlet_id = ? AND is_active = ?", value.ID, value.OutletID, true).Update("is_active", false).Error; err != nil {
				return err
			}
			if len(value.Variants) > 0 {
				return transaction.Create(&value.Variants).Error
			}
		}
		return nil
	})
	return normalizeRepositoryError(err)
}

// SoftDeleteProduct menonaktifkan produk tanpa menghapus gambar maupun record.
func (repository *GORMRepository) SoftDeleteProduct(ctx context.Context, outletID, id string) error {
	result := repository.db.WithContext(ctx).Model(&Product{}).Where("id = ? AND outlet_id = ? AND is_active = ?", id, outletID, true).Update("is_active", false)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// FindCategory mengambil kategori dan tipe produk pada scope outlet.
func (repository *GORMRepository) FindCategory(ctx context.Context, outletID, id string) (*ProductCategory, error) {
	var value ProductCategory
	err := repository.db.WithContext(ctx).Preload("ProductType").Where("product_categories.outlet_id = ? AND product_categories.id = ?", outletID, id).First(&value).Error
	return &value, normalizeRepositoryError(err)
}

// ListCategories mengambil kategori terurut untuk satu outlet dan tipe produk.
func (repository *GORMRepository) ListCategories(ctx context.Context, outletID, productType string, includeInactive bool) ([]ProductCategory, error) {
	var values []ProductCategory
	query := repository.db.WithContext(ctx).Model(&ProductCategory{}).Preload("ProductType").Joins("JOIN product_types ON product_types.id = product_categories.product_type_id").
		Where("product_categories.outlet_id = ? AND product_types.code = ?", outletID, strings.ToLower(strings.TrimSpace(productType)))
	if !includeInactive {
		query = query.Where("product_categories.is_active = ? AND product_types.is_active = ?", true, true)
	}
	err := query.Order("product_categories.sort_order ASC, product_categories.name ASC").Find(&values).Error
	return values, err
}

// CreateCategory menyimpan kategori baru.
func (repository *GORMRepository) CreateCategory(ctx context.Context, value *ProductCategory) error {
	return normalizeRepositoryError(repository.db.WithContext(ctx).Omit("ProductType").Create(value).Error)
}

// UpdateCategory memperbarui nama, urutan, dan status kategori.
func (repository *GORMRepository) UpdateCategory(ctx context.Context, value *ProductCategory) error {
	result := repository.db.WithContext(ctx).Model(&ProductCategory{}).Where("id = ? AND outlet_id = ?", value.ID, value.OutletID).Updates(map[string]any{"name": value.Name, "sort_order": value.SortOrder, "is_active": value.IsActive})
	if result.Error != nil {
		return normalizeRepositoryError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// SoftDeleteCategory menonaktifkan kategori pada satu outlet.
func (repository *GORMRepository) SoftDeleteCategory(ctx context.Context, outletID, id string) error {
	result := repository.db.WithContext(ctx).Model(&ProductCategory{}).Where("id = ? AND outlet_id = ? AND is_active = ?", id, outletID, true).Update("is_active", false)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// CategoryHasActiveProducts memeriksa konflik sebelum kategori dinonaktifkan.
func (repository *GORMRepository) CategoryHasActiveProducts(ctx context.Context, outletID, categoryID string) (bool, error) {
	var count int64
	err := repository.db.WithContext(ctx).Model(&Product{}).Where("outlet_id = ? AND category_id = ? AND is_active = ?", outletID, categoryID, true).Count(&count).Error
	return count > 0, err
}

// ListOrderTypes mengambil jenis order outlet dengan urutan stabil.
func (repository *GORMRepository) ListOrderTypes(ctx context.Context, outletID string, includeInactive bool) ([]OrderType, error) {
	var values []OrderType
	query := repository.db.WithContext(ctx).Where("outlet_id = ?", outletID)
	if !includeInactive {
		query = query.Where("is_active = ?", true)
	}
	err := query.Order("sort_order ASC, name ASC").Find(&values).Error
	return values, err
}

// FindOrderType mengambil jenis order pada scope outlet.
func (repository *GORMRepository) FindOrderType(ctx context.Context, outletID, id string) (*OrderType, error) {
	var value OrderType
	err := repository.db.WithContext(ctx).Where("outlet_id = ? AND id = ?", outletID, id).First(&value).Error
	return &value, normalizeRepositoryError(err)
}

// CreateOrderType menyimpan jenis order baru.
func (repository *GORMRepository) CreateOrderType(ctx context.Context, value *OrderType) error {
	return normalizeRepositoryError(repository.db.WithContext(ctx).Create(value).Error)
}

// UpdateOrderType memperbarui jenis order outlet.
func (repository *GORMRepository) UpdateOrderType(ctx context.Context, value *OrderType) error {
	result := repository.db.WithContext(ctx).Model(&OrderType{}).Where("id = ? AND outlet_id = ?", value.ID, value.OutletID).Updates(map[string]any{"code": value.Code, "name": value.Name, "icon_key": value.IconKey, "sort_order": value.SortOrder, "is_active": value.IsActive})
	if result.Error != nil {
		return normalizeRepositoryError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// SoftDeleteOrderType menonaktifkan jenis order tanpa menghapus record.
func (repository *GORMRepository) SoftDeleteOrderType(ctx context.Context, outletID, id string) error {
	result := repository.db.WithContext(ctx).Model(&OrderType{}).Where("id = ? AND outlet_id = ? AND is_active = ?", id, outletID, true).Update("is_active", false)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func normalizeRepositoryError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && (postgresError.Code == "23505" || postgresError.Code == "23503") {
		return ErrConflict
	}
	return err
}
