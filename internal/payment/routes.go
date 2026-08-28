package payment

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// RegisterRoutes mendaftarkan endpoint konfigurasi pembayaran ke Huma.
func RegisterRoutes(api huma.API, handler *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "get-payment-tax-percentage",
		Method:      http.MethodGet,
		Path:        "/payments/tax-percentage",
		Summary:     "Mengambil persentase pajak pembayaran",
		Description: "Mengambil persentase pajak terbaru milik outlet user. FE menghitung nominal pajak dan total pembayaran dari nilai ini.",
		Tags:        []string{"Pembayaran"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError},
	}, handler.TaxPercentage)

	huma.Register(api, huma.Operation{
		OperationID: "list-payment-wallets", Method: http.MethodGet, Path: "/wallets",
		Summary: "Mengambil pilihan wallet", Description: "Mengambil wallet aktif milik outlet untuk dropdown pembayaran. include_inactive hanya tersedia untuk manager.",
		Tags: []string{"Pembayaran"}, Errors: []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusInternalServerError},
	}, handler.ListWallets)

	huma.Register(api, huma.Operation{
		OperationID: "create-payment-wallet", Method: http.MethodPost, Path: "/wallets", DefaultStatus: http.StatusCreated,
		Summary: "Membuat wallet", Description: "Manager membuat pilihan wallet baru untuk outletnya.",
		Tags: []string{"Pembayaran"}, Errors: []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusConflict, http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}, handler.CreateWallet)

	huma.Register(api, huma.Operation{
		OperationID: "update-payment-wallet", Method: http.MethodPatch, Path: "/wallets/{wallet_id}",
		Summary: "Memperbarui wallet", Description: "Manager memperbarui nama, icon, urutan, atau status wallet outlet.",
		Tags: []string{"Pembayaran"}, Errors: []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}, handler.UpdateWallet)

	huma.Register(api, huma.Operation{
		OperationID: "delete-payment-wallet", Method: http.MethodDelete, Path: "/wallets/{wallet_id}", DefaultStatus: http.StatusNoContent,
		Summary: "Menonaktifkan wallet", Description: "Manager melakukan soft delete wallet agar transaksi lama tetap dapat diaudit.",
		Tags: []string{"Pembayaran"}, Errors: []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError},
	}, handler.DeleteWallet)

	huma.Register(api, huma.Operation{
		OperationID: "create-transaction", Method: http.MethodPost, Path: "/transactions", DefaultStatus: http.StatusCreated,
		Summary: "Menyimpan transaksi pembayaran", Description: "Menyimpan nominal hasil kalkulasi FE dan snapshot seluruh item secara atomik pada sesi kas aktif. Backend memvalidasi konsistensi tanpa query agregasi cart.",
		Tags: []string{"Pembayaran"}, Errors: []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}, handler.CreateTransaction)
}
