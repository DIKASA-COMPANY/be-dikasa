package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrInsufficientStock rejects orders that exceed the current shared inventory balance.
var ErrInsufficientStock = errors.New("insufficient product stock")

// ErrCatalogChanged requires cart review when a product, variant, availability or price has changed.
var ErrCatalogChanged = errors.New("catalog changed; review order")

// EffectiveStockStatus combines the manager sales pause with optional quantitative inventory.
func (p Product) EffectiveStockStatus() string {
	if p.StockStatus == StockStatusOutOfStock || (p.TrackStock && p.StockQuantity == 0) {
		return StockStatusOutOfStock
	}
	if p.TrackStock && p.StockQuantity <= p.LowStockThreshold {
		return "low_stock"
	}
	return StockStatusAvailable
}

// StockMovement is an immutable shared-balance audit entry; request IDs deduplicate manager writes.
type StockMovement struct {
	RefundID          *string   `gorm:"type:uuid" json:"refund_id,omitempty"`
	ID                string    `gorm:"type:uuid;primaryKey" json:"id"`
	OutletID          string    `gorm:"type:uuid" json:"-"`
	ProductID         string    `gorm:"type:uuid" json:"product_id"`
	TransactionID     *string   `gorm:"type:uuid" json:"transaction_id,omitempty"`
	CreatedBy         string    `gorm:"type:uuid" json:"created_by"`
	RequestID         *string   `gorm:"type:uuid" json:"request_id,omitempty"`
	PayloadHash       string    `json:"-"`
	Kind              string    `json:"kind"`
	QuantityDelta     int64     `json:"quantity_delta"`
	BalanceAfter      int64     `json:"balance_after"`
	TrackStock        bool      `json:"track_stock"`
	LowStockThreshold int64     `json:"low_stock_threshold"`
	Reason            string    `json:"reason"`
	CreatedAt         time.Time `json:"created_at"`
}

// StockCommand configures tracking or explicitly adjusts a balance; ordinary catalog PATCH never writes it.
type StockCommand struct {
	RequestID         string `json:"request_id" format:"uuid"`
	Reason            string `json:"reason" minLength:"1" maxLength:"500"`
	QuantityDelta     int64  `json:"quantity_delta,omitempty" minimum:"-1000000000" maximum:"1000000000"`
	TrackStock        *bool  `json:"track_stock,omitempty"`
	InitialQuantity   *int64 `json:"initial_quantity,omitempty" minimum:"0" maximum:"1000000000"`
	LowStockThreshold *int64 `json:"low_stock_threshold,omitempty" minimum:"0" maximum:"1000000000"`
}

// StockService owns manager inventory operations with row locks and an immutable audit trail.
type StockService struct {
	db       *gorm.DB
	accounts AccountFinder
}

// NewStockService creates the inventory boundary using the same PostgreSQL database as checkout.
func NewStockService(db *gorm.DB, accounts AccountFinder) *StockService {
	return &StockService{db: db, accounts: accounts}
}
func (s *StockService) scope(ctx context.Context, userID string) (string, error) {
	return (&Service{accounts: s.accounts}).scope(ctx, userID, true)
}

