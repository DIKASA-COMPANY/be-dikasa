// Package response menulis envelope response HTTP secara konsisten.
package response

import (
	"encoding/json"
	"net/http"

	sharederrors "be-dikasa/internal/shared/errors"
)

// WriteError menulis StatusError sebagai JSON tanpa membocorkan error internal.
func WriteError(w http.ResponseWriter, statusError *sharederrors.StatusError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusError.GetStatus())
	_ = json.NewEncoder(w).Encode(sharederrors.Response{Error: statusError.Body})
}
