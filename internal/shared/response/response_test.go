package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	sharederrors "be-dikasa/internal/shared/errors"
)

func TestWriteErrorUsesJSONEnvelope(t *testing.T) {
	recorder := httptest.NewRecorder()

	WriteError(recorder, sharederrors.Standard(http.StatusNotFound, nil))

	if recorder.Code != http.StatusNotFound || recorder.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected response metadata: %d %q", recorder.Code, recorder.Header().Get("Content-Type"))
	}
	var body sharederrors.Response
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "DATA_TIDAK_DITEMUKAN" {
		t.Fatalf("unexpected response: %+v", body)
	}
}
