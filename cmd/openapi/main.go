// Command openapi mengekspor spesifikasi Huma ke api/openapi.yaml.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"be-dikasa/internal/auth"
	"be-dikasa/internal/cash"
	"be-dikasa/internal/catalog"
	"be-dikasa/internal/payment"
	"be-dikasa/internal/refund"
	"be-dikasa/internal/registration"
	"be-dikasa/internal/reports"
	"be-dikasa/internal/reservation"
	"be-dikasa/internal/sales"
	"be-dikasa/internal/savedorder"
	"be-dikasa/internal/server"
	"be-dikasa/internal/shared/supabasestorage"
	sharedvalidator "be-dikasa/internal/shared/validator"
	"be-dikasa/internal/user"
)

type schemaSignInProvider struct{}

func (schemaSignInProvider) SignIn(context.Context, string, string) (auth.Session, error) {
	return auth.Session{}, nil
}

type schemaRegistrationProvider struct{}

func (schemaRegistrationProvider) Register(context.Context, registration.Registration) (registration.Result, error) {
	return registration.Result{}, nil
}

func (schemaRegistrationProvider) ConfirmOTP(context.Context, string, string) error { return nil }

func (schemaRegistrationProvider) ResendOTP(context.Context, string) (registration.Result, error) {
	return registration.Result{}, nil
}

type schemaCashProvider struct{}

func (schemaCashProvider) Open(context.Context, string, int64) (*cash.Session, error) {
	return &cash.Session{}, nil
}

func (schemaCashProvider) Current(context.Context, string) (*cash.Session, error) {
	return &cash.Session{}, nil
}

type schemaPaymentProvider struct{}

func (schemaPaymentProvider) TaxPercentage(context.Context, string) (float64, error) {
	return 0, nil
}
func (schemaPaymentProvider) ListWallets(context.Context, string, bool) ([]payment.Wallet, error) {
	return nil, nil
}
func (schemaPaymentProvider) CreateWallet(context.Context, string, payment.WalletMutation) (*payment.Wallet, error) {
	return &payment.Wallet{}, nil
}
func (schemaPaymentProvider) UpdateWallet(context.Context, string, string, payment.WalletMutation) (*payment.Wallet, error) {
	return &payment.Wallet{}, nil
}
func (schemaPaymentProvider) DeleteWallet(context.Context, string, string) error { return nil }
func (schemaPaymentProvider) CreateTransaction(context.Context, string, payment.TransactionMutation) (*payment.Transaction, error) {
	return &payment.Transaction{}, nil
}

type schemaCatalogProvider struct{}

type schemaSalesProvider struct{}

func (schemaSalesProvider) ListOrders(context.Context, string, sales.OrderFilter) (sales.OrderPage, error) {
	return sales.OrderPage{}, nil
}

func (schemaSalesProvider) ListCustomers(context.Context, string, sales.CustomerFilter) (sales.CustomerPage, error) {
	return sales.CustomerPage{}, nil
}

func (schemaCatalogProvider) ListProductTypes(context.Context, string) ([]catalog.ProductType, error) {
	return nil, nil
}
func (schemaCatalogProvider) ListProducts(context.Context, string, catalog.ProductFilter) ([]catalog.Product, int64, error) {
	return nil, 0, nil
}
func (schemaCatalogProvider) GetProduct(context.Context, string, string, bool) (*catalog.Product, error) {
	return &catalog.Product{}, nil
}
func (schemaCatalogProvider) PrepareProductImageUpload(context.Context, string, string, int64) (supabasestorage.SignedUpload, error) {
	return supabasestorage.SignedUpload{}, nil
}
func (schemaCatalogProvider) CreateProduct(context.Context, string, catalog.ProductMutation) (*catalog.Product, error) {
	return &catalog.Product{}, nil
}
func (schemaCatalogProvider) UpdateProduct(context.Context, string, string, catalog.ProductMutation) (*catalog.Product, error) {
	return &catalog.Product{}, nil
}
func (schemaCatalogProvider) DeleteProduct(context.Context, string, string) error { return nil }
func (schemaCatalogProvider) ListCategories(context.Context, string, string, bool) ([]catalog.ProductCategory, error) {
	return nil, nil
}
func (schemaCatalogProvider) CreateCategory(context.Context, string, string, catalog.CategoryMutation) (*catalog.ProductCategory, error) {
	return &catalog.ProductCategory{}, nil
}
func (schemaCatalogProvider) UpdateCategory(context.Context, string, string, string, catalog.CategoryMutation) (*catalog.ProductCategory, error) {
	return &catalog.ProductCategory{}, nil
}
func (schemaCatalogProvider) DeleteCategory(context.Context, string, string, string) error {
	return nil
}
func (schemaCatalogProvider) ListOrderTypes(context.Context, string, bool) ([]catalog.OrderType, error) {
	return nil, nil
}
func (schemaCatalogProvider) CreateOrderType(context.Context, string, catalog.OrderTypeMutation) (*catalog.OrderType, error) {
	return &catalog.OrderType{}, nil
}
func (schemaCatalogProvider) UpdateOrderType(context.Context, string, string, catalog.OrderTypeMutation) (*catalog.OrderType, error) {
	return &catalog.OrderType{}, nil
}
func (schemaCatalogProvider) DeleteOrderType(context.Context, string, string) error { return nil }

type schemaUserRepository struct{}

func (schemaUserRepository) FindByIdentifier(context.Context, string) (*user.User, error) {
	return nil, user.ErrNotFound
}

func (schemaUserRepository) FindByID(context.Context, string) (*user.User, error) {
	return nil, user.ErrNotFound
}

func main() {
	output := flag.String("output", "api/openapi.yaml", "lokasi file OpenAPI")
	flag.Parse()

	userService := user.NewService(schemaUserRepository{})
	router := server.NewRouter(server.Dependencies{
		StockHandler:        catalog.NewStockHandler(nil),
		ExportHandler:       catalog.NewExportHandler(nil),
		AuthHandler:         auth.NewHandler(schemaSignInProvider{}, sharedvalidator.New()),
		CashHandler:         cash.NewHandler(schemaCashProvider{}, sharedvalidator.New()),
		ReportsHandler:      reports.NewHandler(nil),
		MovementHandler:     cash.NewMovementHandler(nil),
		CashClosingHandler:  cash.NewReconciliationHandler(nil),
		CatalogHandler:      catalog.NewHandler(schemaCatalogProvider{}, sharedvalidator.New()),
		RefundHandler:       refund.NewHandler(nil),
		SavedOrderHandler:   savedorder.NewHandler(nil),
		PaymentHandler:      payment.NewHandler(schemaPaymentProvider{}),
		RegistrationHandler: registration.NewHandler(schemaRegistrationProvider{}, sharedvalidator.New()),
		ReservationHandler:  reservation.NewHandler(nil, sharedvalidator.New()),
		SalesHandler:        sales.NewHandler(schemaSalesProvider{}),
		UserHandler:         user.NewHandler(userService),
	})
	request := httptest.NewRequest(http.MethodGet, "/dikasa/v1/openapi.yaml", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		panic(fmt.Errorf("menghasilkan OpenAPI: status %d: %s", response.Code, response.Body.String()))
	}
	if response.Body.Len() == 0 {
		panic(errors.New("menghasilkan OpenAPI: dokumen kosong"))
	}
	if err := os.WriteFile(*output, response.Body.Bytes(), 0o644); err != nil {
		panic(fmt.Errorf("menulis OpenAPI: %w", err))
	}
}
