// Package refund records quantity-based reversals without rewriting original sales.
package refund

import "time"

// ReceiptItem combines the original sale snapshot with committed refund quantities.
type ReceiptItem struct {
	ID               string  `json:"id"`
	ProductID        string  `json:"product_id"`
	VariantID        *string `json:"product_variant_id,omitempty"`
	Name             string  `json:"product_name"`
	VariantName      *string `json:"variant_name,omitempty"`
	UnitPrice        int64   `json:"unit_price"`
	Quantity         int     `json:"quantity"`
	LineTotal        int64   `json:"line_total"`
	RefundedQuantity int     `json:"refunded_quantity"`
	ReturnedQuantity int     `json:"returned_quantity"`
	TrackStock       bool    `json:"track_stock"`
	StockQuantity    int64   `json:"stock_quantity"`
	RestockBudget    int64   `json:"restock_budget"`
}

// ReceiptSnapshot is a consistent original-receipt and refund-ledger read.
type ReceiptSnapshot struct {
	ID             string        `json:"id"`
	Number         string        `json:"transaction_number"`
	Status         string        `json:"status"`
	Method         string        `json:"payment_method"`
	RefundRevision int64         `json:"refund_revision"`
	Subtotal       int64         `json:"subtotal"`
	DiscountAmount int64         `json:"discount_amount"`
	TaxAmount      int64         `json:"tax_amount"`
	TotalAmount    int64         `json:"total_amount"`
	RefundedAmount int64         `json:"refunded_amount"`
	Supported      bool          `json:"refunds_supported"`
	Items          []ReceiptItem `json:"items"`
}

// SelectedItem identifies original receipt units, never a current catalog price.
type SelectedItem struct {
	TransactionItemID string `json:"transaction_item_id" format:"uuid"`
	Quantity          int    `json:"quantity" minimum:"1" maximum:"1000"`
	RestockQuantity   int    `json:"restock_quantity" minimum:"0" maximum:"1000"`
}

// Selection describes exactly what the cashier reviewed before returning money.
type Selection struct {
	Mode             string         `json:"mode" enum:"full,items"`
	Method           string         `json:"method" enum:"cash,wallet,debit_card,qris,transfer"`
	Reason           string         `json:"reason" minLength:"1" maxLength:"500"`
	ExpectedRevision int64          `json:"expected_refund_revision" minimum:"0"`
	Items            []SelectedItem `json:"items" minItems:"1" maxItems:"100"`
}

// Command adds the replay identity and displayed preview hash to a selection.
type Command struct {
	Selection
	RequestID   string `json:"request_id" format:"uuid"`
	PreviewHash string `json:"preview_hash" minLength:"64" maxLength:"64"`
}

// RefundItem is an immutable quantity and component-amount audit row.
type RefundItem struct {
	ID                string `json:"id" gorm:"type:uuid;primaryKey"`
	RefundID          string `json:"-" gorm:"type:uuid"`
	TransactionID     string `json:"-" gorm:"type:uuid"`
	OutletID          string `json:"-" gorm:"type:uuid"`
	TransactionItemID string `json:"transaction_item_id" gorm:"type:uuid"`
	Quantity          int    `json:"quantity"`
	RestockQuantity   int    `json:"restock_quantity"`
	NetAmount         int64  `json:"net_amount"`
	TaxAmount         int64  `json:"tax_amount"`
	TotalAmount       int64  `json:"total_amount"`
}

// TableName selects immutable refund allocations.
func (RefundItem) TableName() string { return "transaction_refund_items" }

// Preview is calculated identically for display and committed confirmation.
type Preview struct {
	TransactionID string       `json:"transaction_id"`
	Revision      int64        `json:"refund_revision"`
	SelectionHash string       `json:"preview_hash"`
	NetAmount     int64        `json:"net_amount"`
	TaxAmount     int64        `json:"tax_amount"`
	TotalAmount   int64        `json:"total_amount"`
	ResultStatus  string       `json:"result_status"`
	Items         []RefundItem `json:"items"`
}

// Refund preserves the actor, session, intent and exact successful response.
type Refund struct {
	ID            string       `json:"id" gorm:"type:uuid;primaryKey"`
	Number        string       `json:"refund_number"`
	OutletID      string       `json:"-" gorm:"type:uuid"`
	TransactionID string       `json:"transaction_id" gorm:"type:uuid"`
	ActorID       string       `json:"actor_id" gorm:"type:uuid"`
	ActorName     string       `json:"actor_name"`
	CashSessionID string       `json:"cash_session_id" gorm:"type:uuid"`
	Reason        string       `json:"reason"`
	Method        string       `json:"method"`
	Mode          string       `json:"mode"`
	NetAmount     int64        `json:"net_amount"`
	TaxAmount     int64        `json:"tax_amount"`
	TotalAmount   int64        `json:"total_amount"`
	CreatedAt     time.Time    `json:"created_at"`
	RequestID     string       `json:"request_id" gorm:"type:uuid"`
	PayloadHash   string       `json:"-"`
	Response      []byte       `json:"-" gorm:"type:jsonb"`
	Items         []RefundItem `json:"items" gorm:"foreignKey:RefundID"`
}

// TableName selects refund headers.
func (Refund) TableName() string { return "transaction_refunds" }
