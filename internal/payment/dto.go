package payment

import "time"

// TaxPercentageResponse adalah response persentase pajak outlet untuk kalkulasi FE.
type TaxPercentageResponse struct {
	TaxPercentage float64 `json:"tax_percentage" minimum:"0" maximum:"100" example:"10"`
}

// TaxPercentageOutput membungkus response persentase pajak untuk Huma.
type TaxPercentageOutput struct {
	Body TaxPercentageResponse
}

// WalletData adalah item dropdown wallet outlet.
type WalletData struct {
	ID        string  `json:"id" format:"uuid"`
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	IconKey   *string `json:"icon_key,omitempty"`
	SortOrder int     `json:"sort_order"`
	IsActive  bool    `json:"is_active"`
}

// WalletListResponse adalah envelope list wallet.
type WalletListResponse struct {
	Data []WalletData `json:"data"`
}

// WalletResponse adalah envelope satu wallet.
type WalletResponse struct {
	Data WalletData `json:"data"`
}

// WalletListInput memuat filter manager untuk wallet nonaktif.
type WalletListInput struct {
	IncludeInactive bool `query:"include_inactive" required:"false"`
}

// WalletRequest adalah payload create atau update wallet.
type WalletRequest struct {
	Code      string  `json:"code" validate:"required,max=50" maxLength:"50" example:"gopay"`
	Name      string  `json:"name" validate:"required,max=100" maxLength:"100" example:"GoPay"`
	IconKey   *string `json:"icon_key,omitempty" validate:"omitempty,max=100" maxLength:"100" example:"gopay"`
	SortOrder int     `json:"sort_order" example:"10"`
	IsActive  bool    `json:"is_active" default:"true"`
}

// CreateWalletInput membungkus payload pembuatan wallet.
type CreateWalletInput struct{ Body WalletRequest }

// UpdateWalletInput membungkus path dan payload perubahan wallet.
type UpdateWalletInput struct {
	WalletID string `path:"wallet_id" format:"uuid"`
	Body     WalletRequest
}

// DeleteWalletInput membungkus path soft delete wallet.
type DeleteWalletInput struct {
	WalletID string `path:"wallet_id" format:"uuid"`
}

// WalletListOutput membungkus list wallet untuk Huma.
type WalletListOutput struct{ Body WalletListResponse }

// WalletOutput membungkus satu wallet untuk Huma.
type WalletOutput struct {
	Status int
	Body   WalletResponse
}

// NoContentOutput membungkus response 204 pembayaran.
type NoContentOutput struct{ Status int }

// TransactionItemRequest adalah snapshot satu baris cart yang dihitung FE.
type TransactionItemRequest struct {
	ProductID        string  `json:"product_id" validate:"required,uuid" format:"uuid"`
	ProductVariantID *string `json:"product_variant_id,omitempty" validate:"omitempty,uuid" format:"uuid"`
	ProductName      string  `json:"product_name" validate:"required,max=150" maxLength:"150"`
	VariantName      *string `json:"variant_name,omitempty" validate:"omitempty,max=100" maxLength:"100"`
	UnitPrice        int64   `json:"unit_price" validate:"gte=0,lte=9007199254740991" minimum:"0" maximum:"9007199254740991"`
	Quantity         int     `json:"quantity" validate:"gte=1,lte=1000" minimum:"1" maximum:"1000"`
	Notes            string  `json:"notes,omitempty" maxLength:"500"`
	LineTotal        int64   `json:"line_total" validate:"gte=0,lte=9007199254740991" minimum:"0" maximum:"9007199254740991"`
}

