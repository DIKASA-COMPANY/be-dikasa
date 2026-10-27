package payment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"be-dikasa/internal/user"
)

const (
	maxTransactionItems    = 100
	maxTransactionQuantity = 1000
	maxPaymentAmount       = int64(9_007_199_254_740_991)
)

// AccountFinder mengambil scope outlet dan role user terautentikasi.
type AccountFinder interface {
	FindByIDWithRole(ctx context.Context, id string) (*user.User, error)
}

// TransactionNumberGenerator menghasilkan nomor transaksi unik.
type TransactionNumberGenerator func() string

// Clock menghasilkan waktu pembayaran dan dapat diganti pada test.
type Clock func() time.Time

// ServiceOption mengubah dependency opsional service pembayaran.
type ServiceOption func(*Service)

// Service menyediakan use case wallet dan penyimpanan pembayaran.
type Service struct {
	repository      Repository
	accounts        AccountFinder
	numberGenerator TransactionNumberGenerator
	now             Clock
}

// NewService membuat payment service dengan repository dan sumber akun yang diberikan.
func NewService(repository Repository, accounts AccountFinder, options ...ServiceOption) *Service {
	service := &Service{repository: repository, accounts: accounts, numberGenerator: defaultTransactionNumber, now: time.Now}
	for _, option := range options {
		option(service)
	}
	return service
}

// WithTransactionNumberGenerator mengganti generator nomor transaksi untuk test atau integrasi.
func WithTransactionNumberGenerator(generator TransactionNumberGenerator) ServiceOption {
	return func(service *Service) { service.numberGenerator = generator }
}

// WithClock mengganti sumber waktu pembayaran.
func WithClock(clock Clock) ServiceOption {
	return func(service *Service) { service.now = clock }
}

func defaultTransactionNumber() string {
	random := strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", ""))
	return fmt.Sprintf("TRX-%s-%s", time.Now().UTC().Format("20060102"), random[:12])
}

func (service *Service) scope(ctx context.Context, userID string, managerOnly bool) (string, error) {
	account, err := service.accounts.FindByIDWithRole(ctx, userID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return "", ErrUserNotFound
		}
		return "", err
	}
	if account.OutletID == nil || strings.TrimSpace(*account.OutletID) == "" {
		return "", ErrUserWithoutOutlet
	}
	if managerOnly && !strings.EqualFold(account.Role.Name, "manager") {
		return "", ErrForbidden
	}
	return *account.OutletID, nil
}

// TaxPercentage mengambil persentase pajak terbaru milik outlet user.
func (service *Service) TaxPercentage(ctx context.Context, userID string) (float64, error) {
	outletID, err := service.scope(ctx, userID, false)
	if err != nil {
		return 0, err
	}
	return service.repository.FindTaxPercentageByOutletID(ctx, outletID)
}

// ListWallets mengambil pilihan wallet outlet untuk dropdown pembayaran.
func (service *Service) ListWallets(ctx context.Context, userID string, includeInactive bool) ([]Wallet, error) {
	outletID, err := service.scope(ctx, userID, includeInactive)
	if err != nil {
		return nil, err
	}
	return service.repository.ListWallets(ctx, outletID, includeInactive)
}

// CreateWallet membuat wallet outlet baru dan aktif.
func (service *Service) CreateWallet(ctx context.Context, userID string, mutation WalletMutation) (*Wallet, error) {
	outletID, err := service.scope(ctx, userID, true)
	if err != nil {
		return nil, err
	}
	code, name := strings.TrimSpace(mutation.Code), strings.TrimSpace(mutation.Name)
	if code == "" || name == "" || len(code) > 50 || len(name) > 100 {
		return nil, ErrValidation
	}
	wallet := &Wallet{ID: uuid.NewString(), OutletID: outletID, Code: code, Name: name, IconKey: cleanOptionalString(mutation.IconKey), SortOrder: mutation.SortOrder, IsActive: true}
	if err = service.repository.CreateWallet(ctx, wallet); err != nil {
		return nil, err
	}
	return wallet, nil
}

// UpdateWallet memperbarui wallet outlet milik manager.
func (service *Service) UpdateWallet(ctx context.Context, userID, walletID string, mutation WalletMutation) (*Wallet, error) {
	outletID, err := service.scope(ctx, userID, true)
	if err != nil {
		return nil, err
	}
	wallet, err := service.repository.FindWallet(ctx, outletID, walletID, true)
	if err != nil {
		return nil, err
	}
	code, name := strings.TrimSpace(mutation.Code), strings.TrimSpace(mutation.Name)
	if code == "" || name == "" || len(code) > 50 || len(name) > 100 {
		return nil, ErrValidation
	}
	wallet.Code, wallet.Name, wallet.IconKey = code, name, cleanOptionalString(mutation.IconKey)
	wallet.SortOrder, wallet.IsActive = mutation.SortOrder, mutation.IsActive
	if err = service.repository.UpdateWallet(ctx, wallet); err != nil {
		return nil, err
	}
	return wallet, nil
}

// DeleteWallet melakukan soft delete wallet outlet.
func (service *Service) DeleteWallet(ctx context.Context, userID, walletID string) error {
	outletID, err := service.scope(ctx, userID, true)
	if err != nil {
		return err
	}
	return service.repository.SoftDeleteWallet(ctx, outletID, walletID)
}

