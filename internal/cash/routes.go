package cash

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

// RegisterRoutes mendaftarkan endpoint sesi kas ke Huma.
func RegisterRoutes(api huma.API, handler *Handler) {
	huma.Register(api, huma.Operation{
		OperationID:   "open-cash-session",
		Method:        http.MethodPost,
		Path:          "/cash-sessions",
		DefaultStatus: http.StatusCreated,
		Summary:       "Membuka sesi kas",
		Description:   "Mencatat kas awal Rupiah untuk membuka satu sesi kas pada outlet user. Hanya satu sesi boleh aktif per outlet.",
		Tags:          []string{"Sesi Kas"},
		Errors:        []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusConflict, http.StatusUnprocessableEntity, http.StatusInternalServerError},
	}, handler.Open)

	huma.Register(api, huma.Operation{
		OperationID: "get-current-cash-session",
		Method:      http.MethodGet,
		Path:        "/cash-sessions/current",
		Summary:     "Mengambil sesi kas aktif",
		Description: "Mengambil sesi kas yang sedang aktif pada outlet user terautentikasi.",
		Tags:        []string{"Sesi Kas"},
		Errors:      []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError},
	}, handler.Current)
}
