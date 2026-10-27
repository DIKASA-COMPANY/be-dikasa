package server

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"strings"
	"testing"
	"time"

	firebaseauth "firebase.google.com/go/v4/auth"

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
	sharedvalidator "be-dikasa/internal/shared/validator"
	"be-dikasa/internal/user"
)

type routerSignInProvider struct{}

func (routerSignInProvider) SignIn(context.Context, string, string) (auth.Session, error) {
	return auth.Session{IDToken: "id-token", RefreshToken: "refresh-token", ExpiresIn: 3600, FirebaseUID: "user-id"}, nil
}

type routerRegistrationProvider struct{}

func (routerRegistrationProvider) Register(context.Context, registration.Registration) (registration.Result, error) {
	return registration.Result{Email: "budi@example.com", OTPExpiresIn: 300, ResendAvailableIn: 60}, nil
}

func (routerRegistrationProvider) ConfirmOTP(context.Context, string, string) error { return nil }

func (routerRegistrationProvider) ResendOTP(context.Context, string) (registration.Result, error) {
	return registration.Result{Email: "budi@example.com", OTPExpiresIn: 300, ResendAvailableIn: 60}, nil
}

type routerUserRepository struct{}

func (routerUserRepository) FindByIdentifier(context.Context, string) (*user.User, error) {
	return nil, user.ErrNotFound
}

func (routerUserRepository) FindByID(_ context.Context, id string) (*user.User, error) {
	outletID := "outlet-id"
	return &user.User{ID: id, RoleID: "role-id", OutletID: &outletID, Username: "kasir", PhoneNumber: "08123456789", FirstName: "Dika", LastName: "Sa", IsActive: true}, nil
}

func (routerUserRepository) UpdateProfile(context.Context, *user.User) error          { return nil }
func (routerUserRepository) UpdatePINHash(context.Context, string, string) error      { return nil }
func (routerUserRepository) UpdatePasswordHash(context.Context, string, string) error { return nil }

type routerCashProvider struct{}

func (routerCashProvider) Open(_ context.Context, userID string, openingCash int64) (*cash.Session, error) {
	return &cash.Session{
		ID: "session-id", OutletID: "outlet-id", UserID: userID, OpeningCash: openingCash,
		Status: cash.StatusOpen, OpenedAt: time.Date(2026, time.August, 19, 1, 30, 0, 0, time.UTC),
	}, nil
}

func (routerCashProvider) Current(_ context.Context, userID string) (*cash.Session, error) {
	return routerCashProvider{}.Open(context.Background(), userID, 500_000)
}

type routerPaymentProvider struct{}

func (routerPaymentProvider) TaxPercentage(context.Context, string) (float64, error) {
	return 10, nil
}
func (routerPaymentProvider) ListWallets(context.Context, string, bool) ([]payment.Wallet, error) {
	iconKey := "gopay"
	return []payment.Wallet{{ID: "11111111-1111-4111-8111-111111111111", Code: "gopay", Name: "GoPay", IconKey: &iconKey, SortOrder: 10, IsActive: true}}, nil
}
func (routerPaymentProvider) CreateWallet(_ context.Context, _ string, mutation payment.WalletMutation) (*payment.Wallet, error) {
	return &payment.Wallet{ID: "11111111-1111-4111-8111-111111111111", Code: mutation.Code, Name: mutation.Name, IconKey: mutation.IconKey, SortOrder: mutation.SortOrder, IsActive: true}, nil
}
func (routerPaymentProvider) UpdateWallet(_ context.Context, _ string, id string, mutation payment.WalletMutation) (*payment.Wallet, error) {
	return &payment.Wallet{ID: id, Code: mutation.Code, Name: mutation.Name, IconKey: mutation.IconKey, SortOrder: mutation.SortOrder, IsActive: mutation.IsActive}, nil
}
func (routerPaymentProvider) DeleteWallet(context.Context, string, string) error { return nil }
func (routerPaymentProvider) CreateTransaction(_ context.Context, userID string, mutation payment.TransactionMutation) (*payment.Transaction, error) {
	items := make([]payment.TransactionItem, 0, len(mutation.Items))
	totalQuantity := 0
	for _, item := range mutation.Items {
		totalQuantity += item.Quantity
		items = append(items, payment.TransactionItem{ID: "44444444-4444-4444-8444-444444444444", ProductID: item.ProductID,
			ProductVariantID: item.ProductVariantID, ProductName: item.ProductName, VariantName: item.VariantName,
			UnitPrice: item.UnitPrice, Quantity: item.Quantity, Notes: item.Notes, LineTotal: item.LineTotal})
	}
	return &payment.Transaction{ID: "33333333-3333-4333-8333-333333333333", TransactionNumber: "TRX-TEST-001",
		CashSessionID: "55555555-5555-4555-8555-555555555555", CreatedBy: userID, OrderTypeID: mutation.OrderTypeID,
		RestaurantTableID: mutation.RestaurantTableID, PaymentMethod: mutation.PaymentMethod, Subtotal: mutation.Subtotal,
		DiscountAmount: mutation.DiscountAmount, TaxPercentage: mutation.TaxPercentage, TaxAmount: mutation.TaxAmount,
		TotalAmount: mutation.TotalAmount, PaidAmount: mutation.PaidAmount, ChangeAmount: mutation.ChangeAmount,
		RoundingAmount: mutation.RoundingAmount, TipAmount: mutation.TipAmount, TotalQuantity: totalQuantity,
		Status: payment.TransactionStatusPaid, PaidAt: time.Date(2026, time.August, 20, 10, 0, 0, 0, time.UTC), Items: items}, nil
}

