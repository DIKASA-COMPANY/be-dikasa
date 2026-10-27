package reports

import (
	"be-dikasa/internal/payment"
	"context"
	"database/sql"
	"gorm.io/gorm"
	"time"
)

// GORMRepository reads original ledgers without current catalog joins.
type GORMRepository struct{ db *gorm.DB }

// NewGORMRepository shares the existing POS database.
func NewGORMRepository(db *gorm.DB) *GORMRepository { return &GORMRepository{db} }

// ReadSnapshot loads period sales and period refunds plus any older original parents.
func (r *GORMRepository) ReadSnapshot(ctx context.Context, out string, w Window) (s Snapshot, e error) {
	e = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		s.GeneratedAt = time.Now().UTC()
		if err := tx.Where("outlet_id=? AND created_at>=? AND created_at<?", out, w.Start, w.EndExclusive).Preload("Items").Order("id").Find(&s.Refunds).Error; err != nil {
			return err
		}
		var ids []string
		if err := tx.Table("transactions").Where("outlet_id=? AND paid_at>=? AND paid_at<?", out, w.Start, w.EndExclusive).Pluck("id", &ids).Error; err != nil {
			return err
		}
		for _, f := range s.Refunds {
			ids = append(ids, f.TransactionID)
		}
		if len(ids) == 0 {
			return nil
		}
		var originals []payment.Transaction
		if err := tx.Where("outlet_id=? AND id IN ?", out, ids).Preload("Items", func(q *gorm.DB) *gorm.DB { return q.Order("id") }).Order("id").Find(&originals).Error; err != nil {
			return err
		}
		for _, t := range originals {
			s.Receipts = append(s.Receipts, Receipt{Transaction: t})
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return
}
