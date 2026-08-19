package catalog

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// RegisterRoutes mendaftarkan resource katalog tanpa prefix /list dan tanpa resource food/drink terpisah.
func RegisterRoutes(api huma.API, handler *Handler) {
	readErrors := []int{401, 403, 404, 422, 500}
	writeErrors := []int{401, 403, 404, 409, 413, 422, 502, 503, 500}
	huma.Register(api, huma.Operation{OperationID: "list-product-types", Method: http.MethodGet, Path: "/product-types", Summary: "Daftar tipe produk", Tags: []string{"Katalog"}, Errors: readErrors}, handler.ListProductTypes)
	huma.Register(api, huma.Operation{OperationID: "list-products", Method: http.MethodGet, Path: "/products", Summary: "Daftar produk POS", Description: "User outlet melihat produk aktif. Mencari nama produk secara case-insensitive dan partial melalui searchByProductName; include_inactive hanya untuk manager.", Tags: []string{"Katalog"}, Errors: readErrors}, handler.ListProducts)
	huma.Register(api, huma.Operation{OperationID: "get-product", Method: http.MethodGet, Path: "/products/{product_id}", Summary: "Detail produk", Tags: []string{"Katalog"}, Errors: readErrors}, handler.GetProduct)
	huma.Register(api, huma.Operation{OperationID: "create-product", Method: http.MethodPost, Path: "/products", DefaultStatus: http.StatusCreated, Summary: "Membuat produk", Description: "Khusus manager. Multipart memakai product_type, selling_price, stock_status, harga online/modal opsional, variations JSON opsional, dan gambar PNG/JPEG opsional maksimal 3 MiB.", Tags: []string{"Katalog"}, Errors: writeErrors}, handler.CreateProduct)
	huma.Register(api, huma.Operation{OperationID: "update-product", Method: http.MethodPatch, Path: "/products/{product_id}", Summary: "Memperbarui produk", Description: "Khusus manager. Multipart parsial dapat mengganti daftar variations, mengganti image, atau menghapus image dengan remove_image.", Tags: []string{"Katalog"}, Errors: writeErrors}, handler.UpdateProduct)
	huma.Register(api, huma.Operation{OperationID: "delete-product", Method: http.MethodDelete, Path: "/products/{product_id}", DefaultStatus: http.StatusNoContent, Summary: "Menonaktifkan produk", Description: "Khusus manager. Soft delete mempertahankan object gambar.", Tags: []string{"Katalog"}, Errors: writeErrors}, handler.DeleteProduct)
	huma.Register(api, huma.Operation{OperationID: "list-product-categories", Method: http.MethodGet, Path: "/product-types/{product_type}/categories", Summary: "Daftar kategori sesuai tipe produk", Tags: []string{"Katalog"}, Errors: readErrors}, handler.ListCategories)
	huma.Register(api, huma.Operation{OperationID: "create-product-category", Method: http.MethodPost, Path: "/product-types/{product_type}/categories", DefaultStatus: http.StatusCreated, Summary: "Membuat kategori", Description: "Khusus manager pada outlet terautentikasi.", Tags: []string{"Katalog"}, Errors: writeErrors}, handler.CreateCategory)
	huma.Register(api, huma.Operation{OperationID: "update-product-category", Method: http.MethodPatch, Path: "/product-types/{product_type}/categories/{category_id}", Summary: "Memperbarui kategori", Description: "Khusus manager. Kategori dengan produk aktif tidak dapat dinonaktifkan.", Tags: []string{"Katalog"}, Errors: writeErrors}, handler.UpdateCategory)
	huma.Register(api, huma.Operation{OperationID: "delete-product-category", Method: http.MethodDelete, Path: "/product-types/{product_type}/categories/{category_id}", DefaultStatus: http.StatusNoContent, Summary: "Menonaktifkan kategori", Description: "Khusus manager; ditolak bila kategori masih memiliki produk aktif.", Tags: []string{"Katalog"}, Errors: writeErrors}, handler.DeleteCategory)
	huma.Register(api, huma.Operation{OperationID: "list-order-types", Method: http.MethodGet, Path: "/order-types", Summary: "Daftar jenis order", Tags: []string{"Jenis Order"}, Errors: readErrors}, handler.ListOrderTypes)
	huma.Register(api, huma.Operation{OperationID: "create-order-type", Method: http.MethodPost, Path: "/order-types", DefaultStatus: http.StatusCreated, Summary: "Membuat jenis order", Description: "Khusus manager pada outlet terautentikasi.", Tags: []string{"Jenis Order"}, Errors: writeErrors}, handler.CreateOrderType)
	huma.Register(api, huma.Operation{OperationID: "update-order-type", Method: http.MethodPatch, Path: "/order-types/{order_type_id}", Summary: "Memperbarui jenis order", Description: "Khusus manager pada outlet pemilik data.", Tags: []string{"Jenis Order"}, Errors: writeErrors}, handler.UpdateOrderType)
	huma.Register(api, huma.Operation{OperationID: "delete-order-type", Method: http.MethodDelete, Path: "/order-types/{order_type_id}", DefaultStatus: http.StatusNoContent, Summary: "Menonaktifkan jenis order", Description: "Khusus manager; operasi ini melakukan soft delete.", Tags: []string{"Jenis Order"}, Errors: writeErrors}, handler.DeleteOrderType)
}
