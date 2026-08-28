package sales

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// RegisterRoutes mendaftarkan endpoint daftar order dan pelanggan ke Huma.
func RegisterRoutes(api huma.API, handler *Handler) {
	errors := []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusInternalServerError}
	huma.Register(api, huma.Operation{
		OperationID: "list-orders", Method: http.MethodGet, Path: "/orders",
		Summary: "Daftar order outlet", Description: "Mengambil transaksi terbaru sebagai daftar order outlet. Pencarian mencocokkan nomor nota atau nama pelanggan.",
		Tags: []string{"Penjualan"}, Errors: errors,
	}, handler.ListOrders)
	huma.Register(api, huma.Operation{
		OperationID: "list-registered-customers", Method: http.MethodGet, Path: "/customers",
		Summary: "Daftar pelanggan terdaftar", Description: "Mengambil pelanggan aktif outlet. Pencarian mencocokkan nama, nomor telepon, atau email.",
		Tags: []string{"Penjualan"}, Errors: errors,
	}, handler.ListCustomers)
}
