package refund

import (
	"github.com/google/uuid"
	"reflect"
	"strings"
	"testing"
)

func receipt() ReceiptSnapshot {
	return ReceiptSnapshot{ID: uuid.NewString(), Status: "paid", Method: "cash", Subtotal: 30000, TaxAmount: 3000, TotalAmount: 33000, Items: []ReceiptItem{{ID: uuid.NewString(), ProductID: uuid.NewString(), Name: "Tea", UnitPrice: 10000, Quantity: 3, LineTotal: 30000}}}
}
func selection(r ReceiptSnapshot, q int) Selection {
	return Selection{Mode: "items", Method: "cash", Reason: "Customer return", Items: []SelectedItem{{TransactionItemID: r.Items[0].ID, Quantity: q}}}
}
func TestAllocate(t *testing.T) {
	a, e := Allocate(1, []int64{1, 1, 1}, []string{"a", "b", "c"})
	if e != nil || !reflect.DeepEqual(a, []int64{1, 0, 0}) {
		t.Fatal(a, e)
	}
	a, e = Allocate(9007199254740991, []int64{9007199254740990, 1}, []string{"a", "b"})
	if e != nil || a[0]+a[1] != 9007199254740991 {
		t.Fatal(a, e)
	}
	for _, w := range [][]int64{{0, 0}, {-1, 2}} {
		if _, e = Allocate(1, w, []string{"a", "b"}); e == nil {
			t.Fatal("bad weights accepted")
		}
	}
}
func TestBuildPreviewPartialThenFull(t *testing.T) {
	r := receipt()
	s := selection(r, 1)
	p, e := BuildPreview(r, s)
	if e != nil || p.TotalAmount != 11000 {
		t.Fatal(p, e)
	}
	r.Items[0].RefundedQuantity = 1
	r.RefundRevision = 1
	r.Status = "partially_refunded"
	s = selection(r, 2)
	s.Mode = "full"
	s.ExpectedRevision = 1
	p, e = BuildPreview(r, s)
	if e != nil || p.TotalAmount != 22000 || p.ResultStatus != "refunded" {
		t.Fatal(p, e)
	}
}
func TestTaxRoundingAndFreeItems(t *testing.T) {
	r := receipt()
	r.TaxAmount = 1
	r.TotalAmount = 30001
	for k, want := range []int64{0, 0, 1} {
		r.Items[0].RefundedQuantity = k
		p, e := BuildPreview(r, selection(r, 1))
		if e != nil || p.TaxAmount != want {
			t.Fatal(k, p, e)
		}
	}
	r = receipt()
	r.Items[0].UnitPrice = 0
	r.Items[0].LineTotal = 0
	r.Subtotal = 0
	r.TaxAmount = 0
	r.TotalAmount = 0
	p, e := BuildPreview(r, selection(r, 3))
	if e != nil || p.TotalAmount != 0 || p.ResultStatus != "refunded" {
		t.Fatal(p, e)
	}
}
func TestInvalidSelections(t *testing.T) {
	for _, name := range []string{"zero", "too_many", "duplicate", "return", "reason", "method", "combined_method", "stale", "legacy", "snapshot"} {
		t.Run(name, func(t *testing.T) {
			r := receipt()
			s := selection(r, 1)
			switch name {
			case "zero":
				s.Items[0].Quantity = 0
			case "too_many":
				s.Items[0].Quantity = 4
			case "duplicate":
				s.Items = append(s.Items, s.Items[0])
			case "return":
				s.Items[0].RestockQuantity = 2
			case "reason":
				s.Reason = strings.Repeat("字", 501)
			case "method":
				s.Method = "bad"
			case "combined_method":
				s.Method = "cash|wallet"
			case "stale":
				s.ExpectedRevision = 1
			case "legacy":
				r.Status = "refunded"
			case "snapshot":
				r.TotalAmount++
			}
			if _, e := BuildPreview(r, s); e == nil {
				t.Fatal("invalid accepted")
			}
		})
	}
}
func TestSelectionHash(t *testing.T) {
	r := receipt()
	s := selection(r, 1)
	p, e := BuildPreview(r, s)
	if e != nil {
		t.Fatal(e)
	}
	s.Reason = "other"
	q, _ := BuildPreview(r, s)
	if p.SelectionHash == q.SelectionHash {
		t.Fatal("changed reason matches preview")
	}
}

// TestDiscountVariantBudgetsConserve verifies distinct sale lines sharing a parent.
func TestDiscountVariantBudgetsConserve(t *testing.T) {
	r := receipt()
	variant := uuid.NewString()
	r.Items = []ReceiptItem{
		{ID: "00000000-0000-0000-0000-000000000001", ProductID: r.Items[0].ProductID, Name: "Base", UnitPrice: 100, Quantity: 3, LineTotal: 300},
		{ID: "00000000-0000-0000-0000-000000000002", ProductID: r.Items[0].ProductID, VariantID: &variant, Name: "Variant", UnitPrice: 200, Quantity: 1, LineTotal: 200},
	}
	r.Subtotal = 500
	r.DiscountAmount = 101
	r.TaxAmount = 1
	r.TotalAmount = 400
	var total, net, tax int64
	for _, index := range []int{0, 1, 0, 0} {
		s := selection(r, 1)
		s.Items[0].TransactionItemID = r.Items[index].ID
		p, e := BuildPreview(r, s)
		if e != nil {
			t.Fatal(e)
		}
		total += p.TotalAmount
		net += p.NetAmount
		tax += p.TaxAmount
		r.Items[index].RefundedQuantity++
	}
	if total != 400 || net != 399 || tax != 1 {
		t.Fatal(total, net, tax)
	}
}
