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
	"be-dikasa/internal/registration"
	"be-dikasa/internal/reservation"
	sharederrors "be-dikasa/internal/shared/errors"
	sharedresponse "be-dikasa/internal/shared/response"
	"be-dikasa/internal/user"
)

const apiPrefix = "/dikasa/v1"

// Dependencies berisi handler feature dan layanan token yang dibutuhkan router.
type Dependencies struct {
	AuthHandler         *auth.Handler
	CashHandler         *cash.Handler
	CatalogHandler      *catalog.Handler
	RegistrationHandler *registration.Handler
	ReservationHandler  *reservation.Handler
	UserHandler         *user.Handler
	TokenVerifier       TokenVerifier
}

// NewRouter membuat Gin engine, memasang Firebase bearer auth, dan mendaftarkan API Huma.
func NewRouter(dependencies Dependencies) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	configureHumaErrors()

	router := gin.New()
	router.HandleMethodNotAllowed = true
	router.Use(gin.CustomRecovery(func(c *gin.Context, _ any) {
		sharedresponse.WriteError(c.Writer, sharederrors.Standard(http.StatusInternalServerError, nil))
		c.Abort()
	}))
	router.Use(AuthMiddleware(dependencies.TokenVerifier, isPublicPath))
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
			Type: "http", Scheme: "bearer", BearerFormat: "Firebase ID token",
			Description: "Gunakan access_token Firebase dari endpoint /login.",
		},
	}
	configuration.RejectUnknownQueryParameters = true

	group := router.Group(apiPrefix)
	api := humagin.NewWithGroup(router, group, configuration)
	auth.RegisterRoutes(api, dependencies.AuthHandler)
	cash.RegisterRoutes(api, dependencies.CashHandler)
	if dependencies.CatalogHandler != nil {
		catalog.RegisterRoutes(api, dependencies.CatalogHandler)
	}
	if dependencies.ReservationHandler != nil {
		reservation.RegisterRoutes(api, dependencies.ReservationHandler)
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
		Description: "Memastikan API berjalan dan Firebase ID token berhasil diverifikasi.",
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
	if path == apiPrefix+"/login" || path == apiPrefix+"/register" || strings.HasPrefix(path, apiPrefix+"/register/otp/") || path == apiPrefix+"/docs" || path == apiPrefix+"/docs/" {
		return true
	}
	return strings.HasPrefix(path, apiPrefix+"/openapi") || strings.HasPrefix(path, apiPrefix+"/schemas/")
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
