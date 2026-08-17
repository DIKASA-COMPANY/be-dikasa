package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	firebaseauth "firebase.google.com/go/v4/auth"

	"be-dikasa/internal/auth"
	"be-dikasa/internal/registration"
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
	return &user.User{ID: id, RoleID: "role-id", Username: "kasir", PhoneNumber: "08123456789", FirstName: "Dika", LastName: "Sa", IsActive: true}, nil
}

func newTestRouter() http.Handler {
	userService := user.NewService(routerUserRepository{})
	return NewRouter(Dependencies{
		AuthHandler:         auth.NewHandler(routerSignInProvider{}, sharedvalidator.New()),
		RegistrationHandler: registration.NewHandler(routerRegistrationProvider{}, sharedvalidator.New()),
		UserHandler:         user.NewHandler(userService),
		TokenVerifier:       stubTokenVerifier{token: &firebaseauth.Token{UID: "user-id"}},
	})
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
