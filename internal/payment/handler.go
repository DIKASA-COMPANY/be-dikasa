package payment

import (
	"be-dikasa/internal/catalog"
	"be-dikasa/internal/savedorder"
	"context"
	"errors"
	"net/http"

	sharederrors "be-dikasa/internal/shared/errors"
	sharedvalidator "be-dikasa/internal/shared/validator"
	"be-dikasa/internal/user"
)

// Provider mendefinisikan use case persentase pajak untuk handler HTTP.
type Provider interface {
	TaxPercentage(ctx context.Context, userID string) (float64, error)
	ListWallets(ctx context.Context, userID string, includeInactive bool) ([]Wallet, error)
	CreateWallet(ctx context.Context, userID string, mutation WalletMutation) (*Wallet, error)
	UpdateWallet(ctx context.Context, userID, walletID string, mutation WalletMutation) (*Wallet, error)
	DeleteWallet(ctx context.Context, userID, walletID string) error
	CreateTransaction(ctx context.Context, userID string, mutation TransactionMutation) (*Transaction, error)
}

// Handler melayani endpoint konfigurasi pembayaran melalui Huma.
type Handler struct {
	provider  Provider
	validator *sharedvalidator.Validator
}

// NewHandler membuat handler pembayaran dengan provider yang diberikan.
func NewHandler(provider Provider, validators ...*sharedvalidator.Validator) *Handler {
	validator := sharedvalidator.New()
	if len(validators) > 0 && validators[0] != nil {
		validator = validators[0]
	}
	return &Handler{provider: provider, validator: validator}
}

// TaxPercentage mengembalikan hanya persentase pajak outlet user terautentikasi.
func (handler *Handler) TaxPercentage(ctx context.Context, _ *struct{}) (*TaxPercentageOutput, error) {
	userID, ok := user.AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return nil, sharederrors.Standard(http.StatusUnauthorized, nil)
	}
	taxPercentage, err := handler.provider.TaxPercentage(ctx, userID)
	if err != nil {
		return nil, paymentStatusError(err)
	}
	return &TaxPercentageOutput{Body: TaxPercentageResponse{TaxPercentage: taxPercentage}}, nil
}

// ListWallets mengembalikan pilihan dropdown wallet milik outlet user.
func (handler *Handler) ListWallets(ctx context.Context, input *WalletListInput) (*WalletListOutput, error) {
	userID, ok := user.AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return nil, sharederrors.Standard(http.StatusUnauthorized, nil)
	}
	wallets, err := handler.provider.ListWallets(ctx, userID, input.IncludeInactive)
	if err != nil {
		return nil, paymentStatusError(err)
	}
	data := make([]WalletData, 0, len(wallets))
	for index := range wallets {
		data = append(data, walletData(&wallets[index]))
	}
	return &WalletListOutput{Body: WalletListResponse{Data: data}}, nil
}

// CreateWallet memvalidasi dan membuat wallet outlet sebagai manager.
func (handler *Handler) CreateWallet(ctx context.Context, input *CreateWalletInput) (*WalletOutput, error) {
	userID, ok := user.AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return nil, sharederrors.Standard(http.StatusUnauthorized, nil)
	}
	if details := handler.validator.Validate(input.Body); len(details) > 0 {
		return nil, sharederrors.New(http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Data yang dikirim tidak memenuhi ketentuan.", details)
	}
	wallet, err := handler.provider.CreateWallet(ctx, userID, walletMutation(input.Body))
	if err != nil {
		return nil, paymentStatusError(err)
	}
	return &WalletOutput{Status: http.StatusCreated, Body: WalletResponse{Data: walletData(wallet)}}, nil
}

// UpdateWallet memvalidasi dan memperbarui wallet outlet sebagai manager.
func (handler *Handler) UpdateWallet(ctx context.Context, input *UpdateWalletInput) (*WalletOutput, error) {
	userID, ok := user.AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return nil, sharederrors.Standard(http.StatusUnauthorized, nil)
	}
	if details := handler.validator.Validate(input.Body); len(details) > 0 {
		return nil, sharederrors.New(http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Data yang dikirim tidak memenuhi ketentuan.", details)
	}
	wallet, err := handler.provider.UpdateWallet(ctx, userID, input.WalletID, walletMutation(input.Body))
	if err != nil {
		return nil, paymentStatusError(err)
	}
	return &WalletOutput{Body: WalletResponse{Data: walletData(wallet)}}, nil
}

// DeleteWallet melakukan soft delete wallet outlet sebagai manager.
func (handler *Handler) DeleteWallet(ctx context.Context, input *DeleteWalletInput) (*NoContentOutput, error) {
	userID, ok := user.AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return nil, sharederrors.Standard(http.StatusUnauthorized, nil)
	}
	if err := handler.provider.DeleteWallet(ctx, userID, input.WalletID); err != nil {
		return nil, paymentStatusError(err)
	}
	return &NoContentOutput{Status: http.StatusNoContent}, nil
}

// CreateTransaction memvalidasi snapshot FE dan menyimpan transaksi berhasil.
func (handler *Handler) CreateTransaction(ctx context.Context, input *CreateTransactionInput) (*TransactionOutput, error) {
	userID, ok := user.AuthenticatedUserIDFromContext(ctx)
	if !ok {
		return nil, sharederrors.Standard(http.StatusUnauthorized, nil)
	}
	if details := handler.validator.Validate(input.Body); len(details) > 0 {
		return nil, sharederrors.New(http.StatusUnprocessableEntity, "VALIDASI_GAGAL", "Data yang dikirim tidak memenuhi ketentuan.", details)
	}
	transaction, err := handler.provider.CreateTransaction(ctx, userID, transactionMutation(input.Body))
	if err != nil {
		return nil, paymentStatusError(err)
	}
	return &TransactionOutput{Status: http.StatusCreated, Body: TransactionResponse{Data: transactionData(transaction)}}, nil
}

