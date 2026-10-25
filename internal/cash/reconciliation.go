package cash

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"sort"
	"strings"
	"time"
)

// Reconciliation errors distinguish business refusals from database failures.
var (
	// ErrCloseInvalid rejects invalid count/reason input.
	ErrCloseInvalid = errors.New("invalid cash count or reason")
	// ErrCloseForbidden rejects unauthorized session closure.
	ErrCloseForbidden = errors.New("cash closure forbidden")
	// ErrCloseUnsupported marks activity without a trustworthy reconciliation.
	ErrCloseUnsupported = errors.New("reconciliation cannot be verified")
	// ErrCloseStale requires a new preview after activity changes.
	ErrCloseStale = errors.New("cash activity changed since preview")
	// ErrAlreadyClosed rejects a distinct close intent on a closed session.
	ErrAlreadyClosed = errors.New("cash session already closed")
	// ErrCloseRequestUsed rejects changed intent under an existing UUID.
	ErrCloseRequestUsed = errors.New("closing request already used")
)

// ReceiptActivity preserves the original sale independently of later refunds.
type ReceiptActivity struct {
	ID, Method, Status                                    string
	RefundRevision, PaidAmount, ChangeAmount, TotalAmount int64
}

// RefundActivity belongs to the session that actually paid the refund.
type RefundActivity struct {
	ID, Method  string
	TotalAmount int64
}

// Activity contains only receipts/refunds assigned to one session.
type Activity struct {
	Receipts  []ReceiptActivity
	Refunds   []RefundActivity
	Movements []MovementActivity
}

// SessionSnapshot is a consistent open-session calculation input.
type SessionSnapshot struct {
	ID, OutletID, OpeningUserID, OpeningUserName, Status string
	OpenedAt                                             time.Time
	OpeningCash                                          int64
	Activity                                             Activity
}

// MethodSummary separates payment methods without changing till cash.
type MethodSummary struct {
	Method        string `json:"method"`
	SaleCount     int    `json:"sale_count"`
	SalesAmount   int64  `json:"sales_amount"`
	RefundCount   int    `json:"refund_count"`
	RefundsAmount int64  `json:"refunds_amount"`
}

// ReconciliationSummary freezes the original session cash and noncash activity.
type ReconciliationSummary struct {
	SessionID       string          `json:"session_id"`
	OpeningUserID   string          `json:"opening_user_id"`
	OpeningUserName string          `json:"opening_user_name"`
	OpenedAt        time.Time       `json:"opened_at"`
	OpeningCash     int64           `json:"opening_cash"`
	CashSales       int64           `json:"cash_sales"`
	CashRefunds     int64           `json:"cash_refunds"`
	CashIn          int64           `json:"cash_in"`
	CashOut         int64           `json:"cash_out"`
	MovementCount   int             `json:"movement_count"`
	ExpectedCash    int64           `json:"expected_cash"`
	SaleCount       int             `json:"sale_count"`
	RefundCount     int             `json:"refund_count"`
	Methods         []MethodSummary `json:"methods"`
}

// CloseSelection contains only the cashier's physical count and explanation.
type CloseSelection struct {
	CountedCash int64  `json:"counted_cash" minimum:"0" maximum:"9007199254740991"`
	Reason      string `json:"reason" maxLength:"500"`
}

// ClosePreview binds a physical count to the reviewed session activity.
type ClosePreview struct {
	Summary     ReconciliationSummary `json:"summary"`
	CountedCash int64                 `json:"counted_cash"`
	Difference  int64                 `json:"difference"`
	Reason      string                `json:"reason"`
	PreviewHash string                `json:"preview_hash"`
}

// CloseCommand adds one secure request identity to an exact reviewed selection.
type CloseCommand struct {
	CloseSelection
	RequestID   string `json:"request_id" format:"uuid"`
	PreviewHash string `json:"preview_hash" minLength:"64" maxLength:"64"`
}

// ClosedReconciliation is an immutable successful closure response.
type ClosedReconciliation struct {
	ClosePreview
	SessionID    string    `json:"session_id"`
	ClosedBy     string    `json:"closed_by"`
	ClosedByName string    `json:"closed_by_name"`
	ClosedAt     time.Time `json:"closed_at"`
	RequestID    string    `json:"close_request_id"`
}

// SessionRecord carries live open activity or an immutable/legacy closing result.
type SessionRecord struct {
	SessionData
	OpeningUserName   string                 `json:"opening_user_name"`
	Summary           *ReconciliationSummary `json:"summary"`
	Closing           *ClosedReconciliation  `json:"closing"`
	Supported         bool                   `json:"reconciliation_supported"`
	UnsupportedReason string                 `json:"unsupported_reason,omitempty"`
	CanClose          bool                   `json:"can_close"`
}

// checkedCash bounds big integer intermediates before exposing JSON numbers.
func checkedCash(n *big.Int) (int64, error) {
	limit := big.NewInt(maxOpeningCash)
	if new(big.Int).Abs(new(big.Int).Set(n)).Cmp(limit) > 0 {
		return 0, ErrCloseUnsupported
	}
	return n.Int64(), nil
}
func sumCash(a, b int64) (int64, error) {
	return checkedCash(new(big.Int).Add(big.NewInt(a), big.NewInt(b)))
}
func supportedMethod(m string, refund bool) bool {
	switch m {
	case "cash", "wallet", "debit_card", "qris", "transfer":
		return true
	case "point":
		return !refund
	}
	return false
}

