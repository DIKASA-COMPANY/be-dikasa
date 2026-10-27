package savedorder

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func validMutation() Mutation {
	return Mutation{RequestID: uuid.NewString(), OrderTypeID: uuid.NewString(), Items: []Item{{ProductID: uuid.NewString(), ProductName: "Tea", UnitPrice: 100, Quantity: 2}}}
}

func TestValidateMutation(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Mutation)
		valid  bool
	}{
		{"normal", func(m *Mutation) {}, true},
		{"100 lines", func(m *Mutation) {
			for i := 1; i < 100; i++ {
				m.Items = append(m.Items, Item{ProductID: uuid.NewString(), ProductName: "Tea", Quantity: 1})
			}
		}, true},
		{"101 lines", func(m *Mutation) {
			for i := 1; i < 101; i++ {
				m.Items = append(m.Items, Item{ProductID: uuid.NewString(), ProductName: "Tea", Quantity: 1})
			}
		}, false},
		{"zero", func(m *Mutation) { m.Items[0].Quantity = 0 }, false},
		{"large quantity", func(m *Mutation) { m.Items[0].Quantity = 1001 }, false},
		{"note", func(m *Mutation) { m.Items[0].Notes = strings.Repeat("字", 501) }, false},
		{"label limit", func(m *Mutation) { m.Label = strings.Repeat("字", 100); m.Items[0].Notes = strings.Repeat("字", 500) }, true},
		{"label", func(m *Mutation) { m.Label = strings.Repeat("字", 101) }, false},
		{"uuid", func(m *Mutation) { m.RequestID = "bad" }, false},
		{"negative", func(m *Mutation) { m.Items[0].UnitPrice = -1 }, false},
		{"overflow", func(m *Mutation) { m.Items[0].UnitPrice = 9007199254740991 }, false},
		{"duplicate", func(m *Mutation) { m.Items = append(m.Items, m.Items[0]) }, false},
		{"base and variant", func(m *Mutation) {
			x := m.Items[0]
			id := uuid.NewString()
			x.ProductVariantID = &id
			m.Items = append(m.Items, x)
		}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := validMutation()
			c.change(&m)
			err := ValidateMutation(m)
			if (err == nil) != c.valid {
				t.Fatalf("valid=%v err=%v", c.valid, err)
			}
		})
	}
}

func TestSameCart(t *testing.T) {
	m := validMutation()
	m.Items[0].Notes = "less ice"
	x := m.Items[0]
	v := uuid.NewString()
	x.ProductVariantID = &v
	m.Items = append(m.Items, x)
	o := &Order{OrderTypeID: m.OrderTypeID, Items: m.Items}
	if !SameCart(m.OrderTypeID, []Item{x, m.Items[0]}, o) {
		t.Fatal("reordered cart must match")
	}
	for _, field := range []string{"price", "quantity", "note", "variant", "product", "type", "duplicate"} {
		t.Run(field, func(t *testing.T) {
			items := append([]Item(nil), m.Items...)
			typ := m.OrderTypeID
			switch field {
			case "price":
				items[0].UnitPrice++
			case "quantity":
				items[0].Quantity++
			case "note":
				items[0].Notes = "other"
			case "variant":
				items[0].ProductVariantID = &v
			case "product":
				items[0].ProductID = uuid.NewString()
			case "type":
				typ = uuid.NewString()
			case "duplicate":
				items[0] = items[1]
			}
			if SameCart(typ, items, o) {
				t.Fatal("changed cart matched")
			}
		})
	}
}

func TestEquivalentUUIDsCannotDuplicateCartLines(t *testing.T) {
	m := validMutation()
	copy := m.Items[0]
	copy.ProductID = strings.ToUpper(copy.ProductID)
	m.Items = append(m.Items, copy)
	if ValidateMutation(m) == nil {
		t.Fatal("equivalent UUID duplicate accepted")
	}
}
