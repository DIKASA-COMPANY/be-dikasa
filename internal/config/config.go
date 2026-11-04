// Package config memuat konfigurasi aplikasi dari environment variable.
package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Lookup mendapatkan nilai environment berdasarkan nama variable.
type Lookup func(key string) string

// Supabase memuat konfigurasi Supabase Auth dan Storage.
type Supabase struct {
	URL            string
	PublishableKey string
	SecretKey      string
	StorageBucket  string
}

// Mail memuat konfigurasi SMTP untuk pengiriman OTP.
type Mail struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// Config adalah konfigurasi runtime backend Dikasa.
type Config struct {
	CORSOrigins []string
	Port        string
	DatabaseURL string
	Supabase    Supabase
	OTPSecret   string
	Mail        Mail
}

// Load membaca dan memvalidasi konfigurasi aplikasi.
func Load(lookup Lookup) (Config, error) {
	configuration := Config{
		Port:        strings.TrimSpace(lookup("PORT")),
		DatabaseURL: strings.TrimSpace(lookup("DATABASE_URL")),
		Supabase: Supabase{
			URL:            strings.TrimSpace(lookup("SUPABASE_URL")),
			PublishableKey: strings.TrimSpace(lookup("SUPABASE_PUBLISHABLE_KEY")),
			SecretKey:      strings.TrimSpace(lookup("SUPABASE_SECRET_KEY")),
			StorageBucket:  strings.TrimSpace(lookup("SUPABASE_STORAGE_BUCKET")),
		},
		OTPSecret: strings.TrimSpace(lookup("OTP_SECRET")),
		Mail: Mail{
			Host: strings.TrimSpace(lookup("SMTP_HOST")), Username: strings.TrimSpace(lookup("SMTP_USERNAME")),
			Password: lookup("SMTP_PASSWORD"), From: strings.TrimSpace(lookup("SMTP_FROM")),
		},
	}
	if mode := strings.ToLower(strings.TrimSpace(lookup("AUTH_MODE"))); mode != "" && mode != "supabase" {
		return Config{}, fmt.Errorf("AUTH_MODE=supabase is required; local authentication is not supported")
	}
	if mode := strings.ToLower(strings.TrimSpace(lookup("IMAGE_STORAGE_MODE"))); mode != "" && mode != "supabase" {
		return Config{}, fmt.Errorf("IMAGE_STORAGE_MODE=supabase is required; local or disabled image storage is not supported")
	}
	if mode := strings.ToLower(strings.TrimSpace(lookup("MAIL_MODE"))); mode != "" && mode != "smtp" {
		return Config{}, fmt.Errorf("MAIL_MODE=smtp is required")
	}
	origins := strings.TrimSpace(lookup("CORS_ALLOWED_ORIGINS"))
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
	missing := make([]string, 0, 8)
	if configuration.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if configuration.Supabase.URL == "" {
		missing = append(missing, "SUPABASE_URL")
	}
	if configuration.Supabase.PublishableKey == "" {
		missing = append(missing, "SUPABASE_PUBLISHABLE_KEY")
	}
	if configuration.Supabase.SecretKey == "" {
		missing = append(missing, "SUPABASE_SECRET_KEY")
	}
	if configuration.Supabase.StorageBucket == "" {
		missing = append(missing, "SUPABASE_STORAGE_BUCKET")
	}
	if len(configuration.OTPSecret) < 32 {
		missing = append(missing, "OTP_SECRET (minimal 32 karakter)")
	}
	if configuration.Mail.Host == "" {
		missing = append(missing, "SMTP_HOST")
	}
	if strings.TrimSpace(lookup("SMTP_PORT")) == "" {
		missing = append(missing, "SMTP_PORT")
	}
	if configuration.Mail.From == "" {
		missing = append(missing, "SMTP_FROM")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("konfigurasi wajib belum diatur: %s", strings.Join(missing, ", "))
	}
	port, err := strconv.Atoi(configuration.Port)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("PORT tidak valid: %q", configuration.Port)
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
