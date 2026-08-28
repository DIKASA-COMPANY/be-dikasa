package payment

import (
	"context"
	"errors"
	"testing"

	"be-dikasa/internal/user"
)

type workflowRepository struct {
	wallets       []Wallet
	wallet        *Wallet
	transaction   *Transaction
	cashSessionID string
	err           error
}

func (repository *workflowRepository) FindTaxPercentageByOutletID(context.Context, string) (float64, error) {
	return 10, repository.err
}
func (repository *workflowRepository) ListWallets(context.Context, string, bool) ([]Wallet, error) {
	return repository.wallets, repository.err
}
func (repository *workflowRepository) FindWallet(context.Context, string, string, bool) (*Wallet, error) {
	return repository.wallet, repository.err
}
func (repository *workflowRepository) CreateWallet(_ context.Context, wallet *Wallet) error {
	repository.wallet = wallet
	return repository.err
}
func (repository *workflowRepository) UpdateWallet(_ context.Context, wallet *Wallet) error {
	repository.wallet = wallet
	return repository.err
}
func (repository *workflowRepository) SoftDeleteWallet(context.Context, string, string) error {
	return repository.err
}
func (repository *workflowRepository) FindOpenCashSessionIDByOutletID(context.Context, string) (string, error) {
	return repository.cashSessionID, repository.err
}
func (repository *workflowRepository) CreateTransaction(_ context.Context, transaction *Transaction) error {
	repository.transaction = transaction
	return repository.err
}

type workflowAccountFinder struct {
	account *user.User
	err     error
}

func (finder workflowAccountFinder) FindByIDWithRole(context.Context, string) (*user.User, error) {
	return finder.account, finder.err
}

func paymentAccount(role string) *user.User {
	outletID := "outlet-id"
	return &user.User{ID: "user-id", OutletID: &outletID, Role: user.Role{Name: role}}
}

func TestServiceManagesOutletWalletsForManager(t *testing.T) {
	repository := &workflowRepository{}
	service := NewService(repository, workflowAccountFinder{account: paymentAccount("manager")})

	created, err := service.CreateWallet(context.Background(), "user-id", WalletMutation{
		Code: " gopay ", Name: " GoPay ", IconKey: stringPointer("gopay"), SortOrder: 10,
	})
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}
	if created.OutletID != "outlet-id" || created.Code != "gopay" || created.Name != "GoPay" || !created.IsActive {
		t.Fatalf("unexpected wallet: %+v", created)
	}

	repository.wallet = created
	updated, err := service.UpdateWallet(context.Background(), "user-id", created.ID, WalletMutation{
		Code: "gopay", Name: "GoPay Indonesia", IconKey: stringPointer("gopay"), SortOrder: 20, IsActive: true,
	})
	if err != nil || updated.Name != "GoPay Indonesia" || updated.SortOrder != 20 {
		t.Fatalf("update wallet: %+v err=%v", updated, err)
	}
	if err = service.DeleteWallet(context.Background(), "user-id", created.ID); err != nil {
		t.Fatalf("delete wallet: %v", err)
	}
}

func TestServiceRejectsWalletMutationByCashier(t *testing.T) {
	service := NewService(&workflowRepository{}, workflowAccountFinder{account: paymentAccount("cashier")})

	_, err := service.CreateWallet(context.Background(), "user-id", WalletMutation{Code: "ovo", Name: "OVO"})

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestServiceCreatesPaidTransactionWithItemSnapshots(t *testing.T) {
	repository := &workflowRepository{cashSessionID: "cash-session-id"}
	service := NewService(repository, workflowAccountFinder{account: paymentAccount("cashier")}, WithTransactionNumberGenerator(func() string { return "TRX-TEST-001" }))

	transaction, err := service.CreateTransaction(context.Background(), "user-id", TransactionMutation{
		OrderTypeID: "order-type-id", PaymentMethod: PaymentMethodCash,
		Subtotal: 42_000, TaxPercentage: 10, TaxAmount: 4_200, TotalAmount: 46_200,
		PaidAmount: 50_000, ChangeAmount: 3_800,
		Items: []TransactionItemMutation{{
			ProductID: "product-id", ProductName: "Nasi Goreng", UnitPrice: 21_000,
			Quantity: 2, Notes: "Tidak pedas", LineTotal: 42_000,
		}},
	})

	if err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	if repository.transaction != transaction {
		t.Fatal("service must persist and return the same transaction")
	}
	if transaction.TransactionNumber != "TRX-TEST-001" || transaction.CashSessionID != "cash-session-id" || transaction.TotalQuantity != 2 || transaction.Status != TransactionStatusPaid {
		t.Fatalf("unexpected transaction: %+v", transaction)
	}
	if len(transaction.Items) != 1 || transaction.Items[0].ProductName != "Nasi Goreng" || transaction.Items[0].OutletID != "outlet-id" {
		t.Fatalf("unexpected transaction items: %+v", transaction.Items)
	}
}

func TestServiceRequiresWalletForWalletPayment(t *testing.T) {
	service := NewService(&workflowRepository{cashSessionID: "cash-session-id"}, workflowAccountFinder{account: paymentAccount("cashier")})

	_, err := service.CreateTransaction(context.Background(), "user-id", TransactionMutation{
		OrderTypeID: "order-type-id", PaymentMethod: PaymentMethodWallet,
		Subtotal: 10_000, TotalAmount: 10_000, PaidAmount: 10_000,
		Items: []TransactionItemMutation{{ProductID: "product-id", ProductName: "Produk", UnitPrice: 10_000, Quantity: 1, LineTotal: 10_000}},
	})

	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestServiceRejectsInconsistentFrontendAmounts(t *testing.T) {
	service := NewService(&workflowRepository{cashSessionID: "cash-session-id"}, workflowAccountFinder{account: paymentAccount("cashier")})

	_, err := service.CreateTransaction(context.Background(), "user-id", TransactionMutation{
		OrderTypeID: "order-type-id", PaymentMethod: PaymentMethodCash,
		Subtotal: 20_000, TotalAmount: 20_000, PaidAmount: 20_000,
		Items: []TransactionItemMutation{{ProductID: "product-id", ProductName: "Produk", UnitPrice: 10_000, Quantity: 2, LineTotal: 10_000}},
	})

	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func stringPointer(value string) *string { return &value }
