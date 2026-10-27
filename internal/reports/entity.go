// Package reports calculates outlet-scoped sales activity without rewriting ledgers.
package reports

import (
	"be-dikasa/internal/payment"
	"be-dikasa/internal/refund"
	"errors"
	"time"
)

// Kind identifies one of the four delivered report views.
type Kind string

const (
	// Summary is the daily overview.
	Summary Kind = "summary"
	// Methods groups original and returned payment methods independently.
	Methods Kind = "methods"
	// Products groups original product and variation identity.
	Products Kind = "products"
	// Categories groups original category snapshots, including unknown history.
	Categories Kind = "categories"
)

var (
	// ErrInvalid rejects invalid report filters.
	ErrInvalid = errors.New("invalid report filter")
	// ErrForbidden rejects identities without report access.
	ErrForbidden = errors.New("reports forbidden")
	// ErrUnsupported rejects inconsistent or overflowing ledger values.
	ErrUnsupported = errors.New("report values cannot be verified")
)

// Filter carries inclusive WIB dates and bounded group browsing.
type Filter struct {
	StartDate, EndDate, Query, Sort string
	Page, PageSize                  int
}

// Window uses a half-open UTC interval for inclusive local report dates.
type Window struct {
	Start, EndExclusive          time.Time
	StartDate, EndDate, Timezone string
}

// Receipt preserves an original receipt, including older parents of period refunds.
type Receipt struct{ Transaction payment.Transaction }

// Snapshot contains one consistent, outlet-scoped read of original money and events.
type Snapshot struct {
	Receipts    []Receipt
	Refunds     []refund.Refund
	GeneratedAt time.Time
}

// Totals defines signed whole-Rupiah amounts and independent sale/refund quantities.
type Totals struct {
	GrossSales              int64  `json:"gross_sales"`
	DiscountAmount          int64  `json:"discount_amount"`
	SalesAfterDiscount      int64  `json:"sales_after_discount"`
	ChargedSales            int64  `json:"charged_sales"`
	RefundNet               int64  `json:"refund_net"`
	RefundTax               int64  `json:"refund_tax"`
	RefundAmount            int64  `json:"refund_amount"`
	NetSales                int64  `json:"net_sales"`
	SaleTax                 int64  `json:"sale_tax"`
	NetTax                  int64  `json:"net_tax"`
	ValueAfterRefunds       int64  `json:"value_after_refunds"`
	SaleCount               int64  `json:"sale_count"`
	RefundCount             int64  `json:"refund_count"`
	SoldQuantity            int64  `json:"sold_quantity"`
	ReturnedQuantity        int64  `json:"returned_quantity"`
	NetQuantity             int64  `json:"net_quantity"`
	AverageTransaction      *int64 `json:"average_transaction"`
	ExcludedReceipts        int64  `json:"excluded_receipts"`
	UnknownCategoryQuantity int64  `json:"unknown_category_quantity"`
}

// DailyPoint zero-fills the requested local calendar range.
type DailyPoint struct {
	Date         string `json:"date"`
	SaleCount    int64  `json:"sale_count"`
	ChargedSales int64  `json:"charged_sales"`
	RefundAmount int64  `json:"refund_amount"`
	NetSales     int64  `json:"net_sales"`
}

// MethodRow separates sale method from actual refund method.
type MethodRow struct {
	Method       string `json:"method"`
	SaleCount    int64  `json:"sale_count"`
	SalesAmount  int64  `json:"sales_amount"`
	RefundCount  int64  `json:"refund_count"`
	RefundAmount int64  `json:"refund_amount"`
	NetAmount    int64  `json:"net_amount"`
}

// GroupRow retains stable original identity and varied original unit prices.
type GroupRow struct {
	Key          string  `json:"key"`
	Name         string  `json:"name"`
	ProductID    string  `json:"product_id,omitempty"`
	VariantID    *string `json:"variant_id,omitempty"`
	CategoryID   *string `json:"category_id,omitempty"`
	CategoryName string  `json:"category_name,omitempty"`
	UnitPriceMin *int64  `json:"unit_price_min"`
	UnitPriceMax *int64  `json:"unit_price_max"`
	Totals
}

// Report is shared by screen and workbook calculations.
type Report struct {
	Kind    Kind         `json:"kind"`
	Totals  Totals       `json:"totals"`
	Daily   []DailyPoint `json:"daily"`
	Methods []MethodRow  `json:"methods"`
	Rows    []GroupRow   `json:"rows"`
}

// Meta records the precise filter, generation time and group pagination.
type Meta struct {
	Query       string    `json:"query"`
	Sort        string    `json:"sort"`
	StartDate   string    `json:"start_date"`
	EndDate     string    `json:"end_date"`
	Timezone    string    `json:"timezone"`
	GeneratedAt time.Time `json:"generated_at"`
	Page        int       `json:"page"`
	PageSize    int       `json:"page_size"`
	Total       int64     `json:"total"`
}

// Result uses the existing POS data/meta envelope.
type Result struct {
	Data Report `json:"data"`
	Meta Meta   `json:"meta"`
}
