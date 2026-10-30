package supabaseauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientSignsInWithPassword(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/auth/v1/token" || request.URL.Query().Get("grant_type") != "password" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		if request.Header.Get("apikey") != "publishable-key" {
			t.Fatalf("missing publishable key: %q", request.Header.Get("apikey"))
		}
		var body map[string]string
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body["email"] != "owner@example.com" || body["password"] != "rahasia123" {
			t.Fatalf("unexpected request body: %v, %v", body, err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"access_token":"access","refresh_token":"refresh","expires_in":3600,"token_type":"bearer","user":{"id":"11111111-1111-4111-8111-111111111111","email":"owner@example.com"}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "publishable-key", "secret-key", WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	session, err := client.SignIn(context.Background(), "owner@example.com", "rahasia123")
	if err != nil {
		t.Fatal(err)
	}
	if session.AccessToken != "access" || session.RefreshToken != "refresh" || session.UserID != "11111111-1111-4111-8111-111111111111" || session.ExpiresIn != 3600 {
		t.Fatalf("unexpected session: %+v", session)
	}
}

func TestClientImportsExistingBcryptUser(t *testing.T) {
	var created map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/auth/v1/admin/users" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("apikey") != "secret-key" || request.Header.Get("Authorization") != "Bearer secret-key" {
			t.Fatalf("admin credentials missing: apikey=%q authorization=%q", request.Header.Get("apikey"), request.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(request.Body).Decode(&created); err != nil {
			t.Fatal(err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"11111111-1111-4111-8111-111111111111","email":"owner@example.com"}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "publishable-key", "secret-key", WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	err = client.ImportUser(context.Background(), UserImport{
		ID: "11111111-1111-4111-8111-111111111111", Email: "owner@example.com", PasswordHash: "$2a$10$hash", EmailConfirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created["id"] == "" || created["password_hash"] != "$2a$10$hash" || created["email_confirm"] != true {
		t.Fatalf("unexpected import body: %#v", created)
	}
}

func TestClientUpdatesExistingUserCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPut || request.URL.Path != "/auth/v1/admin/users/11111111-1111-4111-8111-111111111111" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil || body["password_hash"] != "$2a$10$replacement" || body["email"] != "new@example.com" || body["email_confirm"] != true {
			t.Fatalf("unexpected update body: %#v, %v", body, err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"11111111-1111-4111-8111-111111111111"}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "publishable-key", "secret-key", WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	if err = client.UpdateUserCredentials(context.Background(), "11111111-1111-4111-8111-111111111111", "new@example.com", "$2a$10$replacement"); err != nil {
		t.Fatal(err)
	}
}

func TestClientFindsExactUserByEmail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/auth/v1/admin/users" || request.URL.Query().Get("filter") != "owner@example.com" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		_, _ = writer.Write([]byte(`{"users":[{"id":"oauth-id","email":"owner@example.com"},{"id":"other","email":"other@example.com"}]}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "publishable", "secret", WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	id, err := client.FindUserByEmail(context.Background(), " OWNER@example.com ")
	if err != nil || id != "oauth-id" {
		t.Fatalf("unexpected user lookup: id=%q err=%v", id, err)
	}
}

func TestClientVerifiesAccessTokenWithAuthServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/auth/v1/user" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("apikey") != "publishable-key" || request.Header.Get("Authorization") != "Bearer access-token" {
			t.Fatalf("unexpected headers: %#v", request.Header)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"22222222-2222-4222-8222-222222222222","email":"oauth@example.com","email_confirmed_at":"2026-10-29T00:00:00Z","role":"authenticated","app_metadata":{"provider":"google"}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "publishable-key", "secret-key", WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	identity, err := client.VerifyAccessToken(context.Background(), "access-token")
	if err != nil {
		t.Fatal(err)
	}
	if identity.ID != "22222222-2222-4222-8222-222222222222" || identity.Email != "oauth@example.com" || !identity.EmailConfirmed || identity.Claims["provider"] != "google" {
		t.Fatalf("unexpected identity: %+v", identity)
	}
}

func TestClientRejectsInvalidConfigurationAndCredentials(t *testing.T) {
	if _, err := NewClient("not-a-url", "publishable", "secret"); err == nil {
		t.Fatal("expected invalid project URL error")
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"error_code":"invalid_credentials"}`))
	}))
	defer server.Close()
	client, err := NewClient(server.URL, "publishable", "secret", WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.SignIn(context.Background(), "owner@example.com", "wrong-password"); err != ErrInvalidCredentials {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
}
