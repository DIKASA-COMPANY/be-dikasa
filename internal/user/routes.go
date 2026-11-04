package user

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// RegisterRoutes mendaftarkan endpoint user ke Huma.
func RegisterRoutes(api huma.API, handler *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "get-current-user",
		Method:      http.MethodGet,
		Path:        "/users/me",
		Summary:     "Mengambil profil user aktif",
		Description: "Mengambil profil PostgreSQL berdasarkan identitas Supabase access token.",
		Tags:        []string{"User"},
		Errors:      []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError},
	}, handler.Me)
	huma.Register(api, huma.Operation{OperationID: "create-avatar-upload-intent", Method: http.MethodPost, Path: "/users/me/avatar-upload-intents", DefaultStatus: http.StatusCreated, Summary: "Buat signed upload URL avatar", Description: "Backend membuat object_name milik user dan signed upload URL Supabase yang berlaku 2 jam. Client mengunggah langsung ke Supabase lalu mengirim avatar_object_name pada PATCH profil.", Tags: []string{"User"}, Errors: []int{401, 404, 413, 422, 502, 503, 500}}, handler.PrepareAvatarUpload)
	huma.Register(api, huma.Operation{OperationID: "update-current-user", Method: http.MethodPatch, Path: "/users/me", Summary: "Memperbarui profil sendiri", Description: "Multipart parsial tidak menerima byte avatar. Gunakan avatar_object_name dari direct upload Supabase; remove_avatar menghapus avatar. Omit mempertahankan nilai; string kosong menghapus field opsional. Username, role serta outlet read-only.", Tags: []string{"User"}, Errors: []int{401, 404, 409, 413, 422, 502, 503, 500}}, handler.UpdateProfile)
	huma.Register(api, huma.Operation{OperationID: "change-current-user-pin", Method: http.MethodPatch, Path: "/users/me/pin", DefaultStatus: http.StatusNoContent, Summary: "Mengubah PIN akun", Description: "PIN terdiri dari enam digit. current_pin wajib bila akun sudah memiliki PIN.", Tags: []string{"User"}, Errors: []int{401, 404, 409, 422, 503, 500}}, handler.ChangePIN)
	huma.Register(api, huma.Operation{OperationID: "change-current-user-password", Method: http.MethodPatch, Path: "/users/me/password", DefaultStatus: http.StatusNoContent, Summary: "Mengubah password akun", Description: "Memverifikasi password lama lalu menyimpan bcrypt hash PostgreSQL baru (8–72 byte). Hash disinkronkan ke Supabase Auth pada login berikutnya; access token yang sudah terbit tetap berlaku sampai kedaluwarsa. Respons hilang mungkin berarti perubahan tersimpan; jangan retry otomatis.", Tags: []string{"User"}, Errors: []int{401, 404, 409, 422, 503, 500}}, handler.ChangePassword)
	huma.Register(api, huma.Operation{OperationID: "list-profile-genders", Method: http.MethodGet, Path: "/users/me/genders", Summary: "Pilihan gender profil", Tags: []string{"User"}, Errors: []int{401, 500}}, handler.Genders)
}
