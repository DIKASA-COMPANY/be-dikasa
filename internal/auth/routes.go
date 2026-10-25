package auth

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// RegisterRoutes mendaftarkan endpoint auth ke Huma.
func RegisterRoutes(api huma.API, handler *Handler) {
	huma.Register(api, huma.Operation{
		OperationID: "login",
		Method:      http.MethodPost,
		Path:        "/login",
		Summary:     "Masuk ke aplikasi",
		Description: "Memvalidasi kredensial PostgreSQL lalu menerbitkan Firebase ID token atau local JWT sesuai AUTH_MODE. Local token berlaku satu jam tanpa refresh; login ulang setelah kedaluwarsa.",
		Tags:        []string{"Autentikasi"},
		Security:    []map[string][]string{},
		Errors: []int{
			http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden,
			http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity,
			http.StatusInternalServerError, http.StatusBadGateway,
		},
		MaxBodyBytes: 1 << 20,
	}, handler.Login)
}
