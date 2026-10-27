package cash

import (
	"be-dikasa/internal/user"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

// MovementRepository owns transaction boundaries for permanent cash ledger entries.
type MovementRepository struct{ db *gorm.DB }

// NewMovementRepository shares the existing application's PostgreSQL connection.
func NewMovementRepository(db *gorm.DB) *MovementRepository { return &MovementRepository{db} }

// movementRow loads only scoped IDs; empty results expose no cross-outlet existence.
func movementRow(tx *gorm.DB, out, sid, id string) (MovementRecord, error) {
	var m MovementRecord
	e := tx.Where("id=? AND outlet_id=? AND cash_session_id=?", id, out, sid).First(&m).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		e = ErrNotFound
	}
	m.HasProof = m.ProofURL != nil
	return m, e
}

// execute serializes request replay before cash-session and original-entry locks.
func (r *MovementRepository) execute(ctx context.Context, actor, out, sid, reverse string, c MovementCommand, hash string, proof MovementProofStore, now func() time.Time) (result MovementRecord, err error) {
	var uploaded string
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		lock := sha256.Sum256([]byte("cash-movement:" + out + ":" + c.RequestID))
		if e := tx.Exec("SELECT pg_advisory_xact_lock(?)", int64(binary.BigEndian.Uint64(lock[:8]))).Error; e != nil {
			return e
		}
		var old MovementRecord
		e := tx.Where("outlet_id=? AND request_id=?", out, c.RequestID).First(&old).Error
		if e == nil {
			if old.PayloadHash != hash || old.CreatedBy != actor || old.CashSessionID != sid {
				return ErrMovementRequestUsed
			}
			old.HasProof = old.ProofURL != nil
			result = old
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		row, e := readCloseRow(tx, out, sid, true)
		if e != nil {
			return e
		}
		a, e := user.NewGORMRepository(tx).FindByIDWithRole(ctx, actor)
		if e != nil || a == nil || a.OutletID == nil || *a.OutletID != out || !mayClose(a, row.UserID) {
			return ErrCloseForbidden
		}
		if row.Status != StatusOpen {
			return ErrAlreadyClosed
		}
		if reverse != "" {
			var original MovementRecord
			e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND outlet_id=? AND cash_session_id=?", reverse, out, sid).First(&original).Error
			if errors.Is(e, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			if e != nil {
				return e
			}
			if original.ReversesID != nil {
				return ErrMovementReversed
			}
			if a.Role.Name != "manager" && original.CreatedBy != actor {
				return ErrCloseForbidden
			}
			var count int64
			if e = tx.Model(&MovementRecord{}).Where("reverses_id=?", reverse).Count(&count).Error; e != nil {
				return e
			}
			if count != 0 {
				return ErrMovementReversed
			}
			c.Amount = original.Amount
			c.Name = original.Name
			c.Kind = "in"
			if original.Kind == "in" {
				c.Kind = "out"
			}
		}
		snap, e := readCloseSnapshot(tx, row)
		if e != nil {
			return e
		}
		summary, e := Summarize(snap)
		if e != nil {
			return e
		}
		if c.Kind == "out" && c.Amount > summary.ExpectedCash {
			return ErrMovementInsufficient
		}
		m := MovementRecord{ID: uuid.NewString(), OutletID: out, CashSessionID: sid, RequestID: c.RequestID, CreatedBy: actor, CreatedByName: strings.TrimSpace(a.FirstName + " " + a.LastName), CreatedAt: now().UTC(), Kind: c.Kind, Amount: c.Amount, Name: c.Name, Reason: c.Reason, PayloadHash: hash}
		if reverse != "" {
			m.ReversesID = &reverse
		}
		candidate := MovementActivity{ID: m.ID, Kind: m.Kind, Amount: m.Amount}
		snap.Activity.Movements = append(snap.Activity.Movements, candidate)
		updated, e := Summarize(snap)
		if e != nil {
			return e
		}
		if _, e = FlowTotals(updated); e != nil {
			return e
		}
		if c.Proof != nil {
			if proof == nil {
				return ErrProofDisabled
			}
			o, e := proof.Save(ctx, out, sid, *c.Proof)
			if e != nil {
				return e
			}
			uploaded = o.URL
			m.ProofURL = &o.URL
			m.HasProof = true
		}
		if e = tx.Create(&m).Error; e != nil {
			return e
		}
		result = m
		return nil
	})
	if err != nil && uploaded != "" {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		var n int64
		if e := r.db.WithContext(cleanup).Model(&MovementRecord{}).Where("proof_url=?", uploaded).Count(&n).Error; e == nil && n == 0 {
			_ = proof.Delete(cleanup, uploaded)
		}
	}
	return
}

// MovementDetail augments immutable data with current correction eligibility.
type MovementDetail struct {
	Movement     MovementRecord `json:"movement"`
	CanReverse   bool           `json:"can_reverse"`
	ReversedByID *string        `json:"reversed_by_id"`
}

func (r *MovementRepository) detail(ctx context.Context, a *user.User, sid, id string) (d MovementDetail, err error) {
	d.Movement, err = movementRow(r.db.WithContext(ctx), *a.OutletID, sid, id)
	if err != nil {
		return
	}
	row, e := readCloseRow(r.db.WithContext(ctx), *a.OutletID, sid, false)
	if e != nil {
		return d, e
	}
	var ids []string
	if e = r.db.WithContext(ctx).Model(&MovementRecord{}).Where("reverses_id=? AND outlet_id=?", id, *a.OutletID).Pluck("id", &ids).Error; e != nil {
		return d, e
	}
	if len(ids) > 0 {
		d.ReversedByID = &ids[0]
	}
	d.CanReverse = row.Status == StatusOpen && mayClose(a, row.UserID) && (a.Role.Name == "manager" || d.Movement.CreatedBy == a.ID) && d.Movement.ReversesID == nil && d.ReversedByID == nil
	return
}
func (r *MovementRepository) byRequest(ctx context.Context, out, actor, sid, id string) (m MovementRecord, err error) {
	err = r.db.WithContext(ctx).Where("outlet_id=? AND created_by=? AND cash_session_id=? AND request_id=?", out, actor, sid, id).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ErrNotFound
	}
	m.HasProof = m.ProofURL != nil
	return
}
