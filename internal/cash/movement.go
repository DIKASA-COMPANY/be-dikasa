package cash

import (
	"errors"
	"math/big"
	"time"
)

// MovementActivity is canonical manual cash activity used in closing hashes.
type MovementActivity struct {
	ID, Kind                               string
	Amount                                 int64
	CreatedBy, CreatedByName, Name, Reason string
	CreatedAt                              time.Time
	ReversesID                             *string
}

// MovementRecord is an immutable manual drawer entry.
type MovementRecord struct {
	ID            string    `json:"id" gorm:"type:uuid;primaryKey"`
	OutletID      string    `json:"outlet_id" gorm:"type:uuid"`
	CashSessionID string    `json:"session_id" gorm:"type:uuid"`
	RequestID     string    `json:"request_id" gorm:"type:uuid"`
	CreatedBy     string    `json:"created_by" gorm:"type:uuid"`
	CreatedByName string    `json:"created_by_name"`
	CreatedAt     time.Time `json:"created_at"`
	Kind          string    `json:"kind"`
	Amount        int64     `json:"amount"`
	Name          string    `json:"name"`
	Reason        string    `json:"reason"`
	ReversesID    *string   `json:"reverses_id" gorm:"type:uuid"`
	PayloadHash   string    `json:"-"`
	ProofURL      *string   `json:"-" gorm:"column:proof_url"`
	HasProof      bool      `json:"has_proof" gorm:"-"`
}

// TableName sets the immutable cash ledger table.
func (MovementRecord) TableName() string { return "cash_movements" }

// CashFlowTotals separates drawer activity from sales reports.
type CashFlowTotals struct {
	Opening       int64 `json:"opening"`
	CashSales     int64 `json:"cash_sales"`
	CashRefunds   int64 `json:"cash_refunds"`
	CashIn        int64 `json:"cash_in"`
	CashOut       int64 `json:"cash_out"`
	TotalIn       int64 `json:"total_in"`
	TotalOut      int64 `json:"total_out"`
	ExpectedCash  int64 `json:"expected_cash"`
	MovementCount int   `json:"movement_count"`
}

// FlowTotals bounds full-session card totals without floating point arithmetic.
func FlowTotals(s ReconciliationSummary) (CashFlowTotals, error) {
	f := CashFlowTotals{Opening: s.OpeningCash, CashSales: s.CashSales, CashRefunds: s.CashRefunds, CashIn: s.CashIn, CashOut: s.CashOut, ExpectedCash: s.ExpectedCash, MovementCount: s.MovementCount}
	var e error
	n := new(big.Int).Add(big.NewInt(s.OpeningCash), big.NewInt(s.CashSales))
	n.Add(n, big.NewInt(s.CashIn))
	f.TotalIn, e = checkedCash(n)
	if e != nil {
		return f, e
	}
	f.TotalOut, e = sumCash(s.CashRefunds, s.CashOut)
	return f, e
}

// Movement domain failures distinguish rejected commands from unknown outcomes.
var (
	// ErrMovementInvalid rejects malformed manual cash input.
	ErrMovementInvalid = errors.New("invalid cash movement")
	// ErrMovementInsufficient refuses withdrawals beyond expected drawer cash.
	ErrMovementInsufficient = errors.New("insufficient drawer cash")
	// ErrMovementReversed rejects repeated or nested reversal.
	ErrMovementReversed = errors.New("cash movement already reversed")
	// ErrMovementRequestUsed rejects a changed command using an existing UUID.
	ErrMovementRequestUsed = errors.New("cash movement request reused")
	// ErrProofDisabled reports absent private proof storage.
	ErrProofDisabled = errors.New("private proof storage disabled")
	// ErrProofInvalid rejects invalid evidence.
	ErrProofInvalid = errors.New("invalid proof image")
	// ErrProofTooLarge enforces the encoded evidence limit.
	ErrProofTooLarge = errors.New("proof exceeds three MiB")
	// ErrProofStorage hides private storage failures.
	ErrProofStorage = errors.New("private proof storage unavailable")
)
