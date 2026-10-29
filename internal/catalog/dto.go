package catalog

import "github.com/danielgtaylor/huma/v2"

// ProductData adalah representasi produk untuk client POS.
type ProductData struct {
	ID                   string               `json:"id" format:"uuid"`
	Name                 string               `json:"name"`
	Price                int64                `json:"price" minimum:"0"`
	SellingPrice         int64                `json:"selling_price" minimum:"0"`
	OnlineSellingPrice   *int64               `json:"online_selling_price,omitempty" minimum:"0"`
	CostPrice            *int64               `json:"cost_price,omitempty" minimum:"0"`
	OnlineCostPrice      *int64               `json:"online_cost_price,omitempty" minimum:"0"`
	StockStatus          string               `json:"stock_status" enum:"available,out_of_stock"`
	TrackStock           bool                 `json:"track_stock"`
	StockQuantity        int64                `json:"stock_quantity" minimum:"0"`
	LowStockThreshold    int64                `json:"low_stock_threshold" minimum:"0"`
	EffectiveStockStatus string               `json:"effective_stock_status" enum:"available,low_stock,out_of_stock"`
	Description          string               `json:"description"`
	ImageURL             *string              `json:"image_url,omitempty" format:"uri" doc:"Supabase Storage public URL; render directly and do not proxy through the API"`
	IsHighlighted        bool                 `json:"is_highlighted"`
	SortOrder            int                  `json:"sort_order"`
	IsActive             bool                 `json:"is_active"`
	Category             CategoryData         `json:"category"`
	ProductTypeCode      string               `json:"product_type" enum:"food,drink,additional"`
	Variations           []ProductVariantData `json:"variations"`
}

// ProductVariantData adalah variasi harga produk untuk client.
type ProductVariantData struct {
	ID                 string `json:"id" format:"uuid"`
	Name               string `json:"name"`
	SellingPrice       int64  `json:"selling_price" minimum:"0"`
	OnlineSellingPrice *int64 `json:"online_selling_price,omitempty" minimum:"0"`
	CostPrice          *int64 `json:"cost_price,omitempty" minimum:"0"`
	OnlineCostPrice    *int64 `json:"online_cost_price,omitempty" minimum:"0"`
	StockStatus        string `json:"stock_status" enum:"available,out_of_stock"`
	SortOrder          int    `json:"sort_order"`
	IsActive           bool   `json:"is_active"`
}

// ProductVariantRequest adalah item JSON pada field multipart variations.
type ProductVariantRequest struct {
	Name               string `json:"name" maxLength:"100"`
	SellingPrice       int64  `json:"selling_price" minimum:"0"`
	OnlineSellingPrice *int64 `json:"online_selling_price,omitempty" minimum:"0"`
	CostPrice          *int64 `json:"cost_price,omitempty" minimum:"0"`
	OnlineCostPrice    *int64 `json:"online_cost_price,omitempty" minimum:"0"`
	StockStatus        string `json:"stock_status" enum:"available,out_of_stock"`
	SortOrder          int    `json:"sort_order"`
}

// ProductTypeData adalah item tab tipe produk global.
type ProductTypeData struct {
	ID        string `json:"id" format:"uuid"`
	Code      string `json:"code" enum:"food,drink,additional"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
}

// CategoryData adalah item dropdown kategori outlet.
type CategoryData struct {
	ID        string `json:"id" format:"uuid"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	IsActive  bool   `json:"is_active"`
}

// OrderTypeData adalah item dropdown jenis order outlet.
type OrderTypeData struct {
	ID        string  `json:"id" format:"uuid"`
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	IconKey   *string `json:"icon_key,omitempty"`
	SortOrder int     `json:"sort_order"`
	IsActive  bool    `json:"is_active"`
}

// ProductForm adalah field multipart untuk create dan update produk.
type ProductForm struct {
	Name               string                  `form:"name" required:"true" maxLength:"150"`
	CategoryID         string                  `form:"category_id" required:"true" format:"uuid"`
	ProductType        string                  `form:"product_type" required:"true" enum:"food,drink,additional"`
	SellingPrice       int64                   `form:"selling_price" required:"true" minimum:"0"`
	OnlineSellingPrice int64                   `form:"online_selling_price" required:"false" minimum:"0"`
	CostPrice          int64                   `form:"cost_price" required:"false" minimum:"0"`
	OnlineCostPrice    int64                   `form:"online_cost_price" required:"false" minimum:"0"`
	StockStatus        string                  `form:"stock_status" required:"true" enum:"available,out_of_stock"`
	Variations         []ProductVariantRequest `form:"variations" required:"false" contentType:"application/json"`
	Description        string                  `form:"description" required:"false"`
	IsHighlighted      bool                    `form:"is_highlighted" required:"false"`
	SortOrder          int                     `form:"sort_order" required:"false"`
	IsActive           bool                    `form:"is_active" required:"false"`
	RemoveImage        bool                    `form:"remove_image" required:"false"`
	Image              huma.FormFile           `form:"image" required:"false" contentType:"image/png,image/jpeg"`
}

