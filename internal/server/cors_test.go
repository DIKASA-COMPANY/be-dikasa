package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSAllowsConfiguredPreflightBeforeAuthentication(t *testing.T) {
	router := gin.New()
	router.Use(CORS([]string{"http://127.0.0.1:8080"}))
	router.Use(AuthMiddleware(stubTokenVerifier{}, nil))
	router.POST("/dikasa/v1/login", func(c *gin.Context) { c.Status(200) })
	for _, origin := range []string{"http://127.0.0.1:8080", "http://untrusted.test"} {
		request := httptest.NewRequest(http.MethodOptions, "/dikasa/v1/login", nil)
		request.Header.Set("Origin", origin)
		request.Header.Set("Access-Control-Request-Method", "POST")
		request.Header.Set("Access-Control-Request-Headers", "content-type, authorization")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if origin == "http://127.0.0.1:8080" {
			if response.Code != 204 || response.Header().Get("Access-Control-Allow-Origin") != origin {
				t.Fatalf("preflight failed: %d", response.Code)
			}
		} else if response.Code != 403 || response.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("allowed untrusted origin")
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/dikasa/v1/login", nil)
	request.Header.Set("Origin", "http://127.0.0.1:8080")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != 401 || response.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Fatal("CORS bypassed auth or hid auth error")
	}
}

func TestDisabledRegistrationReturnsUnavailableWithoutCreatingAccounts(t *testing.T) {
	router := NewRouter(Dependencies{RegistrationDisabled: true})
	for _, path := range []string{"/register", "/register/otp/confirm", "/register/otp/resend"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("POST", "/dikasa/v1"+path, nil))
		if response.Code != 503 {
			t.Fatalf("expected registration disabled for %s, got %d", path, response.Code)
		}
	}
}

func TestCORSAllowsSameOriginSwaggerWritesWithoutCrossOriginAllowlist(t *testing.T) {
	router := gin.New()
	router.Use(CORS(nil))
	router.POST("/dikasa/v1/login", func(c *gin.Context) { c.Status(200) })
	request := httptest.NewRequest("POST", "http://localhost:8081/dikasa/v1/login", nil)
	request.Header.Set("Origin", "http://localhost:8081")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("same-origin login blocked: %d", response.Code)
	}
}
