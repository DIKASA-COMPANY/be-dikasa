package savedorder

import (
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

const maxAmount = int64(9007199254740991)

// canonicalID makes equivalent UUID spellings share one replay lock and cart key.
func canonicalID(id string) string {
	if parsed, e := uuid.Parse(id); e == nil {
		return parsed.String()
	}
	return id
}

func itemKey(i Item) string {
	v := ""
	if i.ProductVariantID != nil {
		v = canonicalID(*i.ProductVariantID)
	}
	return canonicalID(i.ProductID) + ":" + v
}

// ValidateMutation bounds snapshots before any database write or arithmetic.
func ValidateMutation(m Mutation) error {
	if _, e := uuid.Parse(m.RequestID); e != nil {
		return ErrValidation
	}
	if _, e := uuid.Parse(m.OrderTypeID); e != nil {
		return ErrValidation
	}
	if utf8.RuneCountInString(m.Label) > 100 || len(m.Items) == 0 || len(m.Items) > 100 {
		return ErrValidation
	}
	var sum int64
	keys := map[string]bool{}
	for _, i := range m.Items {
		if _, e := uuid.Parse(i.ProductID); e != nil {
			return ErrValidation
		}
		if i.ProductVariantID != nil {
			if _, e := uuid.Parse(*i.ProductVariantID); e != nil {
				return ErrValidation
			}
		}
		if strings.TrimSpace(i.ProductName) == "" || utf8.RuneCountInString(i.ProductName) > 150 || utf8.RuneCountInString(i.Notes) > 500 || i.Quantity < 1 || i.Quantity > 1000 || i.UnitPrice < 0 || i.UnitPrice > maxAmount/int64(i.Quantity) {
			return ErrValidation
		}
		if i.VariantName != nil && utf8.RuneCountInString(*i.VariantName) > 100 {
			return ErrValidation
		}
		line := i.UnitPrice * int64(i.Quantity)
		if sum > maxAmount-line {
			return ErrValidation
		}
		sum += line
		if keys[itemKey(i)] {
			return ErrValidation
		}
		keys[itemKey(i)] = true
	}
	return nil
}

// SameCart compares payment-relevant snapshots independently of line ordering.
func SameCart(orderTypeID string, items []Item, o *Order) bool {
	if canonicalID(orderTypeID) != canonicalID(o.OrderTypeID) || len(items) != len(o.Items) {
		return false
	}
	saved := map[string]Item{}
	for _, i := range o.Items {
		if _, ok := saved[itemKey(i)]; ok {
			return false
		}
		saved[itemKey(i)] = i
	}
	for _, i := range items {
		s, ok := saved[itemKey(i)]
		if !ok || s.UnitPrice != i.UnitPrice || s.Quantity != i.Quantity || strings.TrimSpace(s.Notes) != strings.TrimSpace(i.Notes) {
			return false
		}
		delete(saved, itemKey(i))
	}
	return len(saved) == 0
}
