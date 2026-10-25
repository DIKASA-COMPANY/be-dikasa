package reports

import (
	"be-dikasa/internal/payment"
	"be-dikasa/internal/refund"
	"testing"
	"time"
)

func fixture() Snapshot {
	monday := time.Date(2026, 10, 19, 3, 0, 0, 0, time.UTC)
	wed := monday.AddDate(0, 0, 2)
	return Snapshot{Receipts: []Receipt{{Transaction: payment.Transaction{ID: "sale", PaymentMethod: "qris", Status: "partially_refunded", RefundRevision: 1, PaidAt: monday, Subtotal: 300, DiscountAmount: 30, TaxAmount: 27, TotalAmount: 297, Items: []payment.TransactionItem{{ID: "line", ProductID: "product", ProductName: "Tea", UnitPrice: 100, Quantity: 3, LineTotal: 300}}}}}, Refunds: []refund.Refund{{ID: "refund", TransactionID: "sale", Method: "cash", CreatedAt: wed, NetAmount: 90, TaxAmount: 9, TotalAmount: 99, Items: []refund.RefundItem{{TransactionItemID: "line", Quantity: 1, NetAmount: 90, TaxAmount: 9, TotalAmount: 99}}}}}
}
func TestRefundDateAndMethod(t *testing.T) {
	for _, tc := range []struct {
		day                      string
		gross, net, value, count int64
	}{{"2026-10-19", 300, 270, 297, 1}, {"2026-10-21", 0, -90, -99, 0}} {
		f, w, e := NormalizeFilter(Summary, Filter{StartDate: tc.day, EndDate: tc.day}, time.Now())
		if e != nil {
			t.Fatal(e)
		}
		r, e := Calculate(Summary, fixture(), f, w)
		if e != nil || r.Totals.GrossSales != tc.gross || r.Totals.NetSales != tc.net || r.Totals.ValueAfterRefunds != tc.value || r.Totals.SaleCount != tc.count {
			t.Fatal(tc, r, e)
		}
		if tc.count == 0 && r.Totals.AverageTransaction != nil {
			t.Fatal("divide zero")
		}
	}
}
func TestGroupConservationAndUnknownCategory(t *testing.T) {
	f, w, _ := NormalizeFilter(Products, Filter{StartDate: "2026-10-19", EndDate: "2026-10-21"}, time.Now())
	r, e := Calculate(Products, fixture(), f, w)
	if e != nil || len(r.Rows) != 1 || r.Totals.NetSales != 180 || r.Rows[0].NetTax != 18 || r.Rows[0].NetQuantity != 2 {
		t.Fatal(r, e)
	}
	r, e = Calculate(Categories, fixture(), f, w)
	if e != nil || r.Rows[0].Key != "unknown" || r.Totals.UnknownCategoryQuantity != 3 {
		t.Fatal(r, e)
	}
}
func TestWIBWindowAndValidation(t *testing.T) {
	_, w, e := NormalizeFilter(Summary, Filter{StartDate: "2026-10-24", EndDate: "2026-10-24"}, time.Now())
	if e != nil || w.Start.Format(time.RFC3339) != "2026-10-23T17:00:00Z" {
		t.Fatal(w, e)
	}
	for _, f := range []Filter{{StartDate: "2026-02-30", EndDate: "2026-03-01"}, {StartDate: "2026-10-24", EndDate: "2026-10-23"}, {StartDate: "2024-01-01", EndDate: "2025-01-01"}} {
		if _, _, e := NormalizeFilter(Summary, f, time.Now()); e == nil {
			t.Fatal(f)
		}
	}
	if _, _, e := NormalizeFilter(Summary, Filter{StartDate: "2024-01-01", EndDate: "2024-12-31"}, time.Now()); e != nil {
		t.Fatal(e)
	}
}
func TestEmptyAndLegacy(t *testing.T) {
	f, w, _ := NormalizeFilter(Summary, Filter{StartDate: "2026-10-19", EndDate: "2026-10-21"}, time.Now())
	r, e := Calculate(Summary, Snapshot{}, f, w)
	if e != nil || len(r.Daily) != 3 || r.Totals.SaleCount != 0 {
		t.Fatal(r, e)
	}
	s := fixture()
	s.Refunds = nil
	s.Receipts[0].Transaction.Status = "refunded"
	s.Receipts[0].Transaction.RefundRevision = 0
	r, e = Calculate(Summary, s, f, w)
	if e != nil || r.Totals.ExcludedReceipts != 1 || r.Totals.SaleCount != 0 {
		t.Fatal(r, e)
	}
}

