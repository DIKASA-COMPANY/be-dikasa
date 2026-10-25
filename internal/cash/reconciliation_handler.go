package cash

import (
	shared "be-dikasa/internal/shared/errors"
	"be-dikasa/internal/user"
	"context"
	"errors"
	"github.com/danielgtaylor/huma/v2"
)

// ReconciliationHandler exposes session history and reviewed cash closing.
type ReconciliationHandler struct{ s *ReconciliationService }

// NewReconciliationHandler reuses existing authenticated account resolution.
func NewReconciliationHandler(s *ReconciliationService) *ReconciliationHandler {
	return &ReconciliationHandler{s}
}

// CashSessionInput identifies a session in the authenticated outlet.
type CashSessionInput struct {
	ID string `path:"session_id" format:"uuid"`
}

// CashSessionsInput bounds outlet history pagination.
type CashSessionsInput struct {
	Page     int `query:"page" default:"1" minimum:"1" maximum:"1000000"`
	PageSize int `query:"page_size" default:"20" minimum:"1" maximum:"100"`
}

// CashPreviewInput carries physical counted cash and explanation.
type CashPreviewInput struct {
	ID   string `path:"session_id" format:"uuid"`
	Body CloseSelection
}

// CashCloseInput binds a closure request to its reviewed preview.
type CashCloseInput struct {
	ID   string `path:"session_id" format:"uuid"`
	Body CloseCommand
}

// CashDetailOutput wraps session detail and optional legacy reconciliation.
type CashDetailOutput struct {
	Body struct {
		Data SessionRecord `json:"data"`
	}
}

// CashPreviewOutput wraps authoritative close preview.
type CashPreviewOutput struct {
	Body struct {
		Data ClosePreview `json:"data"`
	}
}

// CashCloseOutput wraps immutable success, including exact replay.
type CashCloseOutput struct {
	Body struct {
		Data ClosedReconciliation `json:"data"`
	}
}

// CashSessionsOutput follows existing paginated envelopes.
type CashSessionsOutput struct {
	Body struct {
		Data []SessionRecord `json:"data"`
		Meta struct {
			Page     int   `json:"page"`
			PageSize int   `json:"page_size"`
			Total    int64 `json:"total"`
		} `json:"meta"`
	}
}

func closingIdentity(ctx context.Context) (string, error) {
	id, ok := user.AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return "", shared.Standard(401, nil)
	}
	return id, nil
}

// closingError exposes stable business codes without database internals.
func closingError(e error) error {
	switch {
	case errors.Is(e, ErrCloseForbidden):
		return shared.Standard(403, nil)
	case errors.Is(e, ErrNotFound):
		return shared.Standard(404, nil)
	case errors.Is(e, ErrCloseInvalid):
		return shared.New(422, "HITUNGAN_KAS_TIDAK_VALID", "Periksa nominal kas dan alasan selisih (maksimal 500 karakter).", nil)
	case errors.Is(e, ErrCloseUnsupported):
		return shared.New(422, "REKONSILIASI_TIDAK_DIDUKUNG", "Aktivitas sesi tidak dapat diverifikasi. Periksa catatan asli; sesi tetap terbuka.", nil)
	case errors.Is(e, ErrCloseStale):
		return shared.New(409, "SESI_KAS_PRATINJAU_BERUBAH", "Aktivitas berubah. Muat ulang dan periksa pratinjau baru.", nil)
	case errors.Is(e, ErrAlreadyClosed):
		return shared.New(409, "SESI_KAS_SUDAH_DITUTUP", "Sesi ini sudah ditutup. Periksa ringkasannya.", nil)
	case errors.Is(e, ErrCloseRequestUsed):
		return shared.New(409, "REQUEST_ID_DIGUNAKAN", "ID permintaan sudah dipakai untuk penutupan lain.", nil)
	}
	return shared.Standard(500, nil)
}