// CreateTransaction memvalidasi snapshot FE lalu menyimpan pembayaran dan item secara atomik.
func (service *Service) CreateTransaction(ctx context.Context, userID string, mutation TransactionMutation) (*Transaction, error) {
	// Draft APIs and linked checkout share the same cashier/manager role boundary.
	if mutation.SavedOrderID != nil {
		account, e := service.accounts.FindByIDWithRole(ctx, userID)
		if e != nil {
			return nil, ErrForbidden
		}
		role := strings.ToLower(account.Role.Name)
		if role != "cashier" && role != "manager" {
			return nil, ErrForbidden
		}
	}
	outletID, err := service.scope(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	items, totalQuantity, err := validateTransactionMutation(outletID, mutation)
	if err != nil {
		return nil, err
	}
	var wallet *Wallet
	if mutation.PaymentMethod == PaymentMethodWallet {
		wallet, err = service.repository.FindWallet(ctx, outletID, *mutation.WalletID, false)
		if err != nil {
			return nil, err
		}
	}
	cashSessionID, err := service.repository.FindOpenCashSessionIDByOutletID(ctx, outletID)
	if err != nil {
		return nil, err
	}
	value := &Transaction{
		SavedOrderID: mutation.SavedOrderID, SavedOrderRevision: mutation.SavedOrderRevision,
		ID: uuid.NewString(), TransactionNumber: service.numberGenerator(), OutletID: outletID,
		CashSessionID: cashSessionID, CreatedBy: userID, OrderTypeID: mutation.OrderTypeID,
		RestaurantTableID: cleanOptionalString(mutation.RestaurantTableID), PaymentMethod: mutation.PaymentMethod,
		WalletID: cleanOptionalString(mutation.WalletID), Wallet: wallet, Subtotal: mutation.Subtotal,
		DiscountAmount: mutation.DiscountAmount, TaxPercentage: mutation.TaxPercentage, TaxAmount: mutation.TaxAmount,
		TotalAmount: mutation.TotalAmount, PaidAmount: mutation.PaidAmount, ChangeAmount: mutation.ChangeAmount,
		RoundingAmount: mutation.RoundingAmount, TipAmount: mutation.TipAmount, TotalQuantity: totalQuantity,
		Status: TransactionStatusPaid, PaidAt: service.now().UTC(), Items: items,
	}
	if err = service.repository.CreateTransaction(ctx, value); err != nil {
		return nil, err
	}
	return value, nil
}

func validateTransactionMutation(outletID string, mutation TransactionMutation) ([]TransactionItem, int, error) {
	if (mutation.SavedOrderID == nil) != (mutation.SavedOrderRevision == nil) {
		return nil, 0, ErrValidation
	}
	if mutation.SavedOrderID != nil {
		if _, err := uuid.Parse(*mutation.SavedOrderID); err != nil || *mutation.SavedOrderRevision < 1 {
			return nil, 0, ErrValidation
		}
	}
	if strings.TrimSpace(mutation.OrderTypeID) == "" || !validPaymentMethod(mutation.PaymentMethod) || len(mutation.Items) == 0 || len(mutation.Items) > maxTransactionItems {
		return nil, 0, ErrValidation
	}
	if mutation.PaymentMethod == PaymentMethodWallet {
		if mutation.WalletID == nil || strings.TrimSpace(*mutation.WalletID) == "" {
			return nil, 0, ErrValidation
		}
	} else if mutation.WalletID != nil {
		return nil, 0, ErrValidation
	}
	amounts := []int64{mutation.Subtotal, mutation.DiscountAmount, mutation.TaxAmount, mutation.TotalAmount, mutation.PaidAmount, mutation.ChangeAmount, mutation.RoundingAmount, mutation.TipAmount}
	for _, amount := range amounts {
		if amount < 0 || amount > maxPaymentAmount {
			return nil, 0, ErrValidation
		}
	}
	if mutation.TaxPercentage < 0 || mutation.TaxPercentage > 100 || mutation.DiscountAmount > mutation.Subtotal || mutation.PaidAmount < mutation.TotalAmount {
		return nil, 0, ErrValidation
	}
	if mutation.Subtotal-mutation.DiscountAmount+mutation.TaxAmount != mutation.TotalAmount {
		return nil, 0, ErrValidation
	}
	items := make([]TransactionItem, 0, len(mutation.Items))
	var subtotal int64
	totalQuantity := 0
	for _, item := range mutation.Items {
		name := strings.TrimSpace(item.ProductName)
		if strings.TrimSpace(item.ProductID) == "" || name == "" || len(name) > 150 || item.UnitPrice < 0 || item.UnitPrice > maxPaymentAmount || item.Quantity < 1 || item.Quantity > maxTransactionQuantity {
			return nil, 0, ErrValidation
		}
		if item.UnitPrice > maxPaymentAmount/int64(item.Quantity) || item.LineTotal != item.UnitPrice*int64(item.Quantity) {
			return nil, 0, ErrValidation
		}
		if subtotal > maxPaymentAmount-item.LineTotal {
			return nil, 0, ErrValidation
		}
		subtotal += item.LineTotal
		totalQuantity += item.Quantity
		items = append(items, TransactionItem{
			ID: uuid.NewString(), OutletID: outletID, ProductID: item.ProductID,
			ProductVariantID: cleanOptionalString(item.ProductVariantID), ProductName: name,
			VariantName: cleanOptionalString(item.VariantName), UnitPrice: item.UnitPrice,
			Quantity: item.Quantity, Notes: strings.TrimSpace(item.Notes), LineTotal: item.LineTotal,
		})
	}
	if subtotal != mutation.Subtotal {
		return nil, 0, ErrValidation
	}
	return items, totalQuantity, nil
}

func validPaymentMethod(value string) bool {
	switch value {
	case PaymentMethodCash, PaymentMethodWallet, PaymentMethodDebitCard, PaymentMethodQRIS, PaymentMethodPoint, PaymentMethodTransfer:
		return true
	default:
		return false
	}
}

func cleanOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	cleaned := strings.TrimSpace(*value)
	if cleaned == "" {
		return nil
	}
	return &cleaned
}
