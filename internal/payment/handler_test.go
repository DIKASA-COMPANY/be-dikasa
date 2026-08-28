package payment

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	sharederrors "be-dikasa/internal/shared/errors"
	"be-dikasa/internal/user"
)

type stubProvider struct {
	taxPercentage float64
	wallets       []Wallet
	transaction   *Transaction
	err           error
}

func (provider stubProvider) TaxPercentage(context.Context, string) (float64, error) {
	return provider.taxPercentage, provider.err
}
func (provider stubProvider) ListWallets(context.Context, string, bool) ([]Wallet, error) {
	return provider.wallets, provider.err
}
func (provider stubProvider) CreateWallet(context.Context, string, WalletMutation) (*Wallet, error) {
	if len(provider.wallets) > 0 {
		return &provider.wallets[0], provider.err
	}
	return nil, provider.err
}
func (provider stubProvider) UpdateWallet(context.Context, string, string, WalletMutation) (*Wallet, error) {
	if len(provider.wallets) > 0 {
		return &provider.wallets[0], provider.err
	}
	return nil, provider.err
}
func (provider stubProvider) DeleteWallet(context.Context, string, string) error { return provider.err }
func (provider stubProvider) CreateTransaction(context.Context, string, TransactionMutation) (*Transaction, error) {
	return provider.transaction, provider.err
}

func TestHandlerReturnsOnlyTaxPercentage(t *testing.T) {
	handler := NewHandler(stubProvider{taxPercentage: 10})
	ctx := user.WithAuthenticatedUserID(context.Background(), "user-id")

	output, err := handler.TaxPercentage(ctx, &struct{}{})

	if err != nil {
		t.Fatalf("get tax percentage: %v", err)
	}
	body, marshalErr := json.Marshal(output.Body)
	if marshalErr != nil {
		t.Fatalf("marshal response: %v", marshalErr)
	}
	if string(body) != `{"tax_percentage":10}` {
		t.Fatalf("unexpected response body: %s", body)
	}
}

func TestHandlerRequiresAuthentication(t *testing.T) {
	handler := NewHandler(stubProvider{taxPercentage: 10})

	_, err := handler.TaxPercentage(context.Background(), &struct{}{})

	statusError, ok := err.(*sharederrors.StatusError)
	if !ok || statusError.GetStatus() != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized StatusError, got %T %v", err, err)
	}
}

func TestHandlerMapsTaxPercentageErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "user without outlet", err: ErrUserWithoutOutlet, status: http.StatusForbidden},
		{name: "user missing", err: ErrUserNotFound, status: http.StatusNotFound},
		{name: "outlet missing", err: ErrOutletNotFound, status: http.StatusNotFound},
		{name: "database failure", err: errors.New("database unavailable"), status: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewHandler(stubProvider{err: test.err})
			ctx := user.WithAuthenticatedUserID(context.Background(), "user-id")

			_, err := handler.TaxPercentage(ctx, &struct{}{})

			statusError, ok := err.(*sharederrors.StatusError)
			if !ok || statusError.GetStatus() != test.status {
				t.Fatalf("expected status %d, got %T %v", test.status, err, err)
			}
		})
	}
}

func TestHandlerListsWalletDropdownData(t *testing.T) {
	iconKey := "gopay"
	handler := NewHandler(stubProvider{wallets: []Wallet{{ID: "wallet-id", Code: "gopay", Name: "GoPay", IconKey: &iconKey, SortOrder: 10, IsActive: true}}})
	ctx := user.WithAuthenticatedUserID(context.Background(), "user-id")

	output, err := handler.ListWallets(ctx, &WalletListInput{})

	if err != nil {
		t.Fatalf("list wallets: %v", err)
	}
	if len(output.Body.Data) != 1 || output.Body.Data[0].Code != "gopay" || output.Body.Data[0].IconKey == nil {
		t.Fatalf("unexpected wallet response: %+v", output.Body.Data)
	}
}

func TestHandlerReturnsSavedTransactionForSuccessPage(t *testing.T) {
	paid := &Transaction{ID: "transaction-id", TransactionNumber: "TRX-001", PaymentMethod: PaymentMethodCash, TotalAmount: 21_000, PaidAmount: 25_000, ChangeAmount: 4_000, TotalQuantity: 1, Status: TransactionStatusPaid,
		Items: []TransactionItem{{ProductID: "product-id", ProductName: "Nasi Goreng", UnitPrice: 21_000, Quantity: 1, LineTotal: 21_000}}}
	handler := NewHandler(stubProvider{transaction: paid})
	ctx := user.WithAuthenticatedUserID(context.Background(), "user-id")
	input := &CreateTransactionInput{Body: TransactionRequest{
		OrderTypeID: "11111111-1111-4111-8111-111111111111", PaymentMethod: PaymentMethodCash,
		Subtotal: 21_000, TotalAmount: 21_000, PaidAmount: 25_000, ChangeAmount: 4_000,
		Items: []TransactionItemRequest{{ProductID: "22222222-2222-4222-8222-222222222222", ProductName: "Nasi Goreng", UnitPrice: 21_000, Quantity: 1, LineTotal: 21_000}},
	}}

	output, err := handler.CreateTransaction(ctx, input)

	if err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	if output.Status != http.StatusCreated || output.Body.Data.TransactionNumber != "TRX-001" || len(output.Body.Data.Items) != 1 {
		t.Fatalf("unexpected transaction response: %+v", output)
	}
}
