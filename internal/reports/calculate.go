package reports

import (
	"be-dikasa/internal/payment"
	"be-dikasa/internal/refund"
	"math/big"
	"sort"
	"strings"
	"time"
)

const maxAmount int64 = 9007199254740991

// accumulator checks every exposed aggregate, including signed refund-only totals.
type accumulator struct{ err error }

func (a *accumulator) add(v *int64, n int64) {
	b := new(big.Int).Add(big.NewInt(*v), big.NewInt(n))
	if new(big.Int).Abs(new(big.Int).Set(b)).Cmp(big.NewInt(maxAmount)) > 0 {
		a.err = ErrUnsupported
		return
	}
	*v = b.Int64()
}
func finish(t *Totals, a *accumulator) {
	t.NetSales = t.SalesAfterDiscount
	a.add(&t.NetSales, -t.RefundNet)
	t.NetTax = t.SaleTax
	a.add(&t.NetTax, -t.RefundTax)
	t.ValueAfterRefunds = t.ChargedSales
	a.add(&t.ValueAfterRefunds, -t.RefundAmount)
	t.NetQuantity = t.SoldQuantity
	a.add(&t.NetQuantity, -t.ReturnedQuantity)
	if t.SaleCount > 0 {
		n := new(big.Int).Add(big.NewInt(t.ChargedSales), big.NewInt(t.SaleCount/2))
		n.Quo(n, big.NewInt(t.SaleCount))
		v := n.Int64()
		t.AverageTransaction = &v
	}
}

type lineValue struct {
	item     payment.TransactionItem
	net, tax int64
	saleID   string
	paidAt   time.Time
}

// originalLines reproduces the original discount/tax budgets used by refunds.
func originalLines(s payment.Transaction) (map[string]lineValue, error) {
	if len(s.Items) == 0 || s.Subtotal < 0 || s.Subtotal > maxAmount || s.DiscountAmount < 0 || s.DiscountAmount > s.Subtotal || s.TaxAmount < 0 || s.TaxAmount > maxAmount || s.TotalAmount < 0 || s.TotalAmount > maxAmount || s.Subtotal-s.DiscountAmount+s.TaxAmount != s.TotalAmount {
		return nil, ErrUnsupported
	}
	weights := make([]int64, len(s.Items))
	keys := make([]string, len(s.Items))
	var sum int64
	seen := map[string]bool{}
	for n, i := range s.Items {
		if i.ID == "" || seen[i.ID] || i.Quantity < 1 || i.Quantity > 1000 || i.UnitPrice < 0 || i.UnitPrice > maxAmount/int64(i.Quantity) || i.LineTotal != i.UnitPrice*int64(i.Quantity) || sum > maxAmount-i.LineTotal {
			return nil, ErrUnsupported
		}
		seen[i.ID] = true
		sum += i.LineTotal
		weights[n] = i.LineTotal
		keys[n] = i.ID
	}
	if sum != s.Subtotal {
		return nil, ErrUnsupported
	}
	if sum == 0 {
		for n, i := range s.Items {
			weights[n] = int64(i.Quantity)
		}
	}
	disc, e := refund.Allocate(s.DiscountAmount, weights, keys)
	if e != nil {
		return nil, e
	}
	nets := make([]int64, len(weights))
	var netSum int64
	for n, i := range s.Items {
		nets[n] = i.LineTotal - disc[n]
		netSum += nets[n]
	}
	tw := nets
	if netSum == 0 {
		tw = weights
	}
	tax, e := refund.Allocate(s.TaxAmount, tw, keys)
	if e != nil {
		return nil, e
	}
	r := map[string]lineValue{}
	for n, i := range s.Items {
		r[i.ID] = lineValue{i, nets[n], tax[n], s.ID, s.PaidAt}
	}
	return r, nil
}

