package refund

import (
	"be-dikasa/internal/catalog"
	"be-dikasa/internal/payment"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"sort"
	"strings"
	"time"
)

// GORMRepository persists refunds in the original receipt database.
type GORMRepository struct{ db *gorm.DB }

// NewGORMRepository uses a shared database without adding a payment dependency cycle.
func NewGORMRepository(db *gorm.DB) *GORMRepository { return &GORMRepository{db} }

// Detail prevents mixed header/ledger revisions using a read-only consistent snapshot.
func (r *GORMRepository) Detail(ctx context.Context, out, id string) (snap ReceiptSnapshot, e error) {
	e = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { var err error; snap, err = readSnapshot(tx, out, id, false); return err }, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return
}

// readSnapshot scopes the receipt before reading allocations and original stock evidence.
func readSnapshot(tx *gorm.DB, out, id string, lock bool) (ReceiptSnapshot, error) {
	var sale payment.Transaction
	q := tx.Where("id = ? AND outlet_id = ?", id, out)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if e := q.Preload("Items", func(q *gorm.DB) *gorm.DB { return q.Order("id ASC") }).First(&sale).Error; e != nil {
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return ReceiptSnapshot{}, ErrNotFound
		}
		return ReceiptSnapshot{}, e
	}
	s := ReceiptSnapshot{ID: sale.ID, Number: sale.TransactionNumber, Status: sale.Status, Method: sale.PaymentMethod, RefundRevision: sale.RefundRevision, Subtotal: sale.Subtotal, DiscountAmount: sale.DiscountAmount, TaxAmount: sale.TaxAmount, TotalAmount: sale.TotalAmount, Supported: sale.Status == "paid" || sale.Status == "partially_refunded" || (sale.Status == "refunded" && sale.RefundRevision > 0), Items: []ReceiptItem{}}
	var lines []RefundItem
	if e := tx.Where("transaction_id = ? AND outlet_id = ?", id, out).Find(&lines).Error; e != nil {
		return s, e
	}
	refunded := map[string]int{}
	returned := map[string]int{}
	for _, l := range lines {
		refunded[l.TransactionItemID] += l.Quantity
		returned[l.TransactionItemID] += l.RestockQuantity
		s.RefundedAmount += l.TotalAmount
	}
	for _, l := range sale.Items {
		var p catalog.Product
		if e := tx.Where("id = ? AND outlet_id = ?", l.ProductID, out).First(&p).Error; e != nil {
			return s, e
		}
		var budget int64
		if e := tx.Table("stock_movements").Select("COALESCE(SUM(CASE WHEN kind='sale' THEN -quantity_delta WHEN kind='refund_return' THEN -quantity_delta ELSE 0 END),0)").Where("transaction_id = ? AND outlet_id = ? AND product_id = ?", id, out, p.ID).Scan(&budget).Error; e != nil {
			return s, e
		}
		s.Items = append(s.Items, ReceiptItem{ID: l.ID, ProductID: l.ProductID, VariantID: l.ProductVariantID, Name: l.ProductName, VariantName: l.VariantName, UnitPrice: l.UnitPrice, Quantity: l.Quantity, LineTotal: l.LineTotal, RefundedQuantity: refunded[l.ID], ReturnedQuantity: returned[l.ID], TrackStock: p.TrackStock, StockQuantity: p.StockQuantity, RestockBudget: budget})
	}
	return s, nil
}

// History verifies receipt scope before returning ordered audit rows.
func (r *GORMRepository) History(ctx context.Context, out, id string, page, size int) ([]Refund, int64, error) {
	var n int64
	if e := r.db.WithContext(ctx).Model(&payment.Transaction{}).Where("id = ? AND outlet_id = ?", id, out).Count(&n).Error; e != nil {
		return nil, 0, e
	}
	if n == 0 {
		return nil, 0, ErrNotFound
	}
	q := r.db.WithContext(ctx).Model(&Refund{}).Where("transaction_id = ? AND outlet_id = ?", id, out)
	if e := q.Count(&n).Error; e != nil {
		return nil, 0, e
	}
	rows := []Refund{}
	e := q.Omit("Response").Preload("Items").Order("created_at DESC,id").Offset((page - 1) * size).Limit(size).Find(&rows).Error
	return rows, n, e
}

// FindByRequest returns the immutable committed response, not mutable current state.
func (r *GORMRepository) FindByRequest(ctx context.Context, out, request string) (*Refund, error) {
	return findRequest(r.db.WithContext(ctx), out, request, "")
}
func findRequest(tx *gorm.DB, out, request, hash string) (*Refund, error) {
	var row Refund
	if e := tx.Where("outlet_id = ? AND request_id = ?", out, request).Take(&row).Error; e != nil {
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, e
	}
	if hash != "" && row.PayloadHash != hash {
		return nil, ErrRequestUsed
	}
	var result Refund
	if e := json.Unmarshal(row.Response, &result); e != nil {
		return nil, e
	}
	return &result, nil
}

