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
		Description: "Memvalidasi kredensial PostgreSQL, memigrasikan akun lama bila perlu, lalu menerbitkan access dan refresh token Supabase.",
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