type routerCatalogProvider struct{}

func (routerCatalogProvider) ListProductTypes(context.Context, string) ([]catalog.ProductType, error) {
	return nil, nil
}
func (routerCatalogProvider) ListProducts(context.Context, string, catalog.ProductFilter) ([]catalog.Product, int64, error) {
	return nil, 0, nil
}
func (routerCatalogProvider) GetProduct(context.Context, string, string, bool) (*catalog.Product, error) {
	return &catalog.Product{}, nil
}
func (routerCatalogProvider) CreateProduct(context.Context, string, catalog.ProductMutation) (*catalog.Product, error) {
	return &catalog.Product{}, nil
}
func (routerCatalogProvider) UpdateProduct(context.Context, string, string, catalog.ProductMutation) (*catalog.Product, error) {
	return &catalog.Product{}, nil
}
func (routerCatalogProvider) DeleteProduct(context.Context, string, string) error { return nil }
func (routerCatalogProvider) ListCategories(context.Context, string, string, bool) ([]catalog.ProductCategory, error) {
	return nil, nil
}
func (routerCatalogProvider) CreateCategory(context.Context, string, string, catalog.CategoryMutation) (*catalog.ProductCategory, error) {
	return &catalog.ProductCategory{}, nil
}
func (routerCatalogProvider) UpdateCategory(context.Context, string, string, string, catalog.CategoryMutation) (*catalog.ProductCategory, error) {
	return &catalog.ProductCategory{}, nil
}
func (routerCatalogProvider) DeleteCategory(context.Context, string, string, string) error {
	return nil
}
func (routerCatalogProvider) ListOrderTypes(context.Context, string, bool) ([]catalog.OrderType, error) {
	return nil, nil
}
func (routerCatalogProvider) CreateOrderType(context.Context, string, catalog.OrderTypeMutation) (*catalog.OrderType, error) {
	return &catalog.OrderType{}, nil
}
func (routerCatalogProvider) UpdateOrderType(context.Context, string, string, catalog.OrderTypeMutation) (*catalog.OrderType, error) {
	return &catalog.OrderType{}, nil
}
func (routerCatalogProvider) DeleteOrderType(context.Context, string, string) error { return nil }

type routerReservationProvider struct{}

