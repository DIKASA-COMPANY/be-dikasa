package savedorder

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// LockForCheckout locks the draft until the caller's receipt transaction completes.
func LockForCheckout(ctx context.Context, tx *gorm.DB, outlet, id string, revision int64, orderTypeID string, items []Item) (*Order, error) {
	o, e := readOrder(tx.WithContext(ctx), outlet, id, true)
	if e != nil {
		return nil, e
	}
	if o.Status == "paid" {
		return nil, ErrAlreadyPaid
	}
	if o.Status != "open" || o.Revision != revision || !SameCart(orderTypeID, items, o) {
		return nil, ErrConflict
	}
	return o, nil
}

// CompleteCheckout links a paid receipt atomically with stock and receipt writes.
func CompleteCheckout(ctx context.Context, tx *gorm.DB, o *Order, transactionID string) error {
	r := tx.WithContext(ctx).Model(&Order{}).Where("id = ? AND outlet_id = ? AND status = 'open' AND revision = ?", o.ID, o.OutletID, o.Revision).Updates(map[string]any{"status": "paid", "transaction_id": transactionID, "revision": o.Revision + 1, "updated_at": time.Now().UTC()})
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return ErrConflict
	}
	return nil
}
