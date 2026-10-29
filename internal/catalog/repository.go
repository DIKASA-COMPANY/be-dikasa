package catalog

import (
	"context"
	"errors"
)

var (
	// ErrNotFound menandakan data katalog tidak ada pada outlet user.
	ErrNotFound = errors.New("catalog data not found")
	// ErrConflict menandakan constraint unik atau relasi database bertabrakan.
	ErrConflict = errors.New("catalog data conflicts with existing data")
	// ErrValidation menandakan input katalog tidak memenuhi aturan domain.
	ErrValidation = errors.New("catalog input is invalid")
	// ErrForbidden menandakan operasi hanya tersedia untuk manager.
	ErrForbidden = errors.New("catalog access is forbidden")
	// ErrUserWithoutOutlet menandakan user tidak mempunyai scope outlet POS.
	ErrUserWithoutOutlet = errors.New("catalog user is not assigned to an outlet")
	// ErrCategoryMismatch menandakan kategori tidak cocok dengan tipe atau outlet produk.
	ErrCategoryMismatch = errors.New("product category does not match its outlet or product type")
	// ErrCategoryInUse mencegah kategori dinonaktifkan saat masih dipakai produk aktif.
	ErrCategoryInUse = errors.New("category still has active products")
	// ErrStorageDisabled menandakan upload sengaja belum diaktifkan.
	ErrStorageDisabled = errors.New("image storage is disabled")
	// ErrImageTooLarge menandakan file melewati batas tiga MiB.
	ErrImageTooLarge = errors.New("image exceeds maximum size")
	// ErrInvalidImage menandakan MIME, signature, extension, atau decode gambar tidak valid.
	ErrInvalidImage = errors.New("image is not a valid png or jpeg")
	// ErrStorage menandakan Supabase Storage gagal melakukan operasi.
	ErrStorage = errors.New("image storage operation failed")
)

// Repository mendefinisikan operasi persistence katalog.
type Repository interface {
	ListProductTypes(context.Context) ([]ProductType, error)
	FindProductType(context.Context, string) (*ProductType, error)
	ListProducts(context.Context, ProductFilter) ([]Product, int64, error)
	FindProduct(context.Context, string, string, bool) (*Product, error)
	CreateProduct(context.Context, *Product) error
	UpdateProduct(context.Context, *Product) error
	SoftDeleteProduct(context.Context, string, string) error
	FindCategory(context.Context, string, string) (*ProductCategory, error)
	ListCategories(context.Context, string, string, bool) ([]ProductCategory, error)
	CreateCategory(context.Context, *ProductCategory) error
	UpdateCategory(context.Context, *ProductCategory) error
	SoftDeleteCategory(context.Context, string, string) error
	CategoryHasActiveProducts(context.Context, string, string) (bool, error)
	ListOrderTypes(context.Context, string, bool) ([]OrderType, error)
	FindOrderType(context.Context, string, string) (*OrderType, error)
	CreateOrderType(context.Context, *OrderType) error
	UpdateOrderType(context.Context, *OrderType) error
	SoftDeleteOrderType(context.Context, string, string) error
}

// ImageStore saves images and deletes them through their persisted Supabase URL.
type ImageStore interface {
	Save(context.Context, string, string, *ImageUpload) (*StoredImage, error)
	Delete(context.Context, string) error
}