func (routerReservationProvider) ListTypes(context.Context, string) ([]reservation.Type, error) {
	return []reservation.Type{{ID: "11111111-1111-4111-8111-111111111111", Code: "regular", Name: "Reguler"}}, nil
}
func (routerReservationProvider) ListTables(context.Context, string, *time.Time) ([]reservation.Table, error) {
	return nil, nil
}
func (routerReservationProvider) CreateTable(context.Context, string, reservation.TableMutation) (*reservation.Table, error) {
	return &reservation.Table{}, nil
}
func (routerReservationProvider) UpdateTable(context.Context, string, string, reservation.TableMutation) (*reservation.Table, error) {
	return &reservation.Table{}, nil
}
func (routerReservationProvider) DeleteTable(context.Context, string, string) error { return nil }
func (routerReservationProvider) List(context.Context, string, reservation.Filter) ([]reservation.Reservation, int64, error) {
	return nil, 0, nil
}
func (routerReservationProvider) Get(context.Context, string, string) (*reservation.Reservation, error) {
	return &reservation.Reservation{}, nil
}
func (routerReservationProvider) Create(_ context.Context, userID string, mutation reservation.Mutation) (*reservation.Reservation, error) {
	return &reservation.Reservation{ID: "33333333-3333-4333-8333-333333333333", CreatedBy: userID,
		CustomerName: mutation.CustomerName, CustomerContact: mutation.CustomerContact, ReservedAt: mutation.ReservedAt,
		Notes: mutation.Notes, PrintReservation: mutation.PrintReservation, SendReservation: mutation.SendReservation,
		Status: reservation.StatusBooked, ReservationType: reservation.Type{ID: mutation.ReservationTypeID, Code: "regular", Name: "Reguler"},
		RestaurantTable: reservation.Table{ID: mutation.RestaurantTableID, Name: "Meja 1", Capacity: 4, IsActive: true}}, nil
}
func (routerReservationProvider) Update(context.Context, string, string, reservation.Mutation) (*reservation.Reservation, error) {
	return &reservation.Reservation{}, nil
}
func (routerReservationProvider) Cancel(context.Context, string, string) error { return nil }

type routerSalesProvider struct{}

func (routerSalesProvider) ListOrders(context.Context, string, sales.OrderFilter) (sales.OrderPage, error) {
	tableID, tableName := "22222222-2222-4222-8222-222222222222", "Meja 5"
	return sales.OrderPage{Data: []sales.Order{{
		ID: "11111111-1111-4111-8111-111111111111", ReceiptNumber: "5834608256035020", CustomerName: "Jane Doe",
		OrderedAt: time.Date(2026, time.August, 28, 10, 0, 0, 0, time.UTC), OrderTypeID: "33333333-3333-4333-8333-333333333333",
		OrderTypeCode: "table_order", OrderTypeName: "Dine-In", RestaurantTableID: &tableID, RestaurantTableName: &tableName,
		TotalQuantity: 7, TotalAmount: 120_000, Status: sales.StatusPaid,
	}}, Page: 1, PageSize: 30, Total: 1}, nil
}

func (routerSalesProvider) ListCustomers(context.Context, string, sales.CustomerFilter) (sales.CustomerPage, error) {
	genderID, genderName, email := "44444444-4444-4444-8444-444444444444", "female", "jane@example.com"
	return sales.CustomerPage{Data: []sales.Customer{{
		ID: "55555555-5555-4555-8555-555555555555", Name: "Jane Doe", Address: "Jalan Cikawao No. 10, Bandung", Points: 2321,
		GenderID: &genderID, GenderName: &genderName, PhoneNumber: "08123456789", Email: &email,
	}}, Page: 1, PageSize: 30, Total: 1}, nil
}

func newTestRouter() http.Handler {
	userService := user.NewService(routerUserRepository{})
	return NewRouter(Dependencies{
		StockHandler:        catalog.NewStockHandler(nil),
		ExportHandler:       catalog.NewExportHandler(nil),
		AuthHandler:         auth.NewHandler(routerSignInProvider{}, sharedvalidator.New()),
		RegistrationHandler: registration.NewHandler(routerRegistrationProvider{}, sharedvalidator.New()),
		ReservationHandler:  reservation.NewHandler(routerReservationProvider{}, sharedvalidator.New()),
		UserHandler:         user.NewHandler(userService),
		CashHandler:         cash.NewHandler(routerCashProvider{}, sharedvalidator.New()),
		ReportsHandler:      reports.NewHandler(nil),
		CashClosingHandler:  cash.NewReconciliationHandler(nil),
		MovementHandler:     cash.NewMovementHandler(nil),
		CatalogHandler:      catalog.NewHandler(routerCatalogProvider{}, sharedvalidator.New()),
		RefundHandler:       refund.NewHandler(nil),
		SavedOrderHandler:   savedorder.NewHandler(nil),
		PaymentHandler:      payment.NewHandler(routerPaymentProvider{}),
		SalesHandler:        sales.NewHandler(routerSalesProvider{}),
		TokenVerifier:       stubTokenVerifier{token: &firebaseauth.Token{UID: "user-id"}},
	})
}

