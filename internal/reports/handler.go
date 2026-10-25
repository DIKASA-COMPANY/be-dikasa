package reports

import (
	shared "be-dikasa/internal/shared/errors"
	"be-dikasa/internal/user"
	"context"
	"errors"
	"fmt"
	"github.com/danielgtaylor/huma/v2"
)

// Input contains only report filters; outlet/actor never come from the request.
type Input struct {
	StartDate string `query:"start_date"`
	EndDate   string `query:"end_date"`
	Query     string `query:"query" maxLength:"200"`
	Sort      string `query:"sort" enum:"quantity_desc,net_desc,name_asc"`
	Page      int    `query:"page" default:"1" minimum:"1" maximum:"1000000"`
	PageSize  int    `query:"page_size" default:"20" minimum:"1" maximum:"100"`
}

// ExportInput adds the validated report type to the same export filters.
type ExportInput struct {
	Input
	Kind string `path:"report_kind" enum:"summary,methods,products,categories"`
}

// Output returns the common data/meta envelope.
type Output struct{ Body Result }

// ExportOutput downloads real XLSX bytes.
type ExportOutput struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	Body               []byte
}

// Handler adapts reports to the existing authenticated API group.
type Handler struct{ s *Service }

// NewHandler creates the reporting HTTP adapter.
func NewHandler(s *Service) *Handler { return &Handler{s} }
func (in Input) filter() Filter {
	return Filter{StartDate: in.StartDate, EndDate: in.EndDate, Query: in.Query, Sort: in.Sort, Page: in.Page, PageSize: in.PageSize}
}
func actor(ctx context.Context) (string, error) {
	id, ok := user.AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return "", shared.Standard(401, nil)
	}
	return id, nil
}

// reportError hides internal errors while preserving meaningful refusals.
func reportError(e error) error {
	switch {
	case errors.Is(e, ErrForbidden):
		return shared.Standard(403, nil)
	case errors.Is(e, ErrInvalid):
		return shared.New(422, "FILTER_LAPORAN_TIDAK_VALID", "Periksa tanggal WIB (maksimal366 hari), pencarian dan urutan.", nil)
	case errors.Is(e, ErrUnsupported):
		return shared.New(422, "DATA_LAPORAN_TIDAK_DIDUKUNG", "Nilai laporan tidak dapat diverifikasi atau melebihi batas.", nil)
	}
	return shared.Standard(500, nil)
}

// Read returns one authorized report with full filtered totals.
func (h *Handler) Read(ctx context.Context, in *Input, k Kind) (*Output, error) {
	id, e := actor(ctx)
	if e != nil {
		return nil, e
	}
	r, e := h.s.Read(ctx, id, k, in.filter())
	if e != nil {
		return nil, reportError(e)
	}
	return &Output{Body: r}, nil
}

// Export generates all filtered rows at the export timestamp.
func (h *Handler) Export(ctx context.Context, in *ExportInput) (*ExportOutput, error) {
	id, e := actor(ctx)
	if e != nil {
		return nil, e
	}
	b, e := h.s.Export(ctx, id, Kind(in.Kind), in.filter())
	if e != nil {
		return nil, reportError(e)
	}
	return &ExportOutput{ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ContentDisposition: fmt.Sprintf(`attachment; filename="dikasa-%s.xlsx"`, in.Kind), Body: b}, nil
}

// RegisterRoutes registers four read-only reports and matching XLSX export.
func RegisterRoutes(api huma.API, h *Handler) {
	for _, kind := range []Kind{Summary, Methods, Products, Categories} {
		k := kind
		huma.Register(api, huma.Operation{OperationID: "report-" + string(k), Method: "GET", Path: "/reports/" + string(k), Summary: "Laporan " + string(k), Description: "Kasir/manager outlet aktif. Tanggal inklusif WIB, maksimal366 hari. Penjualan menurut paid_at, refund menurut created_at. Nilai asli; kategori historis yang tidak tercatat tidak ditebak.", Tags: []string{"Laporan"}, Errors: []int{401, 403, 422, 500}}, func(ctx context.Context, in *Input) (*Output, error) { return h.Read(ctx, in, k) })
	}
	huma.Register(api, huma.Operation{OperationID: "export-report-xlsx", Method: "GET", Path: "/reports/{report_kind}/export.xlsx", Summary: "Export Excel laporan", Description: "Kasir/manager outlet. Semua baris cocok filter, tanpa pemotongan halaman. Dibuat ulang saat export; label literal tanpa formula.", Tags: []string{"Laporan"}, Errors: []int{401, 403, 422, 500}}, h.Export)
}
