package refund

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	"sort"
	"strings"
)

func canonical(id string) string {
	if u, e := uuid.Parse(id); e == nil {
		return u.String()
	}
	return id
}

// SelectionHash binds the confirmation to the exact canonical previewed intent.
func SelectionHash(id string, s Selection) (string, error) {
	s.Reason = strings.TrimSpace(s.Reason)
	s.Items = append([]SelectedItem(nil), s.Items...)
	for i := range s.Items {
		s.Items[i].TransactionItemID = canonical(s.Items[i].TransactionItemID)
	}
	sort.Slice(s.Items, func(i, j int) bool { return s.Items[i].TransactionItemID < s.Items[j].TransactionItemID })
	b, e := json.Marshal(struct {
		ID        string
		Selection Selection
	}{canonical(id), s})
	if e != nil {
		return "", e
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

// BuildPreview validates sold/remaining units and allocates only original receipt values.
func BuildPreview(r ReceiptSnapshot, s Selection) (Preview, error) {
	p := Preview{TransactionID: r.ID, Revision: r.RefundRevision, ResultStatus: "partially_refunded", Items: []RefundItem{}}
	if (s.Mode != "full" && s.Mode != "items") || !validMethod(s.Method) || s.Method == "" || len([]rune(strings.TrimSpace(s.Reason))) < 1 || len([]rune(s.Reason)) > 500 || len(s.Items) == 0 || len(s.Items) > 100 {
		return p, ErrValidation
	}
	if r.Status == "cancelled" || (r.Status == "refunded" && r.RefundRevision == 0) {
		return p, ErrLegacy
	}
	if s.ExpectedRevision != r.RefundRevision {
		return p, ErrRevision
	}
	if len(r.Items) == 0 || len(r.Items) > 100 || r.Subtotal < 0 || r.Subtotal > maxAmount || r.DiscountAmount < 0 || r.DiscountAmount > r.Subtotal || r.TaxAmount < 0 || r.TotalAmount < 0 || r.TotalAmount > maxAmount || r.Subtotal-r.DiscountAmount > maxAmount-r.TaxAmount || r.Subtotal-r.DiscountAmount+r.TaxAmount != r.TotalAmount {
		return p, ErrLegacy
	}
	weights := make([]int64, len(r.Items))
	keys := make([]string, len(r.Items))
	byID := map[string]int{}
	var sum int64
	remaining := 0
	for i, l := range r.Items {
		key := canonical(l.ID)
		if _, ok := byID[key]; ok {
			return p, ErrLegacy
		}
		if l.Quantity < 1 || l.Quantity > 1000 || l.UnitPrice < 0 || l.UnitPrice > maxAmount/int64(l.Quantity) || l.LineTotal != l.UnitPrice*int64(l.Quantity) || sum > maxAmount-l.LineTotal || l.RefundedQuantity < 0 || l.RefundedQuantity > l.Quantity {
			return p, ErrLegacy
		}
		sum += l.LineTotal
		weights[i] = l.LineTotal
		keys[i] = key
		byID[key] = i
		remaining += l.Quantity - l.RefundedQuantity
	}
	if sum != r.Subtotal {
		return p, ErrLegacy
	}
	if remaining == 0 {
		return p, ErrQuantity
	}
	// A zero gross receipt can still contain recorded tax; quantity weights define its allocation.
	gross := append([]int64(nil), weights...)
	if sum == 0 {
		for i, l := range r.Items {
			weights[i] = int64(l.Quantity)
		}
	}
	discount, e := Allocate(r.DiscountAmount, weights, keys)
	if e != nil {
		return p, e
	}
	net := make([]int64, len(weights))
	var netSum int64
	for i := range net {
		net[i] = gross[i] - discount[i]
		netSum += net[i]
	}
	taxWeights := net
	if netSum == 0 {
		taxWeights = weights
	}
	tax, e := Allocate(r.TaxAmount, taxWeights, keys)
	if e != nil {
		return p, e
	}
	seen := map[string]bool{}
	returns := map[string]int64{}
	selected := 0
	for _, v := range s.Items {
		key := canonical(v.TransactionItemID)
		i, ok := byID[key]
		if !ok || seen[key] || v.Quantity < 1 || v.Quantity > 1000 || v.RestockQuantity < 0 || v.RestockQuantity > v.Quantity {
			return p, ErrValidation
		}
		seen[key] = true
		l := r.Items[i]
		if v.Quantity > l.Quantity-l.RefundedQuantity {
			return p, ErrQuantity
		}
		selected += v.Quantity
		if v.RestockQuantity > 0 {
			returns[l.ProductID] += int64(v.RestockQuantity)
			if !l.TrackStock || returns[l.ProductID] > l.RestockBudget || l.StockQuantity > 1000000000-returns[l.ProductID] {
				return p, ErrRestock
			}
		}
		n := component(net[i], l.RefundedQuantity, v.Quantity, l.Quantity)
		t := component(tax[i], l.RefundedQuantity, v.Quantity, l.Quantity)
		p.Items = append(p.Items, RefundItem{TransactionItemID: l.ID, Quantity: v.Quantity, RestockQuantity: v.RestockQuantity, NetAmount: n, TaxAmount: t, TotalAmount: n + t})
		p.NetAmount += n
		p.TaxAmount += t
	}
	if s.Mode == "full" && selected != remaining {
		return p, ErrQuantity
	}
	if selected == remaining {
		p.ResultStatus = "refunded"
	}
	p.TotalAmount = p.NetAmount + p.TaxAmount
	p.SelectionHash, _ = SelectionHash(r.ID, s)
	return p, nil
}

// validMethod matches an entire supported manual return method.
func validMethod(m string) bool {
	switch m {
	case "cash", "wallet", "debit_card", "qris", "transfer":
		return true
	}
	return false
}