func TestRouterPublishesOrderAndRegisteredCustomerLists(t *testing.T) {
	router := newTestRouter()
	tests := []struct {
		path     string
		contains []string
	}{
		{path: "/dikasa/v1/orders?search=Jane&page=1&page_size=30", contains: []string{`"receipt_number":"5834608256035020"`, `"customer_name":"Jane Doe"`, `"total_amount":120000`, `"name":"Meja 5"`}},
		{path: "/dikasa/v1/customers?search=Jane&page=1&page_size=30", contains: []string{`"name":"Jane Doe"`, `"points":2321`, `"phone_number":"08123456789"`, `"email":"jane@example.com"`}},
	}
	for _, test := range tests {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		request.Header.Set("Authorization", "Bearer valid-token")
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		if response.Code != http.StatusOK {
			t.Fatalf("GET %s: expected 200, got %d: %s", test.path, response.Code, response.Body.String())
		}
		for _, fragment := range test.contains {
			if !strings.Contains(response.Body.String(), fragment) {
				t.Fatalf("GET %s missing %s: %s", test.path, fragment, response.Body.String())
			}
		}
	}
}

func TestRouterRejectsInvalidSalesListFilters(t *testing.T) {
	router := newTestRouter()
	for _, path := range []string{
		"/dikasa/v1/orders?status=unknown",
		"/dikasa/v1/customers?page_size=101",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer valid-token")
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("GET %s: expected 422, got %d: %s", path, response.Code, response.Body.String())
		}
	}
}

func TestRouterPublishesOnlyCurrentOutletTaxPercentage(t *testing.T) {
	router := newTestRouter()
	request := httptest.NewRequest(http.MethodGet, "/dikasa/v1/payments/tax-percentage", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected tax percentage status: %d %s", response.Code, response.Body.String())
	}
	if body := strings.TrimSpace(response.Body.String()); body != `{"tax_percentage":10}` {
		t.Fatalf("unexpected tax percentage response: %s", body)
	}
}

func TestRouterPublishesWalletDropdownAndTransactionSave(t *testing.T) {
	router := newTestRouter()
	walletRequest := httptest.NewRequest(http.MethodGet, "/dikasa/v1/wallets", nil)
	walletRequest.Header.Set("Authorization", "Bearer valid-token")
	walletResponse := httptest.NewRecorder()
	router.ServeHTTP(walletResponse, walletRequest)
	if walletResponse.Code != http.StatusOK || !strings.Contains(walletResponse.Body.String(), `"code":"gopay"`) {
		t.Fatalf("unexpected wallet list: %d %s", walletResponse.Code, walletResponse.Body.String())
	}

	body := `{"order_type_id":"11111111-1111-4111-8111-111111111111","payment_method":"cash","subtotal":21000,"discount_amount":0,"tax_percentage":10,"tax_amount":2100,"total_amount":23100,"paid_amount":25000,"change_amount":1900,"rounding_amount":0,"tip_amount":0,"items":[{"product_id":"22222222-2222-4222-8222-222222222222","product_name":"Nasi Goreng","unit_price":21000,"quantity":1,"line_total":21000}]}`
	transactionRequest := httptest.NewRequest(http.MethodPost, "/dikasa/v1/transactions", strings.NewReader(body))
	transactionRequest.Header.Set("Content-Type", "application/json")
	transactionRequest.Header.Set("Authorization", "Bearer valid-token")
	transactionResponse := httptest.NewRecorder()
	router.ServeHTTP(transactionResponse, transactionRequest)
	if transactionResponse.Code != http.StatusCreated || !strings.Contains(transactionResponse.Body.String(), `"transaction_number":"TRX-TEST-001"`) || !strings.Contains(transactionResponse.Body.String(), `"product_name":"Nasi Goreng"`) {
		t.Fatalf("unexpected transaction response: %d %s", transactionResponse.Code, transactionResponse.Body.String())
	}
}

func TestRouterPublishesAuthenticatedCashSessionFlow(t *testing.T) {
	router := newTestRouter()

	open := httptest.NewRequest(http.MethodPost, "/dikasa/v1/cash-sessions", strings.NewReader(`{"opening_cash":500000}`))
	open.Header.Set("Content-Type", "application/json")
	open.Header.Set("Authorization", "Bearer valid-token")
	openResponse := httptest.NewRecorder()
	router.ServeHTTP(openResponse, open)
	if openResponse.Code != http.StatusCreated || !strings.Contains(openResponse.Body.String(), `"opening_cash":500000`) {
		t.Fatalf("unexpected opening response: %d %s", openResponse.Code, openResponse.Body.String())
	}

	current := httptest.NewRequest(http.MethodGet, "/dikasa/v1/cash-sessions/current", nil)
	current.Header.Set("Authorization", "Bearer valid-token")
	currentResponse := httptest.NewRecorder()
	router.ServeHTTP(currentResponse, current)
	if currentResponse.Code != http.StatusOK || !strings.Contains(currentResponse.Body.String(), `"status":"open"`) {
		t.Fatalf("unexpected current response: %d %s", currentResponse.Code, currentResponse.Body.String())
	}
}

