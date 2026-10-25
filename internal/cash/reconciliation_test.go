package cash

import (
	"strings"
	"testing"
)

func snapshot() SessionSnapshot {
	return SessionSnapshot{ID: "session", OpeningCash: 100000, Status: "open", Activity: Activity{Receipts: []ReceiptActivity{{ID: "a", Method: "cash", Status: "paid", PaidAmount: 100000, ChangeAmount: 20000, TotalAmount: 80000}, {ID: "b", Method: "qris", Status: "paid", PaidAmount: 50000, TotalAmount: 50000}}, Refunds: []RefundActivity{{ID: "r", Method: "cash", TotalAmount: 10000}, {ID: "s", Method: "transfer", TotalAmount: 5000}}}}
}
func TestSummarizeCashAndNoncash(t *testing.T) {
	s, e := Summarize(snapshot())
	if e != nil || s.CashSales != 80000 || s.CashRefunds != 10000 || s.ExpectedCash != 170000 || s.SaleCount != 2 || s.RefundCount != 2 {
		t.Fatal(s, e)
	}
}
func TestRefundStatusDoesNotDoubleSubtract(t *testing.T) {
	r := snapshot()
	r.Activity.Receipts[0].Status = "refunded"
	r.Activity.Receipts[0].RefundRevision = 1
	s, e := Summarize(r)
	if e != nil || s.ExpectedCash != 170000 {
		t.Fatal(s, e)
	}
	r.Activity.Receipts = nil
	s, e = Summarize(r)
	if e != nil || s.ExpectedCash != 90000 {
		t.Fatal(s, e)
	}
}
func TestClosePreviewCountReasonAndHash(t *testing.T) {
	r := snapshot()
	for _, c := range []int64{0, 169000, 171000} {
		p, e := BuildClosePreview(r, CloseSelection{CountedCash: c, Reason: "Counted twice"})
		if e != nil || p.Difference != c-170000 {
			t.Fatal(p, e)
		}
	}
	if _, e := BuildClosePreview(r, CloseSelection{CountedCash: 169000}); e == nil {
		t.Fatal("missing reason accepted")
	}
	p, e := BuildClosePreview(r, CloseSelection{CountedCash: 170000})
	if e != nil {
		t.Fatal(e)
	}
	q, _ := BuildClosePreview(r, CloseSelection{CountedCash: 170000, Reason: "note"})
	if p.PreviewHash == q.PreviewHash {
		t.Fatal("unbound reason")
	}
	for _, s := range []CloseSelection{{CountedCash: -1}, {CountedCash: 170000, Reason: strings.Repeat("字", 501)}} {
		if _, e = BuildClosePreview(r, s); e == nil {
			t.Fatal("invalid count/reason")
		}
	}
}
func TestSummarizeUnsupportedAndOverflow(t *testing.T) {
	for _, kind := range []string{"legacy", "change", "overflow", "method"} {
		r := snapshot()
		switch kind {
		case "legacy":
			r.Activity.Receipts[0].Status = "cancelled"
		case "change":
			r.Activity.Receipts[0].ChangeAmount = 100001
		case "overflow":
			r.OpeningCash = 9007199254740991
		case "method":
			r.Activity.Refunds[0].Method = "point"
		}
		if _, e := Summarize(r); e == nil {
			t.Fatal(kind)
		}
	}
	r := snapshot()
	r.OpeningCash = 0
	r.Activity.Receipts = nil
	p, e := BuildClosePreview(r, CloseSelection{Reason: "Prior sale refund"})
	if e != nil || p.Summary.ExpectedCash != -10000 || p.Difference != 10000 {
		t.Fatal(p, e)
	}
}
