// Package savedorder persists unpaid outlet carts independently of paid receipts.
package savedorder

import "time"

// Item is an unpaid cart line snapshot; identity is the product/variant pair.
type Item struct {
	OutletID         string  `json:"-" gorm:"type:uuid"`
	ID               string  `json:"-" gorm:"type:uuid;primaryKey"`
	SavedOrderID     string  `json:"-" gorm:"type:uuid"`
	Position         int     `json:"position,omitempty"`
	ProductID        string  `json:"product_id" gorm:"type:uuid" format:"uuid"`
	ProductVariantID *string `json:"product_variant_id,omitempty" gorm:"type:uuid" format:"uuid"`
	ProductName      string  `json:"product_name" maxLength:"150"`
	VariantName      *string `json:"variant_name,omitempty" maxLength:"100"`
	UnitPrice        int64   `json:"unit_price" minimum:"0" maximum:"9007199254740991"`
	Quantity         int     `json:"quantity" minimum:"1" maximum:"1000"`
	Notes            string  `json:"notes,omitempty" maxLength:"500"`
}

// TableName selects the draft line table.
func (Item) TableName() string { return "saved_order_items" }

// Order is an outlet-scoped revisioned cart with an optional final receipt link.
type Order struct {
	ID                string    `json:"id" gorm:"type:uuid;primaryKey"`
	OutletID          string    `json:"-" gorm:"type:uuid"`
	CreatedBy         string    `json:"created_by" gorm:"type:uuid"`
	CreatorName       string    `json:"creator_name" gorm:"->;-:migration"`
	Label             string    `json:"label"`
	OrderTypeID       string    `json:"order_type_id" gorm:"type:uuid"`
	Revision          int64     `json:"revision"`
	Status            string    `json:"status"`
	TransactionID     *string   `json:"transaction_id" gorm:"type:uuid"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
	Items             []Item    `json:"items" gorm:"foreignKey:SavedOrderID"`
	EstimatedSubtotal int64     `json:"estimated_subtotal" gorm:"-"`
	TotalQuantity     int       `json:"total_quantity" gorm:"-"`
}

// TableName selects the draft table.
func (Order) TableName() string { return "saved_orders" }

// Mutation carries a replay key and replacement cart, never an actor or outlet.
type Mutation struct {
	RequestID        string `json:"request_id" format:"uuid"`
	ExpectedRevision int64  `json:"expected_revision,omitempty" minimum:"0"`
	Label            string `json:"label" maxLength:"100"`
	OrderTypeID      string `json:"order_type_id" format:"uuid"`
	Items            []Item `json:"items" minItems:"1" maxItems:"100"`
}

// Summarize calculates the displayed pre-tax estimate from bounded item snapshots.
func (o *Order) Summarize() {
	o.EstimatedSubtotal = 0
	o.TotalQuantity = 0
	for _, i := range o.Items {
		o.EstimatedSubtotal += i.UnitPrice * int64(i.Quantity)
		o.TotalQuantity += i.Quantity
	}
}