func TestRouterPublishesRegistrationOTPFlowWithoutToken(t *testing.T) {
	router := newTestRouter()
	requests := []struct {
		path string
		body string
		want int
	}{
		{"/dikasa/v1/register", `{"business_name":"Warung Dikasa","business_address":"Jl. Merdeka 10","business_phone":"081234567890","owner_name":"Budi Santoso","owner_date_of_birth":"1990-01-02","owner_phone":"081298765432","owner_email":"budi@example.com","username":"budisantoso","password":"rahasia123","password_confirmation":"rahasia123","verification_email":"budi@example.com"}`, http.StatusCreated},
		{"/dikasa/v1/register/otp/confirm", `{"email":"budi@example.com","otp":"12345"}`, http.StatusOK},
		{"/dikasa/v1/register/otp/resend", `{"email":"budi@example.com"}`, http.StatusOK},
	}
	for _, test := range requests {
		request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != test.want {
			t.Fatalf("POST %s: expected %d, got %d: %s", test.path, test.want, response.Code, response.Body.String())
		}
	}
}

func TestRouterPublishesLoginAndProtectsFeatureRoutes(t *testing.T) {
	router := newTestRouter()

	login := httptest.NewRequest(http.MethodPost, "/dikasa/v1/login", strings.NewReader(`{"identifier":"kasir","password":"rahasia123"}`))
	login.Header.Set("Content-Type", "application/json")
	loginResponse := httptest.NewRecorder()
	router.ServeHTTP(loginResponse, login)
	if loginResponse.Code != http.StatusOK || !strings.Contains(loginResponse.Body.String(), `"access_token":"id-token"`) {
		t.Fatalf("unexpected login response: %d %s", loginResponse.Code, loginResponse.Body.String())
	}

	unauthorized := httptest.NewRequest(http.MethodGet, "/dikasa/v1/users/me", nil)
	unauthorizedResponse := httptest.NewRecorder()
	router.ServeHTTP(unauthorizedResponse, unauthorized)
	if unauthorizedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, unauthorizedResponse.Code)
	}

	profile := httptest.NewRequest(http.MethodGet, "/dikasa/v1/users/me", nil)
	profile.Header.Set("Authorization", "Bearer valid-token")
	profileResponse := httptest.NewRecorder()
	router.ServeHTTP(profileResponse, profile)
	if profileResponse.Code != http.StatusOK || !strings.Contains(profileResponse.Body.String(), `"username":"kasir"`) {
		t.Fatalf("unexpected profile response: %d %s", profileResponse.Code, profileResponse.Body.String())
	}
}

func TestRouterPublishesSelfProfileUpdate(t *testing.T) {
	router := newTestRouter()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("first_name", "Janet")
	_ = writer.WriteField("phone_number", "081234567890")
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPatch, "/dikasa/v1/users/me", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"first_name":"Janet"`) {
		t.Fatalf("unexpected profile update: %d %s", response.Code, response.Body.String())
	}
}

func TestRouterPublishesReservationCreation(t *testing.T) {
	router := newTestRouter()
	request := httptest.NewRequest(http.MethodPost, "/dikasa/v1/reservations", strings.NewReader(`{"customer_name":"Sari","customer_contact":"081234567890","reservation_type_id":"11111111-1111-4111-8111-111111111111","restaurant_table_id":"22222222-2222-4222-8222-222222222222","reserved_at":"2026-12-31T19:00:00+07:00","notes":"Dekat jendela","print_reservation":true,"send_reservation":false}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"status":"booked"`) || !strings.Contains(response.Body.String(), `"customer_name":"Sari"`) {
		t.Fatalf("unexpected reservation response: %d %s", response.Code, response.Body.String())
	}
}

func TestRouterAcceptsFigmaProductMultipartWithJSONVariations(t *testing.T) {
	router := newTestRouter()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for name, value := range map[string]string{
		"name": "Nasi Goreng", "category_id": "11111111-1111-4111-8111-111111111111",
		"product_type": "food", "selling_price": "21000", "online_selling_price": "23000",
		"cost_price": "12000", "stock_status": "available", "description": "Pedas",
	} {
		if err := writer.WriteField(name, value); err != nil {
			t.Fatal(err)
		}
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="variations"`)
	header.Set("Content-Type", "application/json")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write([]byte(`[{"name":"Jumbo","selling_price":28000,"stock_status":"available","sort_order":10}]`)); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/dikasa/v1/products", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("unexpected product response: %d %s", response.Code, response.Body.String())
	}
}

