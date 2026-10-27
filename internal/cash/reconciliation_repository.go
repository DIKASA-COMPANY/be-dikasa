package cash

import (
	"be-dikasa/internal/user"
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
	"strings"
	"time"
)

// ReconciliationRepository reads session activity and atomically freezes closure.
type ReconciliationRepository struct{ db *gorm.DB }

// NewReconciliationRepository leaves existing opening/current interfaces unchanged.
func NewReconciliationRepository(db *gorm.DB) *ReconciliationRepository {
	return &ReconciliationRepository{db}
}

type closeRow struct {
	Session
	OpeningUserName  string
	ClosePayloadHash string
	ClosingSummary   []byte
}

// readCloseRow scopes before optionally locking a session row.
func readCloseRow(tx *gorm.DB, out, id string, lock bool) (closeRow, error) {
	var row closeRow
	q := tx.Table("cash_sessions AS c").Select("c.*, TRIM(u.first_name || ' ' || u.last_name) AS opening_user_name").Joins("JOIN users u ON u.id=c.user_id").Where("c.outlet_id=? AND c.id=?", out, id)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "c"}})
	}
	e := q.Take(&row).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		e = ErrNotFound
	}
	return row, e
}

// readCloseSnapshot reads original receipts and refund-session payouts without fan-out.
func readCloseSnapshot(tx *gorm.DB, row closeRow) (SessionSnapshot, error) {
	s := SessionSnapshot{ID: row.ID, OutletID: row.OutletID, OpeningUserID: row.UserID, OpeningUserName: row.OpeningUserName, Status: row.Status, OpenedAt: row.OpenedAt, OpeningCash: row.OpeningCash}
	e := tx.Table("transactions").Select("id,payment_method AS method,status,refund_revision,paid_amount,change_amount,total_amount").Where("outlet_id=? AND cash_session_id=?", row.OutletID, row.ID).Order("id").Scan(&s.Activity.Receipts).Error
	if e != nil {
		return s, e
	}
	e = tx.Table("transaction_refunds").Select("id,method,total_amount").Where("outlet_id=? AND cash_session_id=?", row.OutletID, row.ID).Order("id").Scan(&s.Activity.Refunds).Error
	if e != nil {
		return s, e
	}
	e = tx.Table("cash_movements").Select("id,kind,amount,created_by,created_by_name,name,reason,created_at,reverses_id").Where("outlet_id=? AND cash_session_id=?", row.OutletID, row.ID).Order("id").Scan(&s.Activity.Movements).Error
	return s, e
}

// recordFromRow uses saved closing JSON rather than recalculating historical balances.
func recordFromRow(tx *gorm.DB, row closeRow) (SessionRecord, error) {
	r := SessionRecord{SessionData: sessionResponse(&row.Session).Data, OpeningUserName: row.OpeningUserName}
	if row.Status == StatusClosed {
		if len(row.ClosingSummary) > 0 {
			var c ClosedReconciliation
			if e := json.Unmarshal(row.ClosingSummary, &c); e != nil {
				return r, e
			}
			r.Closing = &c
			r.Summary = &c.Summary
			r.Supported = true
		}
		return r, nil
	}
	s, e := readCloseSnapshot(tx, row)
	if e != nil {
		return r, e
	}
	summary, e := Summarize(s)
	if errors.Is(e, ErrCloseUnsupported) {
		r.UnsupportedReason = "Aktivitas lama atau nominal sesi tidak dapat diverifikasi."
		return r, nil
	}
	if e != nil {
		return r, e
	}
	r.Summary = &summary
	r.Supported = true
	return r, nil
}

// Detail reads one outlet-scoped session under a consistent snapshot.
func (r *ReconciliationRepository) Detail(ctx context.Context, out, id string) (result SessionRecord, e error) {
	e = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, e := readCloseRow(tx, out, id, false)
		if e != nil {
			return e
		}
		result, e = recordFromRow(tx, row)
		return e
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return
}