// Confirm atomically serializes refund replay, original units, stock and receipt status.
func (r *GORMRepository) Confirm(ctx context.Context, out, actor, id string, c Command) (result *Refund, err error) {
	c.RequestID = canonical(c.RequestID)
	selectionHash, _ := SelectionHash(id, c.Selection)
	raw, _ := json.Marshal([]string{actor, id, selectionHash, c.PreviewHash})
	digest := sha256.Sum256(raw)
	hash := hex.EncodeToString(digest[:])
	lock := sha256.Sum256([]byte("refund:" + out + ":" + c.RequestID))
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec("SELECT pg_advisory_xact_lock(?)", int64(binary.BigEndian.Uint64(lock[:8]))).Error; e != nil {
			return e
		}
		old, e := findRequest(tx, out, c.RequestID, hash)
		if e == nil {
			result = old
			return nil
		}
		if !errors.Is(e, ErrNotFound) {
			return e
		}
		s, e := readSnapshot(tx, out, id, true)
		if e != nil {
			return e
		}
		preview, e := BuildPreview(s, c.Selection)
		if e != nil {
			return e
		}
		if preview.SelectionHash != c.PreviewHash {
			return ErrRevision
		}
		var cash struct{ ID string }
		e = tx.Table("cash_sessions").Clauses(clause.Locking{Strength: "SHARE"}).Where("outlet_id = ? AND status = 'open'", out).Take(&cash).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return ErrCashSession
		}
		if e != nil {
			return e
		}
		returns := map[string]int64{}
		for _, line := range preview.Items {
			for _, original := range s.Items {
				if original.ID == line.TransactionItemID {
					returns[original.ProductID] += int64(line.RestockQuantity)
				}
			}
		}
		// Refresh parent state under stable row locks; preview eligibility may already be stale.
		ids := []string{}
		for product, q := range returns {
			if q > 0 {
				ids = append(ids, product)
			}
		}
		sort.Strings(ids)
		products := map[string]catalog.Product{}
		for _, product := range ids {
			var p catalog.Product
			if e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND outlet_id = ?", product, out).First(&p).Error; e != nil {
				return e
			}
			if !p.TrackStock || p.StockQuantity > 1000000000-returns[product] {
				return ErrRestock
			}
			products[product] = p
		}
		now := time.Now().UTC()
		value := &Refund{ID: uuid.NewString(), Number: "REF-" + now.Format("20060102") + "-" + strings.ToUpper(uuid.NewString()[:12]), OutletID: out, TransactionID: id, ActorID: actor, CashSessionID: cash.ID, Reason: strings.TrimSpace(c.Reason), Method: c.Method, Mode: c.Mode, NetAmount: preview.NetAmount, TaxAmount: preview.TaxAmount, TotalAmount: preview.TotalAmount, CreatedAt: now, RequestID: c.RequestID, PayloadHash: hash, Items: preview.Items}
		if e = tx.Table("users").Select("TRIM(first_name || ' ' || last_name)").Where("id = ?", actor).Scan(&value.ActorName).Error; e != nil {
			return e
		}
		for i := range value.Items {
			line := &value.Items[i]
			line.ID = uuid.NewString()
			line.RefundID = value.ID
			line.TransactionID = id
			line.OutletID = out
		}
		if e = tx.Omit("Items", "Response").Create(value).Error; e != nil {
			return e
		}
		if e = tx.Create(&value.Items).Error; e != nil {
			return e
		}
		for _, product := range ids {
			p := products[product]
			balance := p.StockQuantity + returns[product]
			if e = tx.Model(&catalog.Product{}).Where("id = ? AND outlet_id = ?", product, out).Update("stock_quantity", balance).Error; e != nil {
				return e
			}
			movement := catalog.StockMovement{ID: uuid.NewString(), OutletID: out, ProductID: product, TransactionID: &id, RefundID: &value.ID, CreatedBy: actor, Kind: "refund_return", QuantityDelta: returns[product], BalanceAfter: balance, TrackStock: true, LowStockThreshold: p.LowStockThreshold, Reason: value.Reason, CreatedAt: now}
			if e = tx.Create(&movement).Error; e != nil {
				return e
			}
		}
		if e = tx.Model(&payment.Transaction{}).Where("id = ? AND outlet_id = ?", id, out).Updates(map[string]any{"status": preview.ResultStatus, "refund_revision": s.RefundRevision + 1, "updated_at": now}).Error; e != nil {
			return e
		}
		response, e := json.Marshal(value)
		if e != nil {
			return e
		}
		if e = tx.Exec("UPDATE transaction_refunds SET response=?::jsonb WHERE id=?", string(response), value.ID).Error; e != nil {
			return e
		}
		result = value
		return nil
	})
	return
}
