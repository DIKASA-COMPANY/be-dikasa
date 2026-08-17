// Package errors mendefinisikan general error API berbahasa Indonesia.
package errors

import "net/http"

// Body adalah detail error yang aman dikirim kepada client.
type Body struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// Response adalah envelope JSON general error.
type Response struct {
	Error Body `json:"error"`
}

// StatusError adalah error terstruktur yang dapat dikembalikan handler Huma.
type StatusError struct {
	Status int  `json:"-"`
	Body   Body `json:"error"`
}

// Error memenuhi interface error menggunakan pesan aman untuk pengguna.
func (statusError *StatusError) Error() string { return statusError.Body.Message }

// GetStatus mengembalikan status HTTP untuk Huma.
func (statusError *StatusError) GetStatus() int { return statusError.Status }

var catalog = map[int]Body{
	http.StatusBadRequest:           {Code: "PERMINTAAN_TIDAK_VALID", Message: "Permintaan tidak valid."},
	http.StatusUnauthorized:         {Code: "AUTENTIKASI_DIPERLUKAN", Message: "Autentikasi diperlukan."},
	http.StatusForbidden:            {Code: "AKSES_DITOLAK", Message: "Anda tidak memiliki izin untuk mengakses sumber daya ini."},
	http.StatusNotFound:             {Code: "DATA_TIDAK_DITEMUKAN", Message: "Data yang diminta tidak ditemukan."},
	http.StatusMethodNotAllowed:     {Code: "METODE_TIDAK_DIIZINKAN", Message: "Metode HTTP tidak diizinkan untuk endpoint ini."},
	http.StatusConflict:             {Code: "KONFLIK_DATA", Message: "Permintaan bertentangan dengan kondisi data saat ini."},
	http.StatusUnsupportedMediaType: {Code: "TIPE_KONTEN_TIDAK_DIDUKUNG", Message: "Tipe konten permintaan tidak didukung."},
	http.StatusUnprocessableEntity:  {Code: "VALIDASI_GAGAL", Message: "Data yang dikirim tidak memenuhi ketentuan."},
	http.StatusTooManyRequests:      {Code: "TERLALU_BANYAK_PERMINTAAN", Message: "Terlalu banyak permintaan. Silakan coba lagi nanti."},
	http.StatusInternalServerError:  {Code: "KESALAHAN_INTERNAL", Message: "Terjadi kesalahan pada server."},
	http.StatusBadGateway:           {Code: "LAYANAN_EKSTERNAL_BERMASALAH", Message: "Layanan eksternal sedang bermasalah."},
	http.StatusServiceUnavailable:   {Code: "LAYANAN_TIDAK_TERSEDIA", Message: "Layanan sementara tidak tersedia."},
	http.StatusGatewayTimeout:       {Code: "LAYANAN_EKSTERNAL_TIMEOUT", Message: "Layanan eksternal tidak merespons tepat waktu."},
}

// New membuat custom status error dengan envelope API Dikasa.
func New(status int, code, message string, details map[string]string) *StatusError {
	return &StatusError{Status: status, Body: Body{Code: code, Message: message, Details: details}}
}

// Standard membuat status error dari katalog general error HTTP.
func Standard(status int, details map[string]string) *StatusError {
	body, exists := catalog[status]
	if !exists {
		status = http.StatusInternalServerError
		body = catalog[status]
	}
	body.Details = details
	return &StatusError{Status: status, Body: body}
}
