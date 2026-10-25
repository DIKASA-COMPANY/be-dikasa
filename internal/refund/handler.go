package refund

import (
	sharederrors "be-dikasa/internal/shared/errors"
	"be-dikasa/internal/user"
	"context"
	"errors"
	"github.com/danielgtaylor/huma/v2"
)

// Handler exposes outlet-scoped refund operations.
type Handler struct{ s *Service }

// NewHandler constructs the refund HTTP adapter.
func NewHandler(s *Service) *Handler { return &Handler{s} }

// ReceiptInput identifies an original receipt.
type ReceiptInput struct {
	ID string `path:"transaction_id" format:"uuid"`
}

// PreviewInput carries an original-item selection without authoritative amounts.
type PreviewInput struct {
	ID   string `path:"transaction_id" format:"uuid"`
	Body Selection
}

// ConfirmInput binds one replay UUID to a reviewed preview.
type ConfirmInput struct {
	ID   string `path:"transaction_id" format:"uuid"`
	Body Command
}

// HistoryInput bounds audit pagination.
type HistoryInput struct {
	ID       string `path:"transaction_id" format:"uuid"`
	Page     int    `query:"page" default:"1" minimum:"1" maximum:"1000000"`
	PageSize int    `query:"page_size" default:"20" minimum:"1" maximum:"100"`
}

// RequestInput resolves committed confirmation by request identity.
type RequestInput struct {
	ID string `path:"request_id" format:"uuid"`
}

// DetailOutput returns original receipt and refund eligibility.
type DetailOutput struct {
	Body struct {
		Data ReceiptSnapshot `json:"data"`
	}
}

// PreviewOutput returns server-calculated monetary components.
type PreviewOutput struct {
	Body struct {
		Data Preview `json:"data"`
	}
}

// RefundOutput returns the immutable successful refund.
type RefundOutput struct {
	Body struct {
		Data *Refund `json:"data"`
	}
}

// HistoryOutput follows the existing paginated POS envelope.
type HistoryOutput struct {
	Body struct {
		Data []Refund `json:"data"`
		Meta struct {
			Page     int   `json:"page"`
			PageSize int   `json:"page_size"`
			Total    int64 `json:"total"`
		} `json:"meta"`
	}
}

func identity(ctx context.Context) (string, error) {
	id, ok := user.AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return "", sharederrors.Standard(401, nil)
	}
	return id, nil
}

// StatusError returns stable codes without leaking database errors.
func StatusError(e error) error {
	switch {
	case errors.Is(e, ErrNotFound):
		return sharederrors.Standard(404, nil)
	case errors.Is(e, ErrForbidden):
		return sharederrors.Standard(403, nil)
	case errors.Is(e, ErrValidation):
		return sharederrors.Standard(422, nil)
	case errors.Is(e, ErrLegacy):
		return sharederrors.New(422, "TRANSAKSI_REFUND_TIDAK_DIDUKUNG", "Snapshot transaksi tidak mendukung refund. Periksa catatan asli.", nil)
	case errors.Is(e, ErrRevision):
		return sharederrors.New(409, "REFUND_REVISI_BERUBAH", "Refund berubah. Muat ulang transaksi dan periksa pratinjau baru.", nil)
	case errors.Is(e, ErrQuantity):
		return sharederrors.New(409, "JUMLAH_REFUND_TIDAK_TERSEDIA", "Jumlah item sudah direfund atau tidak tersedia.", nil)
	case errors.Is(e, ErrRequestUsed):
		return sharederrors.New(409, "REQUEST_ID_DIGUNAKAN", "ID permintaan sudah dipakai untuk perubahan lain.", nil)
	case errors.Is(e, ErrCashSession):
		return sharederrors.New(409, "SESI_KAS_BELUM_AKTIF", "Buka sesi kas sebelum mencatat refund.", nil)
	case errors.Is(e, ErrRestock):
		return sharederrors.New(409, "STOK_PENGEMBALIAN_TIDAK_VALID", "Pengembalian stok tidak tersedia. Periksa pelacakan dan stok penjualan asli.", nil)
	default:
		return sharederrors.Standard(500, nil)
	}
}