// Execute applies a deduplicated configuration or adjustment under a product row lock.
func (s *StockService) Execute(ctx context.Context, userID, productID, kind string, cmd StockCommand) (*StockMovement, error) {
	outlet, err := s.scope(ctx, userID)
	if err != nil {
		return nil, err
	}
	if _, err = uuid.Parse(cmd.RequestID); err != nil {
		return nil, ErrValidation
	}
	cmd.Reason = strings.TrimSpace(cmd.Reason)
	if cmd.Reason == "" || len([]rune(cmd.Reason)) > 500 || (kind != "adjust" && kind != "configure") {
		return nil, ErrValidation
	}
	payload, _ := json.Marshal(struct {
		ProductID, Kind string
		Command         StockCommand
	}{productID, kind, cmd})
	hash := sha256.Sum256(payload)
	fingerprint := hex.EncodeToString(hash[:])
	var result StockMovement
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var p Product
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND outlet_id = ?", productID, outlet).First(&p).Error; e != nil {
			return normalizeRepositoryError(e)
		}
		var previous StockMovement
		e := tx.Where("outlet_id = ? AND request_id = ?", outlet, cmd.RequestID).First(&previous).Error
		if e == nil {
			if previous.PayloadHash != fingerprint {
				return ErrConflict
			}
			result = previous
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		old := p.StockQuantity
		if e = applyStockCommand(&p, kind, cmd); e != nil {
			return e
		}
		if e = tx.Model(&Product{}).Where("id = ? AND outlet_id = ?", productID, outlet).Updates(map[string]any{"track_stock": p.TrackStock, "stock_quantity": p.StockQuantity, "low_stock_threshold": p.LowStockThreshold}).Error; e != nil {
			return e
		}
		result = StockMovement{ID: uuid.NewString(), OutletID: outlet, ProductID: productID, CreatedBy: userID, RequestID: &cmd.RequestID, PayloadHash: fingerprint, Kind: kind, QuantityDelta: p.StockQuantity - old, BalanceAfter: p.StockQuantity, TrackStock: p.TrackStock, LowStockThreshold: p.LowStockThreshold, Reason: cmd.Reason, CreatedAt: time.Now().UTC()}
		return normalizeRepositoryError(tx.Create(&result).Error)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// applyStockCommand prevents resetting a live balance through a stale configuration form.
func applyStockCommand(p *Product, kind string, cmd StockCommand) error {
	if kind == "adjust" {
		if !p.TrackStock || cmd.QuantityDelta == 0 || cmd.QuantityDelta < -1_000_000_000 || cmd.QuantityDelta > 1_000_000_000 || cmd.TrackStock != nil || cmd.InitialQuantity != nil || cmd.LowStockThreshold != nil {
			return ErrValidation
		}
		next := p.StockQuantity + cmd.QuantityDelta
		if next < 0 || next > 1_000_000_000 {
			return ErrInsufficientStock
		}
		p.StockQuantity = next
		return nil
	}
	if cmd.TrackStock == nil || cmd.QuantityDelta != 0 {
		return ErrValidation
	}
	if *cmd.TrackStock && !p.TrackStock {
		if cmd.InitialQuantity == nil || *cmd.InitialQuantity < 0 || *cmd.InitialQuantity > 1_000_000_000 {
			return ErrValidation
		}
		p.StockQuantity = *cmd.InitialQuantity
	} else if cmd.InitialQuantity != nil {
		return ErrConflict
	}
	if cmd.LowStockThreshold != nil {
		if *cmd.LowStockThreshold < 0 || *cmd.LowStockThreshold > 1_000_000_000 {
			return ErrValidation
		}
		p.LowStockThreshold = *cmd.LowStockThreshold
	}
	p.TrackStock = *cmd.TrackStock
	return nil
}

// Movements returns one manager-scoped page of a product's immutable stock history.
func (s *StockService) Movements(ctx context.Context, userID, productID string, page, size int) ([]StockMovement, int64, error) {
	outlet, err := s.scope(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	if page < 1 || size < 1 || size > 100 {
		return nil, 0, ErrValidation
	}
	var p Product
	if err = s.db.WithContext(ctx).Where("id = ? AND outlet_id = ?", productID, outlet).First(&p).Error; err != nil {
		return nil, 0, normalizeRepositoryError(err)
	}
	query := s.db.WithContext(ctx).Model(&StockMovement{}).Where("outlet_id = ? AND product_id = ?", outlet, productID)
	var total int64
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := []StockMovement{}
	err = query.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error
	return rows, total, err
}

// StockSaleItem contains only the catalog reference and amount needed for atomic checkout validation.
type StockSaleItem struct {
	ProductID string
	VariantID *string
	UnitPrice int64
	Quantity  int
}

// validateStockSale verifies the chosen prices/variants and aggregates all lines against the parent balance.
func validateStockSale(p Product, items []StockSaleItem) (int64, error) {
	if !p.IsActive || !p.Category.IsActive || p.StockStatus == StockStatusOutOfStock {
		return 0, ErrCatalogChanged
	}
	var quantity int64
	for _, item := range items {
		price := p.Price
		if item.VariantID != nil {
			found := false
			for _, v := range p.Variants {
				if v.ID == *item.VariantID && v.ProductID == p.ID && v.IsActive && v.StockStatus != StockStatusOutOfStock {
					price = v.Price
					found = true
					break
				}
			}
			if !found {
				return 0, ErrCatalogChanged
			}
		}
		if item.UnitPrice != price || item.Quantity < 1 {
			return 0, ErrCatalogChanged
		}
		quantity += int64(item.Quantity)
	}
	if p.TrackStock && quantity > p.StockQuantity {
		return 0, ErrInsufficientStock
	}
	return quantity, nil
}

// DeductSaleStock locks products in stable order and deducts shared balances inside the receipt transaction.
func DeductSaleStock(ctx context.Context, tx *gorm.DB, outlet, userID, transactionID string, items []StockSaleItem) error {
	grouped := map[string][]StockSaleItem{}
	for _, item := range items {
		grouped[item.ProductID] = append(grouped[item.ProductID], item)
	}
	ids := make([]string, 0, len(grouped))
	for id := range grouped {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		var p Product
		if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Category").Preload("Variants").Where("id = ? AND outlet_id = ?", id, outlet).First(&p).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCatalogChanged
			}
			return err
		}
		quantity, err := validateStockSale(p, grouped[id])
		if err != nil {
			return err
		}
		if !p.TrackStock {
			continue
		}
		p.StockQuantity -= quantity
		if err = tx.Model(&Product{}).Where("id = ? AND outlet_id = ?", id, outlet).Update("stock_quantity", p.StockQuantity).Error; err != nil {
			return err
		}
		movement := StockMovement{ID: uuid.NewString(), OutletID: outlet, ProductID: id, TransactionID: &transactionID, CreatedBy: userID, Kind: "sale", QuantityDelta: -quantity, BalanceAfter: p.StockQuantity, TrackStock: true, LowStockThreshold: p.LowStockThreshold, Reason: "Penjualan", CreatedAt: time.Now().UTC()}
		if err = tx.Create(&movement).Error; err != nil {
			return err
		}
	}
	return nil
}
