package sales

import (
	"context"
	"errors"
)

var (
	// ErrUserNotFound menandakan identitas token tidak memiliki user PostgreSQL.
	ErrUserNotFound = errors.New("sales user not found")
	// ErrForbidden menandakan user tidak terhubung ke outlet operasional.
	ErrForbidden = errors.New("sales access is forbidden")
	// ErrValidation menandakan filter daftar tidak valid.
	ErrValidation = errors.New("sales filter is invalid")
)

// Repository mengambil daftar order dan pelanggan dalam scope outlet.
type Repository interface {
	ListOrders(context.Context, OrderFilter) ([]Order, int64, error)
	ListCustomers(context.Context, CustomerFilter) ([]Customer, int64, error)
}
