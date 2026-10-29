package cash

import "github.com/danielgtaylor/huma/v2"

// RegisterMovementRoutes stays within the parent authenticated /dikasa/v1 API.
func RegisterMovementRoutes(api huma.API, h *MovementHandler) {
	errs := []int{401, 403, 404, 409, 413, 422, 500, 502, 503}
	huma.Register(api, huma.Operation{OperationID: "cash-activity", Method: "GET", Path: "/cash-sessions/{session_id}/activity", Summary: "Aktivitas tunai satu sesi", Description: "Total penuh sesi, halaman event terpisah. Kas awal + penjualan tunai - refund tunai + kas masuk - kas keluar. Sesi tertutup memakai ringkasan tersimpan.", Tags: []string{"Kas Kasir"}, Errors: errs}, h.Activity)
	huma.Register(api, huma.Operation{OperationID: "create-cash-movement", Method: "POST", Path: "/cash-sessions/{session_id}/movements", DefaultStatus: 201, Summary: "Catat kas masuk/keluar", Description: "Kasir pembuka atau manager. UUID terikat aktor/sesi/body/foto; replay identik tidak menambah entri. Bukti PNG/JPEG opsional <=3 MiB. Pengeluaran tidak boleh melebihi kas diharapkan. Respons hilang: pulihkan atau kirim ulang intent asli, jangan UUID baru.", Tags: []string{"Kas Kasir"}, Errors: errs}, h.Create)
	huma.Register(api, huma.Operation{OperationID: "recover-cash-movement", Method: "GET", Path: "/cash-sessions/{session_id}/movements/by-request/{request_id}", Summary: "Pulihkan permintaan kas sendiri", Tags: []string{"Kas Kasir"}, Errors: errs}, h.ByRequest)
	huma.Register(api, huma.Operation{OperationID: "cash-movement-detail", Method: "GET", Path: "/cash-sessions/{session_id}/movements/{movement_id}", Summary: "Rincian entri kas", Tags: []string{"Kas Kasir"}, Errors: errs}, h.Detail)
	huma.Register(api, huma.Operation{OperationID: "reverse-cash-movement", Method: "POST", Path: "/cash-sessions/{session_id}/movements/{movement_id}/reverse", DefaultStatus: 201, Summary: "Koreksi entri kas dengan entri lawan", Description: "Kasir pembuka: entri sendiri; manager: entri outlet. Sesi wajib terbuka, satu koreksi per original, nominal/arah diturunkan server. Original tidak diubah.", Tags: []string{"Kas Kasir"}, Errors: errs}, h.Reverse)
	huma.Register(api, huma.Operation{OperationID: "cash-movement-proof", Method: "GET", Path: "/cash-sessions/{session_id}/movements/{movement_id}/proof", Summary: "URL bukti kas", Description: "Wajib autentikasi dan outlet cocok. Mengembalikan Supabase Storage public URL agar frontend merender gambar langsung tanpa proxy file melalui API.", Tags: []string{"Kas Kasir"}, Errors: errs}, h.Proof)
}
