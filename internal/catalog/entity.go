// Package catalog mengelola katalog POS dan jenis order yang terisolasi per outlet.
package catalog

import (
	"io"
	"time"
)

const (
	// StockStatusAvailable menandakan produk dapat dipesan.
	StockStatusAvailable = "available"
	// StockStatusOutOfStock menandakan produk sedang habis.
	StockStatusOutOfStock = "out_of_stock"
)

// ProductType adalah master global kelompok produk.
type ProductType struct {
	ID        string    `gorm:"type:uuid;primaryKey"`
	Code      string    `gorm:"size:30;uniqueIndex;not null"`
	Name      string    `gorm:"size:60;not null"`
	SortOrder int       `gorm:"not null"`
	IsActive  bool      `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

// TableName menetapkan tabel master tipe produk.
func (ProductType) TableName() string { return "product_types" }

// ProductCategory adalah kategori produk milik satu outlet dan satu tipe produk.
type ProductCategory struct {
	ID            string      `gorm:"type:uuid;primaryKey"`
	OutletID      string      `gorm:"type:uuid;not null;index"`
	ProductTypeID string      `gorm:"type:uuid;not null;index"`
	Name          string      `gorm:"size:100;not null"`
	SortOrder     int         `gorm:"not null"`
	IsActive      bool        `gorm:"not null"`
	CreatedAt     time.Time   `gorm:"not null"`
	UpdatedAt     time.Time   `gorm:"not null"`
	ProductType   ProductType `gorm:"foreignKey:ProductTypeID"`
}

// TableName menetapkan tabel kategori produk.
func (ProductCategory) TableName() string { return "product_categories" }

// Product adalah item katalog POS milik outlet.
type Product struct {
	ID                 string           `gorm:"type:uuid;primaryKey"`
	OutletID           string           `gorm:"type:uuid;not null;index"`
	CategoryID         string           `gorm:"type:uuid;not null;index"`
	Name               string           `gorm:"size:150;not null"`
	Price              int64            `gorm:"not null"`
	OnlineSellingPrice *int64           `gorm:"column:online_selling_price"`
	CostPrice          *int64           `gorm:"column:cost_price"`
	OnlineCostPrice    *int64           `gorm:"column:online_cost_price"`
	StockStatus        string           `gorm:"size:20;not null"`
	Description        string           `gorm:"type:text;not null"`
	ImageURL           *string          `gorm:"type:text"`
	ImageObjectName    *string          `gorm:"type:text"`
	IsHighlighted      bool             `gorm:"not null"`
	SortOrder          int              `gorm:"not null"`
	IsActive           bool             `gorm:"not null"`
	CreatedAt          time.Time        `gorm:"not null"`
	UpdatedAt          time.Time        `gorm:"not null"`
	Category           ProductCategory  `gorm:"foreignKey:CategoryID"`
	Variants           []ProductVariant `gorm:"foreignKey:ProductID"`
	ReplaceVariants    bool             `gorm:"-"`
}

// TableName menetapkan tabel produk.
func (Product) TableName() string { return "products" }

// ProductVariant adalah pilihan ukuran atau varian harga dalam satu produk.
type ProductVariant struct {
	ID                 string    `gorm:"type:uuid;primaryKey"`
	ProductID          string    `gorm:"type:uuid;not null;index"`
	OutletID           string    `gorm:"type:uuid;not null;index"`
	Name               string    `gorm:"size:100;not null"`
	Price              int64     `gorm:"not null"`
	OnlineSellingPrice *int64    `gorm:"column:online_selling_price"`
	CostPrice          *int64    `gorm:"column:cost_price"`
	OnlineCostPrice    *int64    `gorm:"column:online_cost_price"`
	StockStatus        string    `gorm:"size:20;not null"`
	SortOrder          int       `gorm:"not null"`
	IsActive           bool      `gorm:"not null"`
	CreatedAt          time.Time `gorm:"not null"`
	UpdatedAt          time.Time `gorm:"not null"`
}

// TableName menetapkan tabel variasi produk.
func (ProductVariant) TableName() string { return "product_variants" }

// OrderType adalah pilihan jenis order milik outlet.
type OrderType struct {
	ID        string    `gorm:"type:uuid;primaryKey"`
	OutletID  string    `gorm:"type:uuid;not null;index"`
	Code      string    `gorm:"size:50;not null"`
	Name      string    `gorm:"size:100;not null"`
	IconKey   *string   `gorm:"size:100"`
	SortOrder int       `gorm:"not null"`
	IsActive  bool      `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

// TableName menetapkan tabel jenis order.
func (OrderType) TableName() string { return "order_types" }

// ImageUpload adalah file gambar yang akan divalidasi dan disimpan.
type ImageUpload struct {
	Reader      io.ReadSeeker
	Size        int64
	ContentType string
	Filename    string
}

// StoredImage adalah referensi object storage yang persisten.
type StoredImage struct {
	URL        string
	ObjectName string
}

// ProductMutation berisi input create atau patch produk.
type ProductMutation struct {
	Name, CategoryID, ProductType, Description     string
	Price                                          int64
	OnlineSellingPrice, CostPrice, OnlineCostPrice *int64
	StockStatus                                    string
	Variations                                     []ProductVariantMutation
	IsHighlighted, IsActive                        bool
	SortOrder                                      int
	Image                                          *ImageUpload
	RemoveImage                                    bool
	Fields                                         map[string]bool
}

// ProductVariantMutation adalah input variasi pada multipart produk.
type ProductVariantMutation struct {
	Name                                           string
	Price                                          int64
	OnlineSellingPrice, CostPrice, OnlineCostPrice *int64
	StockStatus                                    string
	SortOrder                                      int
	IsActive                                       bool
}

// ProductFilter mengendalikan filter dan pagination list produk.
type ProductFilter struct {
	OutletID, ProductType, CategoryID, SearchByProductName string
	Page, PageSize                                         int
	IncludeInactive                                        bool
}

// CategoryMutation adalah input perubahan kategori.
type CategoryMutation struct {
	Name      string
	SortOrder int
	IsActive  bool
}

// OrderTypeMutation adalah input perubahan jenis order.
type OrderTypeMutation struct {
	Code, Name string
	IconKey    *string
	SortOrder  int
	IsActive   bool
}
