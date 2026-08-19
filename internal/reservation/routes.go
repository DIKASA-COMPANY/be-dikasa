package reservation

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// RegisterRoutes mendaftarkan resource jenis reservasi, meja, dan reservasi.
func RegisterRoutes(api huma.API, handler *Handler) {
	readErrors := []int{401, 403, 404, 422, 500}
	writeErrors := []int{401, 403, 404, 409, 422, 500}
	huma.Register(api, huma.Operation{OperationID: "list-reservation-types", Method: http.MethodGet, Path: "/reservation-types", Summary: "Daftar jenis reservasi", Tags: []string{"Reservasi"}, Errors: readErrors}, handler.ListTypes)
	huma.Register(api, huma.Operation{OperationID: "list-restaurant-tables", Method: http.MethodGet, Path: "/restaurant-tables", Summary: "Daftar meja restoran", Description: "reservation_at opsional untuk hanya menampilkan meja yang tersedia pada waktu persis tersebut.", Tags: []string{"Reservasi"}, Errors: readErrors}, handler.ListTables)
	huma.Register(api, huma.Operation{OperationID: "create-restaurant-table", Method: http.MethodPost, Path: "/restaurant-tables", DefaultStatus: http.StatusCreated, Summary: "Membuat meja restoran", Description: "Khusus manager.", Tags: []string{"Reservasi"}, Errors: writeErrors}, handler.CreateTable)
	huma.Register(api, huma.Operation{OperationID: "update-restaurant-table", Method: http.MethodPatch, Path: "/restaurant-tables/{table_id}", Summary: "Memperbarui meja restoran", Description: "Khusus manager.", Tags: []string{"Reservasi"}, Errors: writeErrors}, handler.UpdateTable)
	huma.Register(api, huma.Operation{OperationID: "delete-restaurant-table", Method: http.MethodDelete, Path: "/restaurant-tables/{table_id}", DefaultStatus: http.StatusNoContent, Summary: "Menonaktifkan meja restoran", Description: "Khusus manager; soft delete.", Tags: []string{"Reservasi"}, Errors: writeErrors}, handler.DeleteTable)
	huma.Register(api, huma.Operation{OperationID: "list-reservations", Method: http.MethodGet, Path: "/reservations", Summary: "Daftar reservasi outlet", Tags: []string{"Reservasi"}, Errors: readErrors}, handler.List)
	huma.Register(api, huma.Operation{OperationID: "get-reservation", Method: http.MethodGet, Path: "/reservations/{reservation_id}", Summary: "Detail reservasi", Tags: []string{"Reservasi"}, Errors: readErrors}, handler.Get)
	huma.Register(api, huma.Operation{OperationID: "create-reservation", Method: http.MethodPost, Path: "/reservations", DefaultStatus: http.StatusCreated, Summary: "Membuat reservasi", Description: "Manager dan cashier dapat membuat reservasi. Flag print/send disimpan sebagai instruksi dan tidak menjalankan provider eksternal.", Tags: []string{"Reservasi"}, Errors: writeErrors}, handler.Create)
	huma.Register(api, huma.Operation{OperationID: "update-reservation", Method: http.MethodPatch, Path: "/reservations/{reservation_id}", Summary: "Memperbarui reservasi", Tags: []string{"Reservasi"}, Errors: writeErrors}, handler.Update)
	huma.Register(api, huma.Operation{OperationID: "cancel-reservation", Method: http.MethodDelete, Path: "/reservations/{reservation_id}", DefaultStatus: http.StatusNoContent, Summary: "Membatalkan reservasi", Description: "Mengubah status menjadi cancelled dan melakukan soft delete.", Tags: []string{"Reservasi"}, Errors: writeErrors}, handler.Cancel)
}
