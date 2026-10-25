package payment

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"testing"
)

func TestSavedOrderAssociationRequiresPair(t *testing.T) {
	id := uuid.NewString()
	rev := int64(1)
	m := TransactionMutation{OrderTypeID: "type", PaymentMethod: "cash", Subtotal: 100, TotalAmount: 100, PaidAmount: 100, Items: []TransactionItemMutation{{ProductID: "product", ProductName: "Tea", UnitPrice: 100, Quantity: 1, LineTotal: 100}}}
	m.SavedOrderID = &id
	if _, _, e := validateTransactionMutation("outlet", m); !errors.Is(e, ErrValidation) {
		t.Fatal("missing revision accepted", e)
	}
	m.SavedOrderRevision = &rev
	if _, _, e := validateTransactionMutation("outlet", m); e != nil {
		t.Fatal(e)
	}
	m.SavedOrderID = nil
	if _, _, e := validateTransactionMutation("outlet", m); !errors.Is(e, ErrValidation) {
		t.Fatal("missing ID accepted")
	}
}

func TestSavedPaymentRequiresCashierOrManager(t *testing.T) {
	id := uuid.NewString()
	rev := int64(1)
	m := TransactionMutation{SavedOrderID: &id, SavedOrderRevision: &rev, OrderTypeID: "type", PaymentMethod: "cash", Subtotal: 100, TotalAmount: 100, PaidAmount: 100, Items: []TransactionItemMutation{{ProductID: "product", ProductName: "Tea", UnitPrice: 100, Quantity: 1, LineTotal: 100}}}
	repo := &workflowRepository{cashSessionID: "cash"}
	s := NewService(repo, workflowAccountFinder{account: paymentAccount("admin")})
	if _, e := s.CreateTransaction(context.Background(), "actor", m); !errors.Is(e, ErrForbidden) {
		t.Fatal("unexpected role accepted", e)
	}
}
