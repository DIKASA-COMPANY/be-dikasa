package savedorder

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// RegisterRoutes adds outlet-scoped drafts beneath the router's /dikasa/v1 group.
func RegisterRoutes(api huma.API, h *Handler) {
	errs := []int{401, 403, 404, 409, 422, 500}
	huma.Register(api, huma.Operation{OperationID: "create-saved-order", Method: http.MethodPost, Path: "/saved-orders", Summary: "Simpan pesanan belum dibayar", Tags: []string{"Pesanan tersimpan"}, Errors: errs}, h.Create)
	huma.Register(api, huma.Operation{OperationID: "list-saved-orders", Method: http.MethodGet, Path: "/saved-orders", Summary: "Daftar pesanan tersimpan outlet", Tags: []string{"Pesanan tersimpan"}, Errors: errs}, h.List)
	huma.Register(api, huma.Operation{OperationID: "get-saved-order", Method: http.MethodGet, Path: "/saved-orders/{saved_order_id}", Summary: "Detail dan status pesanan tersimpan", Tags: []string{"Pesanan tersimpan"}, Errors: errs}, h.Get)
	huma.Register(api, huma.Operation{OperationID: "update-saved-order", Method: http.MethodPut, Path: "/saved-orders/{saved_order_id}", Summary: "Ubah pesanan pada revisi yang diharapkan", Tags: []string{"Pesanan tersimpan"}, Errors: errs}, h.Update)
	huma.Register(api, huma.Operation{OperationID: "delete-saved-order", Method: http.MethodDelete, Path: "/saved-orders/{saved_order_id}", Summary: "Hapus pesanan belum dibayar", Tags: []string{"Pesanan tersimpan"}, Errors: errs}, h.Delete)
}