func TestRouterServesSwaggerAndOpenAPIWithoutToken(t *testing.T) {
	router := newTestRouter()
	for _, test := range []struct {
		path     string
		contains string
	}{
		{"/dikasa/v1/docs", "swagger-ui"},
		{"/dikasa/v1/openapi.json", `"/users/me"`},
		{"/dikasa/v1/openapi.yaml", "/users/me:"},
	} {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(strings.ToLower(response.Body.String()), strings.ToLower(test.contains)) {
			t.Fatalf("GET %s: %d %s", test.path, response.Code, response.Body.String())
		}
	}
}

func TestRouterUsesIndonesianJSONErrorsAndStrictInput(t *testing.T) {
	router := newTestRouter()
	request := httptest.NewRequest(http.MethodPost, "/dikasa/v1/login", strings.NewReader(`{"identifier":"kasir","password":"rahasia123","asing":true}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "Data yang dikirim tidak memenuhi ketentuan") {
		t.Fatalf("unexpected strict validation response: %d %s", response.Code, response.Body.String())
	}
}

func TestCommittedOpenAPIMatchesServerOutput(t *testing.T) {
	committed, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatalf("read committed OpenAPI: %v", err)
	}
	router := newTestRouter()
	request := httptest.NewRequest(http.MethodGet, "/dikasa/v1/openapi.yaml", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if !bytes.Equal(committed, response.Body.Bytes()) {
		t.Fatal("api/openapi.yaml tidak sinkron; jalankan go generate ./internal/server")
	}
}

func TestRouterRejectsOversizedProductMultipartBeforeParsing(t *testing.T) {
	router := newTestRouter()
	body := bytes.NewReader(make([]byte, 4*1024*1024+1))
	request := httptest.NewRequest(http.MethodPost, "/dikasa/v1/products", body)
	request.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 before multipart parse, got %d", response.Code)
	}
}

func (routerUserRepository) ListGenders(context.Context) ([]user.Gender, error) {
	return []user.Gender{{ID: "11111111-1111-4111-8111-111111111111", Name: "Pria"}}, nil
}
func TestRouterAccountReferenceAndOptionalProfileFields(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest("GET", "/dikasa/v1/users/me/genders", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatal("reference route must require authentication", w.Code)
	}
	req = httptest.NewRequest("GET", "/dikasa/v1/users/me/genders", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"name":"Pria"`) {
		t.Fatal("reference route", w.Code, w.Body.String())
	}
	var body bytes.Buffer
	f := multipart.NewWriter(&body)
	for k, v := range map[string]string{"date_of_birth": "1990-08-10", "personal_address": "Home", "gender_id": "11111111-1111-4111-8111-111111111111", "role_id": "forged", "outlet_id": "forged"} {
		f.WriteField(k, v)
	}
	f.Close()
	req = httptest.NewRequest("PATCH", "/dikasa/v1/users/me", &body)
	req.Header.Set("Content-Type", f.FormDataContentType())
	req.Header.Set("Authorization", "Bearer valid-token")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"personal_address":"Home"`) || !strings.Contains(w.Body.String(), `"date_of_birth":"1990-08-10"`) || strings.Contains(w.Body.String(), "forged") {
		t.Fatal("personal profile contract", w.Code, w.Body.String())
	}
}
func TestRouterRejectsOversizedAccountMultipart(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest("PATCH", "/dikasa/v1/users/me", strings.NewReader("bad body"))
	req.ContentLength = 4*1024*1024 + 1
	req.Header.Set("Content-Type", "multipart/form-data; boundary=test")
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 413 {
		t.Fatal("unbounded profile multipart", w.Code)
	}
}

func TestRouterInitialPINAcceptsOmittedCurrentPIN(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest("PATCH", "/dikasa/v1/users/me/pin", strings.NewReader(`{"new_pin":"012345","new_pin_confirmation":"012345"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 204 {
		t.Fatal("initial PIN HTTP contract", w.Code, w.Body.String())
	}
}