// Detail returns scoped open/closed session information.
func (h *ReconciliationHandler) Detail(ctx context.Context, in *CashSessionInput) (*CashDetailOutput, error) {
	u, e := closingIdentity(ctx)
	if e != nil {
		return nil, e
	}
	v, e := h.s.Detail(ctx, u, in.ID)
	if e != nil {
		return nil, closingError(e)
	}
	o := &CashDetailOutput{}
	o.Body.Data = v
	return o, nil
}

// List returns paginated session history.
func (h *ReconciliationHandler) List(ctx context.Context, in *CashSessionsInput) (*CashSessionsOutput, error) {
	u, e := closingIdentity(ctx)
	if e != nil {
		return nil, e
	}
	v, n, e := h.s.List(ctx, u, in.Page, in.PageSize)
	if e != nil {
		return nil, closingError(e)
	}
	o := &CashSessionsOutput{}
	o.Body.Data = v
	o.Body.Meta.Page = in.Page
	o.Body.Meta.PageSize = in.PageSize
	o.Body.Meta.Total = n
	return o, nil
}

// Preview calculates a count against current activity without closing the session.
func (h *ReconciliationHandler) Preview(ctx context.Context, in *CashPreviewInput) (*CashPreviewOutput, error) {
	u, e := closingIdentity(ctx)
	if e != nil {
		return nil, e
	}
	v, e := h.s.Preview(ctx, u, in.ID, in.Body)
	if e != nil {
		return nil, closingError(e)
	}
	o := &CashPreviewOutput{}
	o.Body.Data = v
	return o, nil
}

// Close records one immutable session closure or returns its original response.
func (h *ReconciliationHandler) Close(ctx context.Context, in *CashCloseInput) (*CashCloseOutput, error) {
	u, e := closingIdentity(ctx)
	if e != nil {
		return nil, e
	}
	v, e := h.s.Close(ctx, u, in.ID, in.Body)
	if e != nil {
		return nil, closingError(e)
	}
	o := &CashCloseOutput{}
	o.Body.Data = v
	return o, nil
}

// RegisterReconciliationRoutes uses the parent authenticated /dikasa/v1 router.
func RegisterReconciliationRoutes(api huma.API, h *ReconciliationHandler) {
	huma.Register(api, huma.Operation{OperationID: "list-cash-sessions", Method: "GET", Path: "/cash-sessions", Summary: "Riwayat sesi kas outlet", Tags: []string{"Sesi Kas"}, Errors: []int{401, 403, 422, 500}}, h.List)
	huma.Register(api, huma.Operation{OperationID: "get-cash-session", Method: "GET", Path: "/cash-sessions/{session_id}", Summary: "Ringkasan sesi kas", Tags: []string{"Sesi Kas"}, Errors: []int{401, 403, 404, 422, 500}}, h.Detail)
	huma.Register(api, huma.Operation{OperationID: "preview-close-cash-session", Method: "POST", Path: "/cash-sessions/{session_id}/close/preview", Summary: "Pratinjau hitungan kas", Description: "Kasir pembuka atau manager outlet. Kas awal + tunai diterima setelah kembalian - refund tunai pada sesi ini. Alasan wajib bila selisih bukan nol.", Tags: []string{"Sesi Kas"}, Errors: []int{401, 403, 404, 409, 422, 500}}, h.Preview)
	huma.Register(api, huma.Operation{OperationID: "close-cash-session", Method: "POST", Path: "/cash-sessions/{session_id}/close", Summary: "Tutup sesi kas", Description: "Kasir pembuka atau manager outlet. Wajib request UUID dan preview hash. Permintaan identik mengembalikan hasil awal; pratinjau usang atau UUID berbeda untuk sesi tertutup ditolak. Tidak menghapus keranjang atau membuka sesi berikutnya.", Tags: []string{"Sesi Kas"}, Errors: []int{401, 403, 404, 409, 422, 500}}, h.Close)
}