// TransactionRequest adalah payload pembayaran final dari FE.
type TransactionRequest struct {
	SavedOrderID       *string                  `json:"saved_order_id,omitempty" format:"uuid"`
	SavedOrderRevision *int64                   `json:"saved_order_revision,omitempty" minimum:"1"`
	OrderTypeID        string                   `json:"order_type_id" validate:"required,uuid" format:"uuid"`
	RestaurantTableID  *string                  `json:"restaurant_table_id,omitempty" validate:"omitempty,uuid" format:"uuid"`
	PaymentMethod      string                   `json:"payment_method" validate:"required,oneof=cash wallet debit_card qris point transfer" enum:"cash,wallet,debit_card,qris,point,transfer"`
	WalletID           *string                  `json:"wallet_id,omitempty" validate:"omitempty,uuid" format:"uuid"`
	Subtotal           int64                    `json:"subtotal" validate:"gte=0,lte=9007199254740991" minimum:"0" maximum:"9007199254740991"`
	DiscountAmount     int64                    `json:"discount_amount" validate:"gte=0,lte=9007199254740991" minimum:"0" maximum:"9007199254740991"`
	TaxPercentage      float64                  `json:"tax_percentage" validate:"gte=0,lte=100" minimum:"0" maximum:"100"`
	TaxAmount          int64                    `json:"tax_amount" validate:"gte=0,lte=9007199254740991" minimum:"0" maximum:"9007199254740991"`
	TotalAmount        int64                    `json:"total_amount" validate:"gte=0,lte=9007199254740991" minimum:"0" maximum:"9007199254740991"`
	PaidAmount         int64                    `json:"paid_amount" validate:"gte=0,lte=9007199254740991" minimum:"0" maximum:"9007199254740991"`
	ChangeAmount       int64                    `json:"change_amount" validate:"gte=0,lte=9007199254740991" minimum:"0" maximum:"9007199254740991"`
	RoundingAmount     int64                    `json:"rounding_amount" validate:"gte=0,lte=9007199254740991" minimum:"0" maximum:"9007199254740991"`
	TipAmount          int64                    `json:"tip_amount" validate:"gte=0,lte=9007199254740991" minimum:"0" maximum:"9007199254740991"`
	Items              []TransactionItemRequest `json:"items" validate:"required,min=1,max=100,dive" minItems:"1" maxItems:"100"`
}

// CreateTransactionInput membungkus payload penyimpanan transaksi.
type CreateTransactionInput struct{ Body TransactionRequest }

// TransactionItemData adalah snapshot item tersimpan untuk struk dan layar sukses.
type TransactionItemData struct {
	ID               string  `json:"id" format:"uuid"`
	ProductID        string  `json:"product_id" format:"uuid"`
	ProductVariantID *string `json:"product_variant_id,omitempty" format:"uuid"`
	ProductName      string  `json:"product_name"`
	VariantName      *string `json:"variant_name,omitempty"`
	UnitPrice        int64   `json:"unit_price"`
	Quantity         int     `json:"quantity"`
	Notes            string  `json:"notes,omitempty"`
	LineTotal        int64   `json:"line_total"`
}

// TransactionData adalah transaksi lengkap untuk layar transaksi berhasil.
type TransactionData struct {
	ID                string                `json:"id" format:"uuid"`
	TransactionNumber string                `json:"transaction_number"`
	CashSessionID     string                `json:"cash_session_id" format:"uuid"`
	OrderTypeID       string                `json:"order_type_id" format:"uuid"`
	RestaurantTableID *string               `json:"restaurant_table_id,omitempty" format:"uuid"`
	PaymentMethod     string                `json:"payment_method"`
	Wallet            *WalletData           `json:"wallet,omitempty"`
	Subtotal          int64                 `json:"subtotal"`
	DiscountAmount    int64                 `json:"discount_amount"`
	TaxPercentage     float64               `json:"tax_percentage"`
	TaxAmount         int64                 `json:"tax_amount"`
	TotalAmount       int64                 `json:"total_amount"`
	PaidAmount        int64                 `json:"paid_amount"`
	ChangeAmount      int64                 `json:"change_amount"`
	RoundingAmount    int64                 `json:"rounding_amount"`
	TipAmount         int64                 `json:"tip_amount"`
	TotalQuantity     int                   `json:"total_quantity"`
	Status            string                `json:"status"`
	PaidAt            time.Time             `json:"paid_at" format:"date-time"`
	Items             []TransactionItemData `json:"items"`
}

// TransactionResponse adalah envelope transaksi tersimpan.
type TransactionResponse struct {
	Data TransactionData `json:"data"`
}

// TransactionOutput membungkus response 201 transaksi.
type TransactionOutput struct {
	Status int
	Body   TransactionResponse
}
