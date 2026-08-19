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
		Description: "Mengambil profil PostgreSQL berdasarkan UID pada Firebase ID token.",
		Tags:        []string{"User"},
		Errors:      []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusInternalServerError},
	}, handler.Me)
	huma.Register(api, huma.Operation{OperationID: "update-current-user", Method: http.MethodPatch, Path: "/users/me", Summary: "Memperbarui profil sendiri", Description: "Multipart parsial untuk nama, nomor HP, email, dan avatar PNG/JPEG opsional maksimal 3 MiB. Role serta outlet tetap read-only.", Tags: []string{"User"}, Errors: []int{401, 404, 409, 413, 422, 502, 503, 500}}, handler.UpdateProfile)
	huma.Register(api, huma.Operation{OperationID: "change-current-user-pin", Method: http.MethodPatch, Path: "/users/me/pin", DefaultStatus: http.StatusNoContent, Summary: "Mengubah PIN akun", Description: "PIN terdiri dari enam digit. current_pin wajib bila akun sudah memiliki PIN.", Tags: []string{"User"}, Errors: []int{401, 404, 422, 500}}, handler.ChangePIN)
	huma.Register(api, huma.Operation{OperationID: "change-current-user-password", Method: http.MethodPatch, Path: "/users/me/password", DefaultStatus: http.StatusNoContent, Summary: "Mengubah password akun", Description: "Memverifikasi password lama lalu menyimpan bcrypt hash password baru.", Tags: []string{"User"}, Errors: []int{401, 404, 422, 500}}, handler.ChangePassword)
}