// List bounds rows independently of receipt/refund aggregation.
func (r *ReconciliationRepository) List(ctx context.Context, out string, page, size int) ([]SessionRecord, int64, error) {
	var count int64
	var ids []string
	q := r.db.WithContext(ctx).Table("cash_sessions").Where("outlet_id=?", out)
	if e := q.Count(&count).Error; e != nil {
		return nil, 0, e
	}
	if e := q.Order("opened_at DESC,id").Offset((page-1)*size).Limit(size).Pluck("id", &ids).Error; e != nil {
		return nil, 0, e
	}
	rows := []SessionRecord{}
	for _, id := range ids {
		row, e := r.Detail(ctx, out, id)
		if e != nil {
			return nil, 0, e
		}
		rows = append(rows, row)
	}
	return rows, count, nil
}

// PreviewSnapshot reads both activity sources at the same database revision.
func (r *ReconciliationRepository) PreviewSnapshot(ctx context.Context, out, id string) (snap SessionSnapshot, e error) {
	e = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, e := readCloseRow(tx, out, id, false)
		if e != nil {
			return e
		}
		if row.Status != StatusOpen {
			return ErrAlreadyClosed
		}
		snap, e = readCloseSnapshot(tx, row)
		return e
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return
}

// Close serializes command replay and all session writers before saving immutable cash.
func (r *ReconciliationRepository) Close(ctx context.Context, out, actor, id string, c CloseCommand) (result ClosedReconciliation, e error) {
	c.Reason = strings.TrimSpace(c.Reason)
	c.RequestID = uuid.MustParse(c.RequestID).String()
	raw, _ := json.Marshal(struct {
		Actor, Session string
		Command        CloseCommand
	}{actor, id, c})
	digest := sha256.Sum256(raw)
	hash := hex.EncodeToString(digest[:])
	lock := sha256.Sum256([]byte("cash-close:" + out + ":" + c.RequestID))
	e = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec("SELECT pg_advisory_xact_lock(?)", int64(binary.BigEndian.Uint64(lock[:8]))).Error; e != nil {
			return e
		}
		var old closeRow
		e := tx.Table("cash_sessions").Where("outlet_id=? AND close_request_id=?", out, c.RequestID).Take(&old).Error
		if e == nil {
			if old.ClosePayloadHash != hash {
				return ErrCloseRequestUsed
			}
			return json.Unmarshal(old.ClosingSummary, &result)
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		row, e := readCloseRow(tx, out, id, true)
		if e != nil {
			return e
		}
		account, e := user.NewGORMRepository(tx).FindByIDWithRole(ctx, actor)
		if e != nil {
			return e
		}
		if !mayClose(account, row.UserID) {
			return ErrCloseForbidden
		}
		if row.Status != StatusOpen {
			return ErrAlreadyClosed
		}
		snapshot, e := readCloseSnapshot(tx, row)
		if e != nil {
			return e
		}
		if closePreviewHash(snapshot, c.CloseSelection) != c.PreviewHash {
			return ErrCloseStale
		}
		preview, e := BuildClosePreview(snapshot, c.CloseSelection)
		if e != nil {
			return e
		}
		if preview.PreviewHash != c.PreviewHash {
			return ErrCloseStale
		}
		now := time.Now().UTC()
		result = ClosedReconciliation{ClosePreview: preview, SessionID: id, ClosedBy: actor, ClosedByName: strings.TrimSpace(account.FirstName + " " + account.LastName), ClosedAt: now, RequestID: c.RequestID}
		response, e := json.Marshal(result)
		if e != nil {
			return e
		}
		return tx.Table("cash_sessions").Where("id=? AND outlet_id=?", id, out).Updates(map[string]any{"status": "closed", "closing_cash": preview.CountedCash, "closed_at": now, "updated_at": now, "closed_by": actor, "closed_by_name": result.ClosedByName, "closing_reason": preview.Reason, "expected_cash": preview.Summary.ExpectedCash, "cash_difference": preview.Difference, "close_request_id": c.RequestID, "close_payload_hash": hash, "close_preview_hash": c.PreviewHash, "closing_summary": gorm.Expr("?::jsonb", string(response))}).Error
	})
	return
}
