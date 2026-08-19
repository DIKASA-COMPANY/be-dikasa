package catalog

import (
	"errors"
	"net/http"
	"testing"
)

func TestCatalogStatusErrorsUseSpecifiedHTTPContracts(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{ErrForbidden, http.StatusForbidden, "AKSES_DITOLAK"},
		{ErrNotFound, http.StatusNotFound, "DATA_TIDAK_DITEMUKAN"},
		{ErrConflict, http.StatusConflict, "KONFLIK_DATA"},
		{ErrCategoryInUse, http.StatusConflict, "KATEGORI_MASIH_DIGUNAKAN"},
		{ErrImageTooLarge, http.StatusRequestEntityTooLarge, "GAMBAR_TERLALU_BESAR"},
		{ErrStorage, http.StatusBadGateway, "PENYIMPANAN_GAMBAR_BERMASALAH"},
		{ErrStorageDisabled, http.StatusServiceUnavailable, "PENYIMPANAN_GAMBAR_BELUM_DIKONFIGURASI"},
	}
	for _, test := range cases {
		statusError := catalogStatusError(test.err)
		if statusError.Status != test.status || statusError.Body.Code != test.code {
			t.Errorf("%v: got %d/%s", test.err, statusError.Status, statusError.Body.Code)
		}
	}
	if statusError := catalogStatusError(errors.New("database detail")); statusError.Status != http.StatusInternalServerError {
		t.Fatalf("unexpected internal mapping: %+v", statusError)
	}
}
