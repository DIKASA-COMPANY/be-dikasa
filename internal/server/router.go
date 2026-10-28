// Package server merakit transport HTTP Gin, Huma, middleware, dan dokumentasi API.
package server

//go:generate go run ../../cmd/openapi -output ../../api/openapi.yaml

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"

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
	sharederrors "be-dikasa/internal/shared/errors"
	sharedresponse "be-dikasa/internal/shared/response"
	"be-dikasa/internal/user"
)

const apiPrefix = "/dikasa/v1"

// Dependencies berisi handler feature dan layanan token yang dibutuhkan router.
type Dependencies struct {
	ReportsHandler       *reports.Handler
	RefundHandler        *refund.Handler
	SavedOrderHandler    *savedorder.Handler
	StockHandler         *catalog.StockHandler
	ExportHandler        *catalog.ExportHandler
	CORSOrigins          []string
	RegistrationDisabled bool
	AuthHandler          *auth.Handler
	CashHandler          *cash.Handler
	CashClosingHandler   *cash.ReconciliationHandler
	MovementHandler      *cash.MovementHandler
	CatalogHandler       *catalog.Handler
	PaymentHandler       *payment.Handler
	RegistrationHandler  *registration.Handler
	ReservationHandler   *reservation.Handler
	SalesHandler         *sales.Handler
	UserHandler          *user.Handler
	TokenVerifier        TokenVerifier
}

// NewRouter registers Huma operations with configured CORS, registration and Bearer authentication.
func NewRouter(dependencies Dependencies) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	configureHumaErrors()

	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(gin.CustomRecovery(func(c *gin.Context, _ any) {
		sharedresponse.WriteError(c.Writer, sharederrors.Standard(http.StatusInternalServerError, nil))
		c.Abort()
	}))
	router.Use(CORS(dependencies.CORSOrigins))
	// Fail before invoking registration handlers, so no inactive accounts or OTPs are stored.
	if dependencies.RegistrationDisabled {
		router.Use(func(c *gin.Context) {
			if c.Request.URL.Path == apiPrefix+"/register" || strings.HasPrefix(c.Request.URL.Path, apiPrefix+"/register/otp/") {
				sharedresponse.WriteError(c.Writer, sharederrors.New(http.StatusServiceUnavailable, "REGISTRASI_DINONAKTIFKAN", "Registrasi email belum tersedia. Gunakan akun pengembangan lokal.", nil))
				c.Abort()
				return
			}
			c.Next()
		})
	}
	router.Use(AuthMiddleware(dependencies.TokenVerifier, func(path string) bool {
		return isPublicPath(path)
	}))
	// Bound multipart parsing before it can allocate memory or spill large uploads to disk.
	router.Use(func(c *gin.Context) {
		if (c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPatch) && ((strings.HasPrefix(c.Request.URL.Path, apiPrefix+"/cash-sessions/") && strings.HasSuffix(c.Request.URL.Path, "/movements")) || c.Request.URL.Path == apiPrefix+"/users/me" || c.Request.URL.Path == apiPrefix+"/products" || strings.HasPrefix(c.Request.URL.Path, apiPrefix+"/products/")) {
			const maxProductBody = int64(4 * 1024 * 1024)
			if c.Request.ContentLength > maxProductBody {
				sharedresponse.WriteError(c.Writer, sharederrors.New(http.StatusRequestEntityTooLarge, "PERMINTAAN_TERLALU_BESAR", "Ukuran permintaan maksimal 4 MiB, termasuk gambar maksimal 3 MiB.", nil))
				c.Abort()
				return
			}
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxProductBody)
		}
		c.Next()
	})
	router.NoRoute(func(c *gin.Context) {
		sharedresponse.WriteError(c.Writer, sharederrors.Standard(http.StatusNotFound, nil))
	})
	router.NoMethod(func(c *gin.Context) {
		sharedresponse.WriteError(c.Writer, sharederrors.Standard(http.StatusMethodNotAllowed, nil))
	})

	configuration := huma.DefaultConfig("API Dikasa", "1.0.0")
	configuration.OpenAPIPath = "/openapi"
	configuration.DocsPath = "/docs"
	configuration.SchemasPath = "/schemas"
	configuration.DocsRenderer = huma.DocsRendererSwaggerUI
	configuration.Transformers = nil
	configuration.CreateHooks = nil
	configuration.Servers = []*huma.Server{{URL: apiPrefix, Description: "API Dikasa"}}
	configuration.Security = []map[string][]string{{"firebaseBearer": {}}}
	configuration.Components.SecuritySchemes = map[string]*huma.SecurityScheme{
		"firebaseBearer": {
			Type: "http", Scheme: "bearer", BearerFormat: "JWT",
			Description: "Gunakan Firebase ID token pada access_token dari /login.",
		},
	}
	configuration.RejectUnknownQueryParameters = true

	group := router.Group(apiPrefix)
	api := humagin.NewWithGroup(router, group, configuration)
	if dependencies.StockHandler != nil {
		catalog.RegisterStockRoutes(api, dependencies.StockHandler)
	}
	if dependencies.ExportHandler != nil {
		catalog.RegisterExportRoutes(api, dependencies.ExportHandler)
	}
	if dependencies.SavedOrderHandler != nil {
		savedorder.RegisterRoutes(api, dependencies.SavedOrderHandler)
	}
	if dependencies.RefundHandler != nil {
		refund.RegisterRoutes(api, dependencies.RefundHandler)
	}
	if dependencies.ReportsHandler != nil {
		reports.RegisterRoutes(api, dependencies.ReportsHandler)
	}
	auth.RegisterRoutes(api, dependencies.AuthHandler)
	cash.RegisterRoutes(api, dependencies.CashHandler)
	if dependencies.MovementHandler != nil {
		cash.RegisterMovementRoutes(api, dependencies.MovementHandler)
	}
	if dependencies.CashClosingHandler != nil {
		cash.RegisterReconciliationRoutes(api, dependencies.CashClosingHandler)
	}
	if dependencies.CatalogHandler != nil {
		catalog.RegisterRoutes(api, dependencies.CatalogHandler)
	}
	if dependencies.PaymentHandler != nil {
		payment.RegisterRoutes(api, dependencies.PaymentHandler)
	}
	if dependencies.ReservationHandler != nil {
		reservation.RegisterRoutes(api, dependencies.ReservationHandler)
	}
	if dependencies.SalesHandler != nil {
		sales.RegisterRoutes(api, dependencies.SalesHandler)
	}
	registration.RegisterRoutes(api, dependencies.RegistrationHandler)
	user.RegisterRoutes(api, dependencies.UserHandler)
	registerSystemRoutes(api)
	localizeOpenAPIResponses(api.OpenAPI())
	return router
}

func registerSystemRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-health",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Memeriksa kondisi API",
		Description: "Memastikan API berjalan dan Bearer token dari auth provider berhasil diverifikasi.",
		Tags:        []string{"Sistem"},
		Errors:      []int{http.StatusUnauthorized, http.StatusInternalServerError},
	}, health)
}

type healthOutput struct {
	Body struct {
		Status  string `json:"status" example:"ok"`
		Service string `json:"service" example:"be-dikasa"`
	}
}

func health(_ context.Context, _ *struct{}) (*healthOutput, error) {
	output := &healthOutput{}
	output.Body.Status = "ok"
	output.Body.Service = "be-dikasa"
	return output, nil
}

func isPublicPath(path string) bool {
	switch path {
	case apiPrefix + "/login",
		apiPrefix + "/register",
		apiPrefix + "/register/otp/confirm",
		apiPrefix + "/register/otp/resend",
		apiPrefix + "/docs",
		apiPrefix + "/docs/",
		apiPrefix + "/openapi.json",
		apiPrefix + "/openapi.yaml":
		return true
	default:
		return strings.HasPrefix(path, apiPrefix+"/schemas/")
	}
}

func configureHumaErrors() {
	newError := func(status int, _ string, errs ...error) huma.StatusError {
		details := humaErrorDetails(errs)
		if status == http.StatusUnprocessableEntity {
			return sharederrors.New(status, "VALIDASI_GAGAL", "Data yang dikirim tidak memenuhi ketentuan.", details)
		}
		return sharederrors.Standard(status, details)
	}
	huma.NewError = newError
	huma.NewErrorWithContext = func(_ huma.Context, status int, message string, errs ...error) huma.StatusError {
		return newError(status, message, errs...)
	}
}

func humaErrorDetails(errs []error) map[string]string {
	details := make(map[string]string)
	for _, err := range errs {
		if err == nil {
			continue
		}
		var detailer huma.ErrorDetailer
		if !errors.As(err, &detailer) {
			continue
		}
		detail := detailer.ErrorDetail()
		field := strings.TrimPrefix(detail.Location, "body.")
		if field == "body" || field == "" {
			field = "request"
		}
		details[field] = translateHumaMessage(detail.Message)
	}
	if len(details) == 0 {
		return nil
	}
	return details
}

func translateHumaMessage(message string) string {
	switch {
	case strings.Contains(message, "unexpected property"):
		return "Field tidak dikenal."
	case strings.Contains(message, "required property"):
		return "Field wajib diisi."
	case strings.Contains(message, "expected string length"):
		return "Panjang teks tidak memenuhi ketentuan."
	case strings.Contains(message, "expected string to match"):
		return "Format nilai tidak valid."
	case strings.Contains(message, "expected string"):
		return "Nilai harus berupa teks."
	default:
		return "Nilai tidak valid."
	}
}

func localizeOpenAPIResponses(openAPI *huma.OpenAPI) {
	descriptions := map[string]string{
		"400": "Permintaan tidak valid.", "401": "Autentikasi diperlukan.",
		"403": "Akses ditolak.", "404": "Data tidak ditemukan.",
		"405": "Metode HTTP tidak diizinkan.", "409": "Terjadi konflik data.",
		"410": "Data sudah kedaluwarsa.",
		"413": "Ukuran payload terlalu besar.",
		"415": "Tipe konten tidak didukung.", "422": "Validasi data gagal.",
		"429": "Terlalu banyak permintaan.", "500": "Terjadi kesalahan pada server.",
		"502": "Layanan eksternal bermasalah.", "503": "Layanan sementara tidak tersedia.",
		"504": "Layanan eksternal tidak merespons tepat waktu.",
	}
	for _, path := range openAPI.Paths {
		operations := []*huma.Operation{path.Get, path.Post, path.Put, path.Patch, path.Delete, path.Head, path.Options, path.Trace}
		for _, operation := range operations {
			if operation == nil {
				continue
			}
			for status, description := range descriptions {
				if response := operation.Responses[status]; response != nil {
					response.Description = description
				}
			}
		}
	}
}
