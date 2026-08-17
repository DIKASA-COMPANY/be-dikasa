package errors

import (
	"net/http"
	"testing"
)

func TestStandardReturnsIndonesianErrors(t *testing.T) {
	tests := []struct {
		status  int
		code    string
		message string
	}{
		{http.StatusBadRequest, "PERMINTAAN_TIDAK_VALID", "Permintaan tidak valid."},
		{http.StatusUnauthorized, "AUTENTIKASI_DIPERLUKAN", "Autentikasi diperlukan."},
		{http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Data yang dikirim tidak memenuhi ketentuan."},
		{http.StatusInternalServerError, "KESALAHAN_INTERNAL", "Terjadi kesalahan pada server."},
	}

	for _, test := range tests {
		errorResponse := Standard(test.status, nil)
		if errorResponse.GetStatus() != test.status || errorResponse.Body.Code != test.code || errorResponse.Body.Message != test.message {
			t.Fatalf("unexpected error response: %+v", errorResponse)
		}
	}
}

func TestNewCreatesCustomStatusError(t *testing.T) {
	err := New(http.StatusUnauthorized, "KREDENSIAL_TIDAK_VALID", "Identifier atau kata sandi salah.", nil)
	if err.GetStatus() != http.StatusUnauthorized || err.Error() != "Identifier atau kata sandi salah." {
		t.Fatalf("unexpected custom error: %+v", err)
	}
}