func TestFilteredTotalsBeforePaginationAndMethods(t *testing.T) {
	s := fixture()
	_, w, _ := NormalizeFilter(Methods, Filter{StartDate: "2026-10-19", EndDate: "2026-10-21"}, time.Now())
	r, e := Calculate(Methods, s, Filter{}, w)
	if e != nil || len(r.Methods) != 2 {
		t.Fatal(r, e)
	}
	for _, m := range r.Methods {
		if m.Method == "cash" && (m.SaleCount != 0 || m.RefundAmount != 99 || m.NetAmount != -99) {
			t.Fatal(m)
		}
		if m.Method == "qris" && m.SalesAmount != 297 {
			t.Fatal(m)
		}
	}
	r, e = Calculate(Products, s, Filter{Query: "Tea", PageSize: 1}, w)
	if e != nil || r.Totals.NetSales != 180 {
		t.Fatal(r, e)
	}
	r, e = Calculate(Products, s, Filter{Query: "absent"}, w)
	if e != nil || len(r.Rows) != 0 || r.Totals.NetSales != 0 {
		t.Fatal(r, e)
	}
}
func TestOriginalOverflowExcludedAndDayBoundary(t *testing.T) {
	s := fixture()
	s.Refunds = nil
	s.Receipts[0].Transaction.Subtotal = 9007199254740992
	f, w, _ := NormalizeFilter(Summary, Filter{StartDate: "2026-10-19", EndDate: "2026-10-19"}, time.Now())
	r, e := Calculate(Summary, s, f, w)
	if e != nil || r.Totals.ExcludedReceipts != 1 {
		t.Fatal(r, e)
	}
	s = fixture()
	s.Refunds = nil
	s.Receipts[0].Transaction.PaidAt = w.EndExclusive
	r, e = Calculate(Summary, s, f, w)
	if e != nil || r.Totals.SaleCount != 0 {
		t.Fatal(r, e)
	}
}

func TestProductMixedHistoricalCategories(t *testing.T) {
	s := fixture()
	s.Refunds = nil
	second := s.Receipts[0]
	second.Transaction.ID = "sale2"
	second.Transaction.Items = append([]payment.TransactionItem(nil), second.Transaction.Items...)
	second.Transaction.Items[0].ID = "line2"
	id, name := "category", "Food"
	second.Transaction.Items[0].CategoryIDSnapshot = &id
	second.Transaction.Items[0].CategoryNameSnapshot = &name
	s.Receipts = append(s.Receipts, second)
	f, w, _ := NormalizeFilter(Products, Filter{StartDate: "2026-10-19", EndDate: "2026-10-19"}, time.Now())
	r, e := Calculate(Products, s, f, w)
	if e != nil || r.Rows[0].CategoryName != "Beragam" {
		t.Fatal(r.Rows, e)
	}
}

func TestMostRecentOriginalNameIncludesFractionalTime(t *testing.T) {
	s := fixture()
	s.Refunds = nil
	newer := s.Receipts[0]
	newer.Transaction.ID = "new"
	newer.Transaction.PaidAt = newer.Transaction.PaidAt.Add(100 * time.Millisecond)
	newer.Transaction.Items = append([]payment.TransactionItem(nil), newer.Transaction.Items...)
	newer.Transaction.Items[0].ID = "new-line"
	newer.Transaction.Items[0].ProductName = "Updated name"
	s.Receipts = append(s.Receipts, newer)
	f, w, _ := NormalizeFilter(Products, Filter{StartDate: "2026-10-19", EndDate: "2026-10-19"}, time.Now())
	r, e := Calculate(Products, s, f, w)
	if e != nil || r.Rows[0].Name != "Updated name" {
		t.Fatal(r.Rows, e)
	}
}
