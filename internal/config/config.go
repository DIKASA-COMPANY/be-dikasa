// Package config memuat konfigurasi aplikasi dari environment variable.
package config

import (
	"fmt"
	"strconv"
	"strings"
)

// Lookup mendapatkan nilai environment berdasarkan nama variable.
type Lookup func(key string) string

// Firebase memuat konfigurasi Firebase Authentication.
type Firebase struct {
	ProjectID string
	WebAPIKey string
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
	Port        string
	DatabaseURL string
	Firebase    Firebase
	OTPSecret   string
	Mail        Mail
}

// Load membaca dan memvalidasi konfigurasi aplikasi.
func Load(lookup Lookup) (Config, error) {
	configuration := Config{
		Port:        strings.TrimSpace(lookup("PORT")),
		DatabaseURL: strings.TrimSpace(lookup("DATABASE_URL")),
		Firebase: Firebase{
			ProjectID: strings.TrimSpace(lookup("FIREBASE_PROJECT_ID")),
			WebAPIKey: strings.TrimSpace(lookup("FIREBASE_WEB_API_KEY")),
		},
		OTPSecret: strings.TrimSpace(lookup("OTP_SECRET")),
		Mail: Mail{
			Host: strings.TrimSpace(lookup("SMTP_HOST")), Username: strings.TrimSpace(lookup("SMTP_USERNAME")),
			Password: lookup("SMTP_PASSWORD"), From: strings.TrimSpace(lookup("SMTP_FROM")),
		},
	}
	if configuration.Port == "" {
		configuration.Port = "8080"
	}

	missing := make([]string, 0, 7)
	if configuration.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if configuration.Firebase.ProjectID == "" {
		missing = append(missing, "FIREBASE_PROJECT_ID")
	}
	if configuration.Firebase.WebAPIKey == "" {
		missing = append(missing, "FIREBASE_WEB_API_KEY")
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