// Summarize sums original inflow and session-local refund outflow exactly once.
func Summarize(s SessionSnapshot) (ReconciliationSummary, error) {
	r := ReconciliationSummary{SessionID: s.ID, OpeningUserID: s.OpeningUserID, OpeningUserName: s.OpeningUserName, OpenedAt: s.OpenedAt, OpeningCash: s.OpeningCash, Methods: []MethodSummary{}}
	if s.ID == "" || s.OpeningCash < 0 || s.OpeningCash > maxOpeningCash {
		return r, ErrCloseUnsupported
	}
	methods := map[string]*MethodSummary{}
	get := func(m string) *MethodSummary {
		if methods[m] == nil {
			methods[m] = &MethodSummary{Method: m}
		}
		return methods[m]
	}
	var err error
	seen := map[string]bool{}
	for _, a := range s.Activity.Receipts {
		if a.ID == "" || seen[a.ID] || !supportedMethod(a.Method, false) || a.PaidAmount < 0 || a.ChangeAmount < 0 || a.PaidAmount > maxOpeningCash || a.ChangeAmount > a.PaidAmount || a.TotalAmount < 0 || a.TotalAmount > maxOpeningCash || ((a.Status == "cancelled" || a.Status == "refunded") && a.RefundRevision == 0) {
			return r, ErrCloseUnsupported
		}
		seen[a.ID] = true
		m := get(a.Method)
		m.SaleCount++
		r.SaleCount++
		m.SalesAmount, err = sumCash(m.SalesAmount, a.TotalAmount)
		if err != nil {
			return r, err
		}
		if a.Method == "cash" {
			r.CashSales, err = sumCash(r.CashSales, a.PaidAmount-a.ChangeAmount)
			if err != nil {
				return r, err
			}
		}
	}
	seen = map[string]bool{}
	for _, a := range s.Activity.Refunds {
		if a.ID == "" || seen[a.ID] || !supportedMethod(a.Method, true) || a.TotalAmount < 0 || a.TotalAmount > maxOpeningCash {
			return r, ErrCloseUnsupported
		}
		seen[a.ID] = true
		m := get(a.Method)
		m.RefundCount++
		r.RefundCount++
		m.RefundsAmount, err = sumCash(m.RefundsAmount, a.TotalAmount)
		if err != nil {
			return r, err
		}
		if a.Method == "cash" {
			r.CashRefunds, err = sumCash(r.CashRefunds, a.TotalAmount)
			if err != nil {
				return r, err
			}
		}
	}
	seen = map[string]bool{}
	for _, m := range s.Activity.Movements {
		if m.ID == "" || seen[m.ID] || (m.Kind != "in" && m.Kind != "out") || m.Amount < 1 || m.Amount > maxOpeningCash {
			return r, ErrCloseUnsupported
		}
		seen[m.ID] = true
		r.MovementCount++
		if m.Kind == "in" {
			r.CashIn, err = sumCash(r.CashIn, m.Amount)
		} else {
			r.CashOut, err = sumCash(r.CashOut, m.Amount)
		}
		if err != nil {
			return r, err
		}
	}
	n := new(big.Int).Add(big.NewInt(r.OpeningCash), big.NewInt(r.CashSales))
	n.Sub(n, big.NewInt(r.CashRefunds))
	n.Add(n, big.NewInt(r.CashIn))
	n.Sub(n, big.NewInt(r.CashOut))
	r.ExpectedCash, err = checkedCash(n)
	if err != nil {
		return r, err
	}
	keys := []string{}
	for key := range methods {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		r.Methods = append(r.Methods, *methods[key])
	}
	return r, nil
}

// BuildClosePreview validates discrepancy explanations and hashes canonical activity.
func BuildClosePreview(s SessionSnapshot, c CloseSelection) (ClosePreview, error) {
	var p ClosePreview
	if c.CountedCash < 0 || c.CountedCash > maxOpeningCash || len([]rune(c.Reason)) > 500 {
		return p, ErrCloseInvalid
	}
	r, e := Summarize(s)
	if e != nil {
		return p, e
	}
	difference, e := sumCash(c.CountedCash, -r.ExpectedCash)
	if e != nil {
		return p, e
	}
	c.Reason = strings.TrimSpace(c.Reason)
	if difference != 0 && c.Reason == "" {
		return p, ErrCloseInvalid
	}
	return ClosePreview{Summary: r, CountedCash: c.CountedCash, Difference: difference, Reason: c.Reason, PreviewHash: closePreviewHash(s, c)}, nil
}

// closePreviewHash binds the exact selection to canonical activity before discrepancy validation.
func closePreviewHash(s SessionSnapshot, c CloseSelection) string {
	c.Reason = strings.TrimSpace(c.Reason)
	s.Activity.Movements = append([]MovementActivity(nil), s.Activity.Movements...)
	sort.Slice(s.Activity.Movements, func(i, j int) bool { return s.Activity.Movements[i].ID < s.Activity.Movements[j].ID })
	s.Activity.Receipts = append([]ReceiptActivity(nil), s.Activity.Receipts...)
	s.Activity.Refunds = append([]RefundActivity(nil), s.Activity.Refunds...)
	sort.Slice(s.Activity.Receipts, func(i, j int) bool { return s.Activity.Receipts[i].ID < s.Activity.Receipts[j].ID })
	sort.Slice(s.Activity.Refunds, func(i, j int) bool { return s.Activity.Refunds[i].ID < s.Activity.Refunds[j].ID })
	raw, _ := json.Marshal(struct {
		Snapshot  SessionSnapshot
		Selection CloseSelection
	}{s, c})
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])

}
