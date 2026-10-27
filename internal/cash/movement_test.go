package cash

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestMovementReconciliationExact(t *testing.T) {
	s := SessionSnapshot{ID: "s", OpeningCash: 100000, Activity: Activity{Receipts: []ReceiptActivity{{ID: "sale", Method: "cash", Status: "paid", PaidAmount: 60000, ChangeAmount: 10000, TotalAmount: 50000}}, Refunds: []RefundActivity{{ID: "r", Method: "cash", TotalAmount: 10000}}, Movements: []MovementActivity{{ID: "i", Kind: "in", Amount: 20000}, {ID: "o", Kind: "out", Amount: 30000}}}}
	r, e := Summarize(s)
	if e != nil || r.ExpectedCash != 130000 || r.CashIn != 20000 || r.CashOut != 30000 || r.MovementCount != 2 {
		t.Fatalf("summary %+v %v", r, e)
	}
	f, e := FlowTotals(r)
	if e != nil || f.TotalIn != 170000 || f.TotalOut != 40000 {
		t.Fatal(f, e)
	}
	p, e := BuildClosePreview(s, CloseSelection{CountedCash: 130000})
	if e != nil {
		t.Fatal(e)
	}
	s.Activity.Movements = append(s.Activity.Movements, MovementActivity{ID: "x", Kind: "in", Amount: 30000}, MovementActivity{ID: "y", Kind: "out", Amount: 30000})
	p2, e := BuildClosePreview(s, CloseSelection{CountedCash: 130000})
	if e != nil || p.PreviewHash == p2.PreviewHash {
		t.Fatal("zero net activity must stale preview", e)
	}
	var old ReconciliationSummary
	if e = json.Unmarshal([]byte(`{"expected_cash":123}`), &old); e != nil || old.ExpectedCash != 123 || old.CashIn != 0 {
		t.Fatal("legacy summary changed", e)
	}
}
func TestMovementInvalidAndOverflow(t *testing.T) {
	for _, ms := range [][]MovementActivity{{{ID: "a", Kind: "in", Amount: 0}}, {{ID: "a", Kind: "out", Amount: -1}}, {{ID: "a", Kind: "bad", Amount: 1}}, {{ID: "a", Kind: "in", Amount: 1}, {ID: "a", Kind: "in", Amount: 1}}, {{ID: "a", Kind: "in", Amount: maxOpeningCash}, {ID: "b", Kind: "in", Amount: 1}}} {
		_, e := Summarize(SessionSnapshot{ID: "s", Activity: Activity{Movements: ms}})
		if !errors.Is(e, ErrCloseUnsupported) {
			t.Fatal("invalid movement accepted", ms, e)
		}
	}
}
