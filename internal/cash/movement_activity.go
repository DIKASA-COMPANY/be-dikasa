package cash

import (
	"be-dikasa/internal/user"
	"context"
	"database/sql"
	"gorm.io/gorm"
	"sort"
	"time"
)

// CashActivityEvent projects original cash sources without copying or editing them.
type CashActivityEvent struct {
	ID            string    `json:"id"`
	SourceID      string    `json:"source_id"`
	Kind          string    `json:"kind"`
	Direction     string    `json:"direction"`
	Amount        int64     `json:"amount"`
	Name          string    `json:"name"`
	Reason        string    `json:"reason"`
	OccurredAt    time.Time `json:"occurred_at"`
	CreatedBy     string    `json:"created_by"`
	CreatedByName string    `json:"created_by_name"`
	ReversesID    *string   `json:"reverses_id"`
}

// MovementCapabilities are authoritative for the selected account and session.
type MovementCapabilities struct {
	CanCreate      bool `json:"can_create"`
	CanUploadProof bool `json:"can_upload_proof"`
}

// ActivityData carries full-session totals independently of the visible page.
type ActivityData struct {
	Session      SessionRecord        `json:"session"`
	Totals       *CashFlowTotals      `json:"totals"`
	Capabilities MovementCapabilities `json:"capabilities"`
	Events       []CashActivityEvent  `json:"events"`
}

// ActivityMeta bounds history pagination.
type ActivityMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// ActivityPage wraps the source projection using the existing data/meta contract.
type ActivityPage struct {
	ActivityData `json:"data"`
	Meta         ActivityMeta `json:"meta"`
}

func (r *MovementRepository) activity(ctx context.Context, a *user.User, sid string, page, size int, proof bool) (p ActivityPage, err error) {
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row, e := readCloseRow(tx, *a.OutletID, sid, false)
		if e != nil {
			return e
		}
		p.Session, e = recordFromRow(tx, row)
		if e != nil {
			return e
		}
		p.Session.CanClose = row.Status == StatusOpen && p.Session.Supported && mayClose(a, row.UserID)
		p.Capabilities = MovementCapabilities{row.Status == StatusOpen && p.Session.Supported && mayClose(a, row.UserID), proof && row.Status == StatusOpen && p.Session.Supported && mayClose(a, row.UserID)}
		if p.Session.Summary != nil {
			f, e := FlowTotals(*p.Session.Summary)
			if e != nil {
				return e
			}
			p.Totals = &f
		}
		events := []CashActivityEvent{{ID: "opening/" + sid, SourceID: sid, Kind: "opening", Direction: "in", Amount: row.OpeningCash, Name: "Modal Awal", OccurredAt: row.OpenedAt, CreatedBy: row.UserID, CreatedByName: row.OpeningUserName}}
		var sales []CashActivityEvent
		e = tx.Table("transactions t").Select("t.id AS source_id, 'sale' AS kind, 'in' AS direction, t.paid_amount-t.change_amount AS amount, 'Penjualan '||t.transaction_number AS name, '' AS reason, t.paid_at AS occurred_at,t.created_by,TRIM(u.first_name||' '||u.last_name) AS created_by_name").Joins("LEFT JOIN users u ON u.id=t.created_by").Where("t.outlet_id=? AND t.cash_session_id=? AND t.payment_method='cash'", *a.OutletID, sid).Scan(&sales).Error
		if e != nil {
			return e
		}
		for _, v := range sales {
			v.ID = "sale/" + v.SourceID
			events = append(events, v)
		}
		var refunds []CashActivityEvent
		e = tx.Table("transaction_refunds r").Select("r.id AS source_id,'refund' AS kind,'out' AS direction,r.total_amount AS amount,'Refund' AS name,r.reason,r.created_at AS occurred_at,r.actor_id AS created_by,r.actor_name AS created_by_name").Where("r.outlet_id=? AND r.cash_session_id=? AND r.method='cash'", *a.OutletID, sid).Scan(&refunds).Error
		if e != nil {
			return e
		}
		for _, v := range refunds {
			v.ID = "refund/" + v.SourceID
			events = append(events, v)
		}
		var moves []MovementRecord
		if e = tx.Where("outlet_id=? AND cash_session_id=?", *a.OutletID, sid).Find(&moves).Error; e != nil {
			return e
		}
		for _, m := range moves {
			kind := "movement"
			if m.ReversesID != nil {
				kind = "reversal"
			}
			events = append(events, CashActivityEvent{ID: "movement/" + m.ID, SourceID: m.ID, Kind: kind, Direction: m.Kind, Amount: m.Amount, Name: m.Name, Reason: m.Reason, OccurredAt: m.CreatedAt, CreatedBy: m.CreatedBy, CreatedByName: m.CreatedByName, ReversesID: m.ReversesID})
		}
		sort.Slice(events, func(i, j int) bool {
			if events[i].OccurredAt.Equal(events[j].OccurredAt) {
				return events[i].ID < events[j].ID
			}
			return events[i].OccurredAt.Before(events[j].OccurredAt)
		})
		p.Meta = ActivityMeta{page, size, int64(len(events))}
		start := (page - 1) * size
		if start > len(events) {
			start = len(events)
		}
		end := start + size
		if end > len(events) {
			end = len(events)
		}
		p.Events = events[start:end]
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return
}