func walletMutation(request WalletRequest) WalletMutation {
	return WalletMutation{Code: request.Code, Name: request.Name, IconKey: request.IconKey, SortOrder: request.SortOrder, IsActive: request.IsActive}
}

func walletData(wallet *Wallet) WalletData {
	return WalletData{ID: wallet.ID, Code: wallet.Code, Name: wallet.Name, IconKey: wallet.IconKey, SortOrder: wallet.SortOrder, IsActive: wallet.IsActive}
}

func transactionMutation(request TransactionRequest) TransactionMutation {
	items := make([]TransactionItemMutation, 0, len(request.Items))
	for _, item := range request.Items {
		items = append(items, TransactionItemMutation{ProductID: item.ProductID, ProductVariantID: item.ProductVariantID,
			ProductName: item.ProductName, VariantName: item.VariantName, UnitPrice: item.UnitPrice,
			Quantity: item.Quantity, Notes: item.Notes, LineTotal: item.LineTotal})
	}
	return TransactionMutation{SavedOrderID: request.SavedOrderID, SavedOrderRevision: request.SavedOrderRevision, OrderTypeID: request.OrderTypeID, RestaurantTableID: request.RestaurantTableID,
		PaymentMethod: request.PaymentMethod, WalletID: request.WalletID, Subtotal: request.Subtotal,
		DiscountAmount: request.DiscountAmount, TaxPercentage: request.TaxPercentage, TaxAmount: request.TaxAmount,
		TotalAmount: request.TotalAmount, PaidAmount: request.PaidAmount, ChangeAmount: request.ChangeAmount,
		RoundingAmount: request.RoundingAmount, TipAmount: request.TipAmount, Items: items}
}

func transactionData(transaction *Transaction) TransactionData {
	items := make([]TransactionItemData, 0, len(transaction.Items))
	for _, item := range transaction.Items {
		items = append(items, TransactionItemData{ID: item.ID, ProductID: item.ProductID, ProductVariantID: item.ProductVariantID,
			ProductName: item.ProductName, VariantName: item.VariantName, UnitPrice: item.UnitPrice,
			Quantity: item.Quantity, Notes: item.Notes, LineTotal: item.LineTotal})
	}
	var wallet *WalletData
	if transaction.Wallet != nil {
		value := walletData(transaction.Wallet)
		wallet = &value
	}
	return TransactionData{ID: transaction.ID, TransactionNumber: transaction.TransactionNumber,
		CashSessionID: transaction.CashSessionID, OrderTypeID: transaction.OrderTypeID,
		RestaurantTableID: transaction.RestaurantTableID, PaymentMethod: transaction.PaymentMethod, Wallet: wallet,
		Subtotal: transaction.Subtotal, DiscountAmount: transaction.DiscountAmount, TaxPercentage: transaction.TaxPercentage,
		TaxAmount: transaction.TaxAmount, TotalAmount: transaction.TotalAmount, PaidAmount: transaction.PaidAmount,
		ChangeAmount: transaction.ChangeAmount, RoundingAmount: transaction.RoundingAmount, TipAmount: transaction.TipAmount,
		TotalQuantity: transaction.TotalQuantity, Status: transaction.Status, PaidAt: transaction.PaidAt, Items: items}
}

// paymentStatusError mencegah detail database bocor ke response API.
func paymentStatusError(err error) *sharederrors.StatusError {
	switch {
	case errors.Is(err, savedorder.ErrAlreadyPaid), errors.Is(err, savedorder.ErrConflict), errors.Is(err, savedorder.ErrNotFound):
		return savedorder.StatusError(err)
	case errors.Is(err, catalog.ErrInsufficientStock):
		return sharederrors.New(http.StatusConflict, "STOK_TIDAK_CUKUP", "Stok berubah atau tidak cukup. Periksa katalog dan pesanan sebelum membayar.", nil)
	case errors.Is(err, catalog.ErrCatalogChanged):
		return sharederrors.New(http.StatusConflict, "KATALOG_BERUBAH", "Produk, variasi, atau harga berubah. Periksa ulang pesanan.", nil)
	case errors.Is(err, ErrUserWithoutOutlet):
		return sharederrors.New(http.StatusForbidden, "USER_TANPA_OUTLET", "User belum terhubung ke outlet.", nil)
	case errors.Is(err, ErrForbidden):
		return sharederrors.Standard(http.StatusForbidden, nil)
	case errors.Is(err, ErrUserNotFound), errors.Is(err, ErrOutletNotFound), errors.Is(err, ErrWalletNotFound):
		return sharederrors.Standard(http.StatusNotFound, nil)
	case errors.Is(err, ErrCashSessionNotFound):
		return sharederrors.New(http.StatusConflict, "SESI_KAS_BELUM_AKTIF", "Buka sesi kas sebelum memproses pembayaran.", nil)
	case errors.Is(err, ErrConflict):
		return sharederrors.Standard(http.StatusConflict, nil)
	case errors.Is(err, ErrValidation), errors.Is(err, ErrInvalidReference):
		return sharederrors.Standard(http.StatusUnprocessableEntity, nil)
	default:
		return sharederrors.Standard(http.StatusInternalServerError, nil)
	}
}
