package payment

import "time"

const (
	// PaymentMethodCash menandakan pembayaran tunai.
	PaymentMethodCash = "cash"
	// PaymentMethodWallet menandakan pembayaran melalui wallet yang dipilih.
	PaymentMethodWallet = "wallet"
	// PaymentMethodDebitCard menandakan pembayaran kartu debit/EDC.
	PaymentMethodDebitCard = "debit_card"
	// PaymentMethodQRIS menandakan pembayaran QRIS.
	PaymentMethodQRIS = "qris"
	// PaymentMethodPoint menandakan pembayaran menggunakan point.
	PaymentMethodPoint = "point"
	// PaymentMethodTransfer menandakan pembayaran transfer bank.
	PaymentMethodTransfer = "transfer"
	// TransactionStatusPaid menandakan transaksi sudah dibayar.
	TransactionStatusPaid = "paid"
)

// Wallet adalah pilihan e-wallet milik satu outlet.
type Wallet struct {
	ID        string    `gorm:"type:uuid;primaryKey"`
	OutletID  string    `gorm:"type:uuid;not null;index"`
	Code      string    `gorm:"size:50;not null"`
	Name      string    `gorm:"size:100;not null"`
	IconKey   *string   `gorm:"size:100"`
	SortOrder int       `gorm:"not null"`
	IsActive  bool      `gorm:"not null"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

// TableName menetapkan tabel wallet pembayaran.
func (Wallet) TableName() string { return "payment_wallets" }

// Transaction adalah snapshot pembayaran POS yang sudah berhasil.
type Transaction struct {
	RefundRevision     int64             `gorm:"not null"`
	SavedOrderID       *string           `gorm:"-"`
	SavedOrderRevision *int64            `gorm:"-"`
	ID                 string            `gorm:"type:uuid;primaryKey"`
	TransactionNumber  string            `gorm:"size:40;uniqueIndex;not null"`
	OutletID           string            `gorm:"type:uuid;not null;index"`
	CashSessionID      string            `gorm:"type:uuid;not null;index"`
	CreatedBy          string            `gorm:"type:uuid;not null;index"`
	OrderTypeID        string            `gorm:"type:uuid;not null;index"`
	RestaurantTableID  *string           `gorm:"type:uuid"`
	PaymentMethod      string            `gorm:"size:20;not null"`
	WalletID           *string           `gorm:"type:uuid;index"`
	Subtotal           int64             `gorm:"not null"`
	DiscountAmount     int64             `gorm:"not null"`
	TaxPercentage      float64           `gorm:"type:numeric(5,2);not null"`
	TaxAmount          int64             `gorm:"not null"`
	TotalAmount        int64             `gorm:"not null"`
	PaidAmount         int64             `gorm:"not null"`
	ChangeAmount       int64             `gorm:"not null"`
	RoundingAmount     int64             `gorm:"not null"`
	TipAmount          int64             `gorm:"not null"`
	TotalQuantity      int               `gorm:"not null"`
	Status             string            `gorm:"size:20;not null"`
	PaidAt             time.Time         `gorm:"not null"`
	CreatedAt          time.Time         `gorm:"not null"`
	UpdatedAt          time.Time         `gorm:"not null"`
	Wallet             *Wallet           `gorm:"foreignKey:WalletID"`
	Items              []TransactionItem `gorm:"foreignKey:TransactionID"`
}

// TableName menetapkan tabel transaksi.
func (Transaction) TableName() string { return "transactions" }

// TransactionItem adalah snapshot produk pada saat transaksi dibayar.
type TransactionItem struct {
	CategoryIDSnapshot   *string   `gorm:"type:uuid"`
	CategoryNameSnapshot *string   `gorm:"type:text"`
	ID                   string    `gorm:"type:uuid;primaryKey"`
	TransactionID        string    `gorm:"type:uuid;not null;index"`
	OutletID             string    `gorm:"type:uuid;not null"`
	ProductID            string    `gorm:"type:uuid;not null;index"`
	ProductVariantID     *string   `gorm:"type:uuid"`
	ProductName          string    `gorm:"size:150;not null"`
	VariantName          *string   `gorm:"size:100"`
	UnitPrice            int64     `gorm:"not null"`
	Quantity             int       `gorm:"not null"`
	Notes                string    `gorm:"type:text;not null"`
	LineTotal            int64     `gorm:"not null"`
	CreatedAt            time.Time `gorm:"not null"`
}

// TableName menetapkan tabel item transaksi.
func (TransactionItem) TableName() string { return "transaction_items" }

// WalletMutation berisi input create atau update wallet.
type WalletMutation struct {
	Code, Name string
	IconKey    *string
	SortOrder  int
	IsActive   bool
}

// TransactionMutation berisi snapshot hasil kalkulasi FE yang akan divalidasi dan disimpan.
type TransactionMutation struct {
	SavedOrderID                *string
	SavedOrderRevision          *int64
	OrderTypeID, PaymentMethod  string
	RestaurantTableID, WalletID *string
	Subtotal, DiscountAmount    int64
	TaxPercentage               float64
	TaxAmount, TotalAmount      int64
	PaidAmount, ChangeAmount    int64
	RoundingAmount, TipAmount   int64
	Items                       []TransactionItemMutation
}

// TransactionItemMutation adalah snapshot satu baris cart dari FE.
type TransactionItemMutation struct {
	ProductID        string
	ProductVariantID *string
	ProductName      string
	VariantName      *string
	UnitPrice        int64
	Quantity         int
	Notes            string
	LineTotal        int64
}