// ProductPatchForm adalah field multipart opsional untuk PATCH produk.
type ProductPatchForm struct {
	Name               string                  `form:"name" required:"false" maxLength:"150"`
	CategoryID         string                  `form:"category_id" required:"false" format:"uuid"`
	ProductType        string                  `form:"product_type" required:"false" enum:"food,drink,additional"`
	SellingPrice       int64                   `form:"selling_price" required:"false" minimum:"0"`
	OnlineSellingPrice int64                   `form:"online_selling_price" required:"false" minimum:"0"`
	CostPrice          int64                   `form:"cost_price" required:"false" minimum:"0"`
	OnlineCostPrice    int64                   `form:"online_cost_price" required:"false" minimum:"0"`
	StockStatus        string                  `form:"stock_status" required:"false" enum:"available,out_of_stock"`
	Variations         []ProductVariantRequest `form:"variations" required:"false" contentType:"application/json"`
	Description        string                  `form:"description" required:"false"`
	IsHighlighted      bool                    `form:"is_highlighted" required:"false"`
	SortOrder          int                     `form:"sort_order" required:"false"`
	IsActive           bool                    `form:"is_active" required:"false"`
	RemoveImage        bool                    `form:"remove_image" required:"false"`
	Image              huma.FormFile           `form:"image" required:"false" contentType:"image/png,image/jpeg"`
}

// ListProductsInput adalah query filter endpoint list produk.
type ListProductsInput struct {
	ProductType         string `query:"product_type" enum:"food,drink,additional"`
	CategoryID          string `query:"category_id" format:"uuid"`
	SearchByProductName string `query:"searchByProductName" maxLength:"100"`
	Page                int    `query:"page" default:"1" minimum:"1"`
	PageSize            int    `query:"page_size" default:"30" minimum:"1" maximum:"100"`
	IncludeInactive     bool   `query:"include_inactive" default:"false"`
}

// ProductIDInput adalah path dan opsi detail produk.
type ProductIDInput struct {
	ProductID       string `path:"product_id" format:"uuid"`
	IncludeInactive bool   `query:"include_inactive" default:"false"`
}

// ProductMutationInput adalah multipart update produk berdasarkan ID.
type ProductMutationInput struct {
	ProductID string `path:"product_id" format:"uuid"`
	RawBody   huma.MultipartFormFiles[ProductPatchForm]
}

// ProductCreateInput adalah multipart pembuatan produk.
type ProductCreateInput struct {
	RawBody huma.MultipartFormFiles[ProductForm]
}

// ProductDeleteInput adalah path soft delete produk.
type ProductDeleteInput struct {
	ProductID string `path:"product_id" format:"uuid"`
}

// ProductTypesOutput membungkus daftar tipe produk.
type ProductTypesOutput struct {
	Body struct {
		Data []ProductTypeData `json:"data"`
	}
}

// ProductOutput membungkus satu produk.
type ProductOutput struct {
	Status int
	Body   struct {
		Data ProductData `json:"data"`
	}
}

// ProductsOutput membungkus halaman produk dan metadata pagination.
type ProductsOutput struct {
	Body struct {
		Data []ProductData `json:"data"`
		Meta struct {
			Page     int   `json:"page"`
			PageSize int   `json:"page_size"`
			Total    int64 `json:"total"`
		} `json:"meta"`
	}
}

// ProductTypePathInput adalah tipe produk pada path dan opsi data nonaktif.
type ProductTypePathInput struct {
	ProductType     string `path:"product_type" enum:"food,drink,additional"`
	IncludeInactive bool   `query:"include_inactive" default:"false"`
}

// CategoryRequest adalah payload kategori.
type CategoryRequest struct {
	Name      string `json:"name" validate:"required,max=100"`
	SortOrder int    `json:"sort_order"`
	IsActive  bool   `json:"is_active"`
}

// CategoryCreateInput adalah request pembuatan kategori.
type CategoryCreateInput struct {
	ProductType string `path:"product_type" enum:"food,drink,additional"`
	Body        CategoryRequest
}

// CategoryMutationInput adalah request perubahan kategori.
type CategoryMutationInput struct {
	ProductType string `path:"product_type" enum:"food,drink,additional"`
	CategoryID  string `path:"category_id" format:"uuid"`
	Body        CategoryRequest
}

// CategoryDeleteInput adalah path soft delete kategori.
type CategoryDeleteInput struct {
	ProductType string `path:"product_type" enum:"food,drink,additional"`
	CategoryID  string `path:"category_id" format:"uuid"`
}

// CategoryOutput membungkus satu kategori.
type CategoryOutput struct {
	Status int
	Body   struct {
		Data CategoryData `json:"data"`
	}
}

// CategoriesOutput membungkus daftar kategori.
type CategoriesOutput struct {
	Body struct {
		Data []CategoryData `json:"data"`
	}
}

// ListOrderTypesInput adalah opsi list jenis order.
type ListOrderTypesInput struct {
	IncludeInactive bool `query:"include_inactive" default:"false"`
}

// OrderTypeRequest adalah payload jenis order.
type OrderTypeRequest struct {
	Code      string  `json:"code" validate:"required,max=50"`
	Name      string  `json:"name" validate:"required,max=100"`
	IconKey   *string `json:"icon_key,omitempty"`
	SortOrder int     `json:"sort_order"`
	IsActive  bool    `json:"is_active"`
}

// OrderTypeCreateInput adalah request pembuatan jenis order.
type OrderTypeCreateInput struct{ Body OrderTypeRequest }

// OrderTypeMutationInput adalah request perubahan jenis order.
type OrderTypeMutationInput struct {
	OrderTypeID string `path:"order_type_id" format:"uuid"`
	Body        OrderTypeRequest
}

// OrderTypeDeleteInput adalah path soft delete jenis order.
type OrderTypeDeleteInput struct {
	OrderTypeID string `path:"order_type_id" format:"uuid"`
}

// OrderTypeOutput membungkus satu jenis order.
type OrderTypeOutput struct {
	Status int
	Body   struct {
		Data OrderTypeData `json:"data"`
	}
}

// OrderTypesOutput membungkus daftar jenis order.
type OrderTypesOutput struct {
	Body struct {
		Data []OrderTypeData `json:"data"`
	}
}

// NoContentOutput menghasilkan response tanpa body.
type NoContentOutput struct{ Status int }
