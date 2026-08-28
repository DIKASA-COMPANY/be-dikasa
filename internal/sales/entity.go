// Package sales menyediakan daftar order dan pelanggan yang terisolasi per outlet.
package sales

import "time"

const (
	// StatusPaid menandakan order sudah dibayar.
	StatusPaid = "paid"
	// StatusRefunded menandakan pembayaran order sudah dikembalikan.
	StatusRefunded = "refunded"
	// StatusCancelled menandakan order dibatalkan.
	StatusCancelled = "cancelled"
)

// Order adalah proyeksi transaksi untuk layar daftar order.
type Order struct {
	ID                  string
	ReceiptNumber       string
	CustomerID          *string
	CustomerName        string
	OrderedAt           time.Time
	OrderTypeID         string
	OrderTypeCode       string
	OrderTypeName       string
	RestaurantTableID   *string
	RestaurantTableName *string
	TotalQuantity       int
	TotalAmount         int64
	Status              string
}

// Customer adalah pelanggan terdaftar milik satu outlet.
type Customer struct {
	ID          string
	Name        string
	Address     string
	Points      int64
	GenderID    *string
	GenderName  *string
	PhoneNumber string
	Email       *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// OrderFilter mengendalikan pencarian, filter, dan pagination order.
type OrderFilter struct {
	OutletID    string
	Search      string
	OrderTypeID string
	Status      string
	Page        int
	PageSize    int
}

// CustomerFilter mengendalikan pencarian, filter gender, dan pagination pelanggan.
type CustomerFilter struct {
	OutletID string
	Search   string
	GenderID string
	Page     int
	PageSize int
}

// OrderPage adalah satu halaman hasil daftar order.
type OrderPage struct {
	Data     []Order
	Page     int
	PageSize int
	Total    int64
}

// CustomerPage adalah satu halaman hasil daftar pelanggan.
type CustomerPage struct {
	Data     []Customer
	Page     int
	PageSize int
	Total    int64
}