// Detail reads a consistent receipt snapshot.
func (h *Handler) Detail(ctx context.Context, in *ReceiptInput) (*DetailOutput, error) {
	u, e := identity(ctx)
	if e != nil {
		return nil, e
	}
	d, e := h.s.Detail(ctx, u, in.ID)
	if e != nil {
		return nil, StatusError(e)
	}
	out := &DetailOutput{}
	out.Body.Data = d
	return out, nil
}

// Preview calculates without writing refund or stock records.
func (h *Handler) Preview(ctx context.Context, in *PreviewInput) (*PreviewOutput, error) {
	u, e := identity(ctx)
	if e != nil {
		return nil, e
	}
	d, e := h.s.Preview(ctx, u, in.ID, in.Body)
	if e != nil {
		return nil, StatusError(e)
	}
	out := &PreviewOutput{}
	out.Body.Data = d
	return out, nil
}

// Confirm records or identically replays one reviewed refund.
func (h *Handler) Confirm(ctx context.Context, in *ConfirmInput) (*RefundOutput, error) {
	u, e := identity(ctx)
	if e != nil {
		return nil, e
	}
	d, e := h.s.Confirm(ctx, u, in.ID, in.Body)
	if e != nil {
		return nil, StatusError(e)
	}
	out := &RefundOutput{}
	out.Body.Data = d
	return out, nil
}

// ByRequest resolves only a committed result; a404 does not prove an in-flight request failed.
func (h *Handler) ByRequest(ctx context.Context, in *RequestInput) (*RefundOutput, error) {
	u, e := identity(ctx)
	if e != nil {
		return nil, e
	}
	d, e := h.s.ByRequest(ctx, u, in.ID)
	if e != nil {
		return nil, StatusError(e)
	}
	out := &RefundOutput{}
	out.Body.Data = d
	return out, nil
}

// History returns immutable refunds with original item references.
func (h *Handler) History(ctx context.Context, in *HistoryInput) (*HistoryOutput, error) {
	u, e := identity(ctx)
	if e != nil {
		return nil, e
	}
	rows, n, e := h.s.History(ctx, u, in.ID, in.Page, in.PageSize)
	if e != nil {
		return nil, StatusError(e)
	}
	out := &HistoryOutput{}
	out.Body.Data = rows
	out.Body.Meta.Page = in.Page
	out.Body.Meta.PageSize = in.PageSize
	out.Body.Meta.Total = n
	return out, nil
}

// RegisterRoutes adds all refund operations beneath the existing versioned router group.
func RegisterRoutes(api huma.API, h *Handler) {
	errs := []int{401, 403, 404, 409, 422, 500}
	huma.Register(api, huma.Operation{OperationID: "get-transaction-detail", Method: "GET", Path: "/transactions/{transaction_id}", Tags: []string{"Refund"}, Summary: "Detail transaksi dan sisa refund", Errors: errs}, h.Detail)
	huma.Register(api, huma.Operation{OperationID: "preview-refund", Method: "POST", Path: "/transactions/{transaction_id}/refunds/preview", Tags: []string{"Refund"}, Summary: "Pratinjau refund tanpa mengubah uang atau stok", Errors: errs}, h.Preview)
	huma.Register(api, huma.Operation{OperationID: "confirm-refund", Method: "POST", Path: "/transactions/{transaction_id}/refunds", DefaultStatus: 201, Tags: []string{"Refund"}, Summary: "Catat uang yang sudah dikembalikan secara manual", Errors: errs}, h.Confirm)
	huma.Register(api, huma.Operation{OperationID: "list-transaction-refunds", Method: "GET", Path: "/transactions/{transaction_id}/refunds", Tags: []string{"Refund"}, Summary: "Riwayat refund transaksi", Errors: errs}, h.History)
	huma.Register(api, huma.Operation{OperationID: "refund-by-request", Method: "GET", Path: "/refunds/by-request/{request_id}", Tags: []string{"Refund"}, Summary: "Periksa hasil permintaan refund", Errors: errs}, h.ByRequest)
}
