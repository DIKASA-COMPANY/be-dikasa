// Package config memuat konfigurasi aplikasi dari environment variable.
package config

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

const (
	// ImageStorageDisabled keeps image uploads unavailable while allowing image-less products.
	ImageStorageDisabled = "disabled"
	// ImageStorageLocal stores product photos on the local filesystem.
	ImageStorageLocal = "local"
	// ImageStorageFirebase stores validated product images in Firebase Storage.
	ImageStorageFirebase = "firebase"
)

// Lookup mendapatkan nilai environment berdasarkan nama variable.
type Lookup func(key string) string

// Firebase memuat konfigurasi Firebase Authentication.
type Firebase struct {
	ProjectID     string
	WebAPIKey     string
	StorageMode   string
	StorageBucket string
}

// Mail memuat konfigurasi SMTP untuk pengiriman OTP.
type Mail struct {
	Mode     string
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// Config adalah konfigurasi runtime backend Dikasa.
type Config struct {
	AuthMode            string
	LocalAuthSecret     string
	CORSOrigins         []string
	Port                string
	LocalImageDirectory string
	CashProofDirectory  string
	PublicBaseURL       string
	DatabaseURL         string
	Firebase            Firebase
	OTPSecret           string
	Mail                Mail
}

// Load membaca dan memvalidasi konfigurasi aplikasi.
func Load(lookup Lookup) (Config, error) {
	configuration := Config{
		CashProofDirectory:  strings.TrimSpace(lookup("CASH_PROOF_DIRECTORY")),
		AuthMode:            strings.ToLower(strings.TrimSpace(lookup("AUTH_MODE"))),
		LocalAuthSecret:     strings.TrimSpace(lookup("LOCAL_AUTH_SECRET")),
		Port:                strings.TrimSpace(lookup("PORT")),
		DatabaseURL:         strings.TrimSpace(lookup("DATABASE_URL")),
		LocalImageDirectory: strings.TrimSpace(lookup("LOCAL_IMAGE_DIRECTORY")),
		PublicBaseURL:       strings.TrimRight(strings.TrimSpace(lookup("PUBLIC_BASE_URL")), "/"),
		Firebase: Firebase{
			ProjectID:     strings.TrimSpace(lookup("FIREBASE_PROJECT_ID")),
			WebAPIKey:     strings.TrimSpace(lookup("FIREBASE_WEB_API_KEY")),
			StorageMode:   strings.ToLower(strings.TrimSpace(lookup("IMAGE_STORAGE_MODE"))),
			StorageBucket: strings.TrimSpace(lookup("FIREBASE_STORAGE_BUCKET")),
		},
		OTPSecret: strings.TrimSpace(lookup("OTP_SECRET")),
		Mail: Mail{
			Mode: strings.ToLower(strings.TrimSpace(lookup("MAIL_MODE"))),
			Host: strings.TrimSpace(lookup("SMTP_HOST")), Username: strings.TrimSpace(lookup("SMTP_USERNAME")),
			Password: lookup("SMTP_PASSWORD"), From: strings.TrimSpace(lookup("SMTP_FROM")),
		},
	}
	if configuration.AuthMode == "" {
		configuration.AuthMode = "firebase"
	}
	if configuration.AuthMode != "firebase" && configuration.AuthMode != "local" {
		return Config{}, fmt.Errorf("AUTH_MODE must be firebase or local")
	}
	if configuration.Mail.Mode == "" {
		configuration.Mail.Mode = "smtp"
		if configuration.AuthMode == "local" {
			configuration.Mail.Mode = "disabled"
		}
	}
	if configuration.Mail.Mode != "smtp" && configuration.Mail.Mode != "disabled" {
		return Config{}, fmt.Errorf("MAIL_MODE must be smtp or disabled")
	}
	if configuration.Mail.Mode == "disabled" && configuration.AuthMode != "local" {
		return Config{}, fmt.Errorf("MAIL_MODE=disabled is only supported with AUTH_MODE=local")
	}
	origins := strings.TrimSpace(lookup("CORS_ALLOWED_ORIGINS"))
	if origins == "" && configuration.AuthMode == "local" {
		origins = "http://localhost:8080,http://127.0.0.1:8080"
	}
	if origins != "" {
		for _, origin := range strings.Split(origins, ",") {
			origin = strings.TrimSpace(origin)
			parsed, err := url.Parse(origin)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || strings.Contains(parsed.Host, "*") || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
				return Config{}, fmt.Errorf("invalid CORS origin: %q", origin)
			}
			configuration.CORSOrigins = append(configuration.CORSOrigins, origin)
		}
	}
	if configuration.Port == "" {
		configuration.Port = "8080"
	}
	if configuration.Firebase.StorageMode == "" {
		configuration.Firebase.StorageMode = ImageStorageDisabled
	}

	missing := make([]string, 0, 7)
	if configuration.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if configuration.AuthMode == "firebase" && configuration.Firebase.ProjectID == "" {
		missing = append(missing, "FIREBASE_PROJECT_ID")
	}
	if configuration.AuthMode == "firebase" && configuration.Firebase.WebAPIKey == "" {
		missing = append(missing, "FIREBASE_WEB_API_KEY")
	}
	if configuration.Firebase.StorageMode != ImageStorageDisabled && configuration.Firebase.StorageMode != ImageStorageFirebase && configuration.Firebase.StorageMode != ImageStorageLocal {
		return Config{}, fmt.Errorf("IMAGE_STORAGE_MODE must be disabled, local or firebase")
	}
	if configuration.Firebase.StorageMode == ImageStorageLocal {
		parsed, err := url.Parse(configuration.PublicBaseURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return Config{}, fmt.Errorf("PUBLIC_BASE_URL must be an http(s) origin")
		}
		if configuration.LocalImageDirectory == "" {
			return Config{}, fmt.Errorf("LOCAL_IMAGE_DIRECTORY is required for local images")
		}
	}
	if configuration.Firebase.StorageMode == ImageStorageFirebase && configuration.Firebase.StorageBucket == "" {
		missing = append(missing, "FIREBASE_STORAGE_BUCKET")
	}
	if configuration.AuthMode == "local" && len(configuration.LocalAuthSecret) < 32 {
		missing = append(missing, "LOCAL_AUTH_SECRET (at least 32 bytes)")
	}
	if configuration.Mail.Mode == "smtp" && len(configuration.OTPSecret) < 32 {
		missing = append(missing, "OTP_SECRET (minimal 32 karakter)")
	}
	if configuration.Mail.Mode == "smtp" && configuration.Mail.Host == "" {
		missing = append(missing, "SMTP_HOST")
	}
	if configuration.Mail.Mode == "smtp" && strings.TrimSpace(lookup("SMTP_PORT")) == "" {
		missing = append(missing, "SMTP_PORT")
	}
	if configuration.Mail.Mode == "smtp" && configuration.Mail.From == "" {
		missing = append(missing, "SMTP_FROM")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("konfigurasi wajib belum diatur: %s", strings.Join(missing, ", "))
	}
	port, err := strconv.Atoi(configuration.Port)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("PORT tidak valid: %q", configuration.Port)
	}
	if configuration.CashProofDirectory == "" && configuration.Firebase.StorageMode == ImageStorageLocal {
		configuration.CashProofDirectory = filepath.Join(configuration.LocalImageDirectory, "cash-proofs")
	}
	if configuration.Mail.Mode == "disabled" {
		return configuration, nil
	}
	smtpPort, err := strconv.Atoi(strings.TrimSpace(lookup("SMTP_PORT")))
	if err != nil || smtpPort < 1 || smtpPort > 65535 {
		return Config{}, fmt.Errorf("SMTP_PORT tidak valid: %q", lookup("SMTP_PORT"))
	}
	configuration.Mail.Port = smtpPort

	return configuration, nil
}

// LoadDotEnv loads development variables without replacing values already present in the process environment.
func LoadDotEnv(path string) error {
	return godotenv.Load(path)
}
