package catalog

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
)

// StockHandler exposes manager-only stock setup, adjustments and paginated audit history.
type StockHandler struct{ service *StockService }

// NewStockHandler connects stock use cases to the API.
func NewStockHandler(service *StockService) *StockHandler { return &StockHandler{service: service} }

// StockWriteInput carries a product-scoped, deduplicated inventory command.
type StockWriteInput struct {
	ProductID string `path:"product_id" format:"uuid"`
	Body      StockCommand
}

// StockWriteOutput contains the committed command's audit record.
type StockWriteOutput struct {
	Body struct {
		Data *StockMovement `json:"data"`
	}
}

// StockHistoryInput specifies one page of product movement history.
type StockHistoryInput struct {
	ProductID string `path:"product_id" format:"uuid"`
	Page      int    `query:"page" default:"1" minimum:"1"`
	PageSize  int    `query:"page_size" default:"30" minimum:"1" maximum:"100"`
}

// StockHistoryOutput contains stock audit rows and their pagination metadata.
type StockHistoryOutput struct {
	Body struct {
		Data []StockMovement `json:"data"`
		Meta struct {
			Page     int   `json:"page"`
			PageSize int   `json:"page_size"`
			Total    int64 `json:"total"`
		} `json:"meta"`
	}
}

func (h *StockHandler) write(ctx context.Context, in *StockWriteInput, kind string) (*StockWriteOutput, error) {
	u, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	move, err := h.service.Execute(ctx, u, in.ProductID, kind, in.Body)
	if err != nil {
		return nil, catalogStatusError(err)
	}
	out := &StockWriteOutput{}
	out.Body.Data = move
	return out, nil
}

// Configure changes tracking settings without accepting an overwrite of a live balance.
func (h *StockHandler) Configure(ctx context.Context, in *StockWriteInput) (*StockWriteOutput, error) {
	return h.write(ctx, in, "configure")
}

// Adjust applies a reasoned signed stock change exactly once per manager request identifier.
func (h *StockHandler) Adjust(ctx context.Context, in *StockWriteInput) (*StockWriteOutput, error) {
	return h.write(ctx, in, "adjust")
}

// History lists recent inventory movements scoped to the manager's outlet.
func (h *StockHandler) History(ctx context.Context, in *StockHistoryInput) (*StockHistoryOutput, error) {
	u, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	rows, total, err := h.service.Movements(ctx, u, in.ProductID, in.Page, in.PageSize)
	if err != nil {
		return nil, catalogStatusError(err)
	}
	out := &StockHistoryOutput{}
	out.Body.Data = rows
	out.Body.Meta.Page = in.Page
	out.Body.Meta.PageSize = in.PageSize
	out.Body.Meta.Total = total
	return out, nil
}

// RegisterStockRoutes registers inventory resources under the existing versioned API group.
func RegisterStockRoutes(api huma.API, h *StockHandler) {
	errors := []int{401, 403, 404, 409, 413, 422, 500}
	huma.Register(api, huma.Operation{OperationID: "configure-product-stock", Method: "POST", Path: "/products/{product_id}/stock", Summary: "Mengatur pelacakan stok bersama", Description: "Manager. request_id menduplikasi perintah yang identik. initial_quantity hanya ketika mengaktifkan pelacakan yang sebelumnya nonaktif.", Tags: []string{"Stok"}, Errors: errors}, h.Configure)
	huma.Register(api, huma.Operation{OperationID: "adjust-product-stock", Method: "POST", Path: "/products/{product_id}/stock-adjustments", Summary: "Menyesuaikan stok dengan alasan dan request_id", Tags: []string{"Stok"}, Errors: errors}, h.Adjust)
	huma.Register(api, huma.Operation{OperationID: "list-stock-movements", Method: "GET", Path: "/products/{product_id}/stock-movements", Summary: "Riwayat perubahan stok outlet", Tags: []string{"Stok"}, Errors: errors}, h.History)
}

// ExportInput filters all exported products without a page-size limit in the workbook.
type ExportInput struct {
	ProductID       string `query:"product_id" format:"uuid"`
	ProductType     string `query:"product_type" enum:"food,drink,additional"`
	CategoryID      string `query:"category_id" format:"uuid"`
	Search          string `query:"searchByProductName" maxLength:"100"`
	Availability    string `query:"availability" enum:"available,low_stock,out_of_stock"`
	IncludeInactive bool   `query:"include_inactive"`
}

// ExportOutput carries a real XLSX attachment, not a JSON or renamed CSV response.
type ExportOutput struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	Body               []byte
}

// ExportHandler connects workbook generation to the manager-scoped catalog service.
type ExportHandler struct{ service *Service }

// NewExportHandler creates the catalog workbook transport.
func NewExportHandler(service *Service) *ExportHandler { return &ExportHandler{service: service} }

// Workbook exports all matching products for the authenticated manager.
func (h *ExportHandler) Workbook(ctx context.Context, in *ExportInput) (*ExportOutput, error) {
	u, err := authenticated(ctx)
	if err != nil {
		return nil, err
	}
	data, err := h.service.ExportWorkbook(ctx, u, ProductFilter{ProductType: in.ProductType, CategoryID: in.CategoryID, SearchByProductName: in.Search, IncludeInactive: in.IncludeInactive}, in.ProductID, in.Availability)
	if err != nil {
		return nil, catalogStatusError(err)
	}
	return &ExportOutput{ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ContentDisposition: `attachment; filename="dikasa-products.xlsx"`, Body: data}, nil
}

// RegisterExportRoutes registers the authenticated product workbook download.
func RegisterExportRoutes(api huma.API, h *ExportHandler) {
	huma.Register(api, huma.Operation{OperationID: "export-products-xlsx", Method: "GET", Path: "/products/export.xlsx", Summary: "Unduh workbook Excel produk dan variasi", Description: "Manager. Seluruh halaman yang cocok dengan filter outlet; stok bersama ditulis satu kali per produk.", Tags: []string{"Katalog"}, Errors: []int{401, 403, 404, 422, 500}}, h.Workbook)
}