// Calculate keeps original sale events and current-period return events independent.
func Calculate(k Kind, s Snapshot, f Filter, w Window) (Report, error) {
	r := Report{Kind: k, Daily: []DailyPoint{}, Methods: []MethodRow{}, Rows: []GroupRow{}}
	a := &accumulator{}
	loc, _ := time.LoadLocation("Asia/Jakarta")
	day := map[string]*DailyPoint{}
	for d := w.Start.In(loc); d.Before(w.EndExclusive); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		day[key] = &DailyPoint{Date: key}
	}
	methods := map[string]*MethodRow{}
	method := func(key string) *MethodRow {
		if methods[key] == nil {
			methods[key] = &MethodRow{Method: key}
		}
		return methods[key]
	}
	groups := map[string]*GroupRow{}
	names := map[string]string{}
	salesSeen := map[string]map[string]bool{}
	refundsSeen := map[string]map[string]bool{}
	group := func(l lineValue) *GroupRow {
		key := l.item.ProductID
		name := l.item.ProductName
		if l.item.ProductVariantID != nil {
			key += "/" + *l.item.ProductVariantID
			name += " • " + stringValue(l.item.VariantName)
		}
		if k == Categories {
			key = "unknown"
			name = "Kategori historis tidak tercatat"
			if l.item.CategoryIDSnapshot != nil && l.item.CategoryNameSnapshot != nil {
				key = *l.item.CategoryIDSnapshot
				name = *l.item.CategoryNameSnapshot
			}
		}
		g := groups[key]
		if g == nil {
			g = &GroupRow{Key: key, Name: name, ProductID: l.item.ProductID, VariantID: l.item.ProductVariantID, CategoryID: l.item.CategoryIDSnapshot, CategoryName: stringValue(l.item.CategoryNameSnapshot)}
			groups[key] = g
			salesSeen[key] = map[string]bool{}
			refundsSeen[key] = map[string]bool{}
		}
		if k == Products && g.CategoryName != stringValue(l.item.CategoryNameSnapshot) {
			g.CategoryName = "Beragam"
			g.CategoryID = nil
		}
		rank := l.paidAt.UTC().Format("2006-01-02T15:04:05.000000000Z") + l.item.ID
		if rank >= names[key] {
			g.Name = name
			names[key] = rank
		}
		price := l.item.UnitPrice
		if g.UnitPriceMin == nil || price < *g.UnitPriceMin {
			v := price
			g.UnitPriceMin = &v
		}
		if g.UnitPriceMax == nil || price > *g.UnitPriceMax {
			v := price
			g.UnitPriceMax = &v
		}
		return g
	}
	lines := map[string]lineValue{}
	valid := map[string]bool{}
	for _, receipt := range s.Receipts {
		t := receipt.Transaction
		legacy := t.Status == "cancelled" || (t.Status == "refunded" && t.RefundRevision == 0)
		ls, e := originalLines(t)
		if legacy || e != nil {
			if inWindow(t.PaidAt, w) {
				r.Totals.ExcludedReceipts++
			}
			continue
		}
		valid[t.ID] = true
		for id, l := range ls {
			lines[id] = l
		}
		if !inWindow(t.PaidAt, w) {
			continue
		}
		if !validMethod(t.PaymentMethod, false) {
			return r, ErrUnsupported
		}
		m := method(t.PaymentMethod)
		a.add(&m.SaleCount, 1)
		a.add(&m.SalesAmount, t.TotalAmount)
		d := day[t.PaidAt.In(loc).Format("2006-01-02")]
		a.add(&d.SaleCount, 1)
		a.add(&d.ChargedSales, t.TotalAmount)
		a.add(&d.NetSales, t.Subtotal-t.DiscountAmount)
		a.add(&r.Totals.GrossSales, t.Subtotal)
		a.add(&r.Totals.DiscountAmount, t.DiscountAmount)
		a.add(&r.Totals.SalesAfterDiscount, t.Subtotal-t.DiscountAmount)
		a.add(&r.Totals.SaleTax, t.TaxAmount)
		a.add(&r.Totals.ChargedSales, t.TotalAmount)
		a.add(&r.Totals.SaleCount, 1)
		for _, l := range ls {
			q := int64(l.item.Quantity)
			a.add(&r.Totals.SoldQuantity, q)
			if l.item.CategoryIDSnapshot == nil {
				a.add(&r.Totals.UnknownCategoryQuantity, q)
			}
			g := group(l)
			a.add(&g.GrossSales, l.item.LineTotal)
			a.add(&g.DiscountAmount, l.item.LineTotal-l.net)
			a.add(&g.SalesAfterDiscount, l.net)
			a.add(&g.SaleTax, l.tax)
			a.add(&g.ChargedSales, l.net+l.tax)
			a.add(&g.SoldQuantity, q)
			if l.item.CategoryIDSnapshot == nil {
				a.add(&g.UnknownCategoryQuantity, q)
			}
			salesSeen[g.Key][t.ID] = true
		}
	}
	for _, ret := range s.Refunds {
		if !inWindow(ret.CreatedAt, w) {
			continue
		}
		if !valid[ret.TransactionID] || !validMethod(ret.Method, true) || ret.NetAmount < 0 || ret.TaxAmount < 0 || ret.TotalAmount < 0 || ret.TotalAmount > maxAmount || ret.NetAmount+ret.TaxAmount != ret.TotalAmount {
			return r, ErrUnsupported
		}
		var net, tax int64
		for _, i := range ret.Items {
			l, ok := lines[i.TransactionItemID]
			if !ok || l.saleID != ret.TransactionID || i.Quantity < 1 || i.Quantity > l.item.Quantity || i.NetAmount < 0 || i.TaxAmount < 0 || i.TotalAmount != i.NetAmount+i.TaxAmount {
				return r, ErrUnsupported
			}
			a.add(&net, i.NetAmount)
			a.add(&tax, i.TaxAmount)
			q := int64(i.Quantity)
			a.add(&r.Totals.ReturnedQuantity, q)
			g := group(l)
			a.add(&g.ReturnedQuantity, q)
			a.add(&g.RefundNet, i.NetAmount)
			a.add(&g.RefundTax, i.TaxAmount)
			a.add(&g.RefundAmount, i.TotalAmount)
			refundsSeen[g.Key][ret.ID] = true
		}
		if net != ret.NetAmount || tax != ret.TaxAmount {
			return r, ErrUnsupported
		}
		m := method(ret.Method)
		a.add(&m.RefundCount, 1)
		a.add(&m.RefundAmount, ret.TotalAmount)
		d := day[ret.CreatedAt.In(loc).Format("2006-01-02")]
		a.add(&d.RefundAmount, ret.TotalAmount)
		a.add(&d.NetSales, -ret.NetAmount)
		a.add(&r.Totals.RefundCount, 1)
		a.add(&r.Totals.RefundNet, ret.NetAmount)
		a.add(&r.Totals.RefundTax, ret.TaxAmount)
		a.add(&r.Totals.RefundAmount, ret.TotalAmount)
	}
	finish(&r.Totals, a)
	for _, d := range day {
		r.Daily = append(r.Daily, *d)
	}
	sort.Slice(r.Daily, func(i, j int) bool { return r.Daily[i].Date < r.Daily[j].Date })
	for _, m := range methods {
		m.NetAmount = m.SalesAmount
		a.add(&m.NetAmount, -m.RefundAmount)
		r.Methods = append(r.Methods, *m)
	}
	sort.Slice(r.Methods, func(i, j int) bool { return r.Methods[i].Method < r.Methods[j].Method })
	for _, g := range groups {
		g.SaleCount = int64(len(salesSeen[g.Key]))
		g.RefundCount = int64(len(refundsSeen[g.Key]))
		finish(&g.Totals, a)
		if f.Query == "" || strings.Contains(strings.ToLower(g.Name), strings.ToLower(f.Query)) {
			r.Rows = append(r.Rows, *g)
		}
	}
	sort.Slice(r.Rows, func(i, j int) bool {
		x, y := r.Rows[i], r.Rows[j]
		switch f.Sort {
		case "name_asc":
			if x.Name != y.Name {
				return x.Name < y.Name
			}
		case "net_desc":
			if x.NetSales != y.NetSales {
				return x.NetSales > y.NetSales
			}
		default:
			if x.NetQuantity != y.NetQuantity {
				return x.NetQuantity > y.NetQuantity
			}
		}
		return x.Key < y.Key
	})
	if f.Query != "" && (k == Products || k == Categories) {
		excluded := r.Totals.ExcludedReceipts
		r.Totals = Totals{ExcludedReceipts: excluded}
		ss, rr := map[string]bool{}, map[string]bool{}
		for _, g := range r.Rows {
			merge(&r.Totals, g.Totals, a)
			for id := range salesSeen[g.Key] {
				ss[id] = true
			}
			for id := range refundsSeen[g.Key] {
				rr[id] = true
			}
		}
		r.Totals.SaleCount = int64(len(ss))
		r.Totals.RefundCount = int64(len(rr))
		finish(&r.Totals, a)
	}
	return r, a.err
}
func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
func validMethod(m string, ret bool) bool {
	switch m {
	case "cash", "wallet", "debit_card", "qris", "transfer":
		return true
	case "point":
		return !ret
	}
	return false
}

// merge accumulates raw components; derived values are calculated only once afterward.
func merge(t *Totals, v Totals, a *accumulator) {
	a.add(&t.GrossSales, v.GrossSales)
	a.add(&t.DiscountAmount, v.DiscountAmount)
	a.add(&t.SalesAfterDiscount, v.SalesAfterDiscount)
	a.add(&t.SaleTax, v.SaleTax)
	a.add(&t.ChargedSales, v.ChargedSales)
	a.add(&t.RefundNet, v.RefundNet)
	a.add(&t.RefundTax, v.RefundTax)
	a.add(&t.RefundAmount, v.RefundAmount)
	a.add(&t.SoldQuantity, v.SoldQuantity)
	a.add(&t.ReturnedQuantity, v.ReturnedQuantity)
	a.add(&t.UnknownCategoryQuantity, v.UnknownCategoryQuantity)
}
