package sales

import "time"

// ListOrdersInput memuat filter pencarian dan pagination daftar order.
type ListOrdersInput struct {
	Search      string `query:"search" maxLength:"100"`
	OrderTypeID string `query:"order_type_id" format:"uuid"`
	Status      string `query:"status" enum:"paid,refunded,cancelled"`
	Page        int    `query:"page" default:"1" minimum:"1"`
	PageSize    int    `query:"page_size" default:"30" minimum:"1" maximum:"100"`
}

// ListCustomersInput memuat filter pencarian dan pagination pelanggan terdaftar.
type ListCustomersInput struct {
	Search   string `query:"search" maxLength:"100"`
	GenderID string `query:"gender_id" format:"uuid"`
	Page     int    `query:"page" default:"1" minimum:"1"`
	PageSize int    `query:"page_size" default:"30" minimum:"1" maximum:"100"`
}

// OrderListTypeData adalah label jenis order yang ditampilkan pada daftar order.
type OrderListTypeData struct {
	ID   string `json:"id" format:"uuid"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// RestaurantTableData adalah meja opsional pada order dine-in.
type RestaurantTableData struct {
	ID   string `json:"id" format:"uuid"`
	Name string `json:"name"`
}

// OrderData adalah satu baris daftar order.
type OrderData struct {
	ID              string               `json:"id" format:"uuid"`
	ReceiptNumber   string               `json:"receipt_number"`
	CustomerID      *string              `json:"customer_id,omitempty" format:"uuid"`
	CustomerName    string               `json:"customer_name"`
	OrderedAt       time.Time            `json:"ordered_at" format:"date-time"`
	OrderType       OrderListTypeData    `json:"order_type"`
	RestaurantTable *RestaurantTableData `json:"restaurant_table,omitempty"`
	TotalQuantity   int                  `json:"total_quantity" minimum:"1"`
	TotalAmount     int64                `json:"total_amount" minimum:"0"`
	Status          string               `json:"status" enum:"paid,refunded,cancelled"`
}

// GenderData adalah gender opsional pelanggan.
type GenderData struct {
	ID   string `json:"id" format:"uuid"`
	Name string `json:"name"`
}

// CustomerData adalah satu baris pelanggan terdaftar.
type CustomerData struct {
	ID          string      `json:"id" format:"uuid"`
	Name        string      `json:"name"`
	Address     string      `json:"address"`
	Points      int64       `json:"points" minimum:"0"`
	Gender      *GenderData `json:"gender,omitempty"`
	PhoneNumber string      `json:"phone_number"`
	Email       *string     `json:"email,omitempty" format:"email"`
	CreatedAt   time.Time   `json:"created_at" format:"date-time"`
	UpdatedAt   time.Time   `json:"updated_at" format:"date-time"`
}

// PaginationMeta menjelaskan halaman list yang dikembalikan.
type PaginationMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// OrdersOutput membungkus halaman daftar order untuk Huma.
type OrdersOutput struct {
	Body struct {
		Data []OrderData    `json:"data"`
		Meta PaginationMeta `json:"meta"`
	}
}

// CustomersOutput membungkus halaman pelanggan terdaftar untuk Huma.
type CustomersOutput struct {
	Body struct {
		Data []CustomerData `json:"data"`
		Meta PaginationMeta `json:"meta"`
	}
}
