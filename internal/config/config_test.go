package config

import "testing"

func TestLoadRequiresApplicationConfiguration(t *testing.T) {
	_, err := Load(func(string) string { return "" })
	if err == nil {
		t.Fatal("expected missing configuration error")
	}
}

func TestLoadReturnsTypedConfiguration(t *testing.T) {
	values := map[string]string{
		"PORT":                 "9090",
		"DATABASE_URL":         "postgres://localhost/dikasa",
		"FIREBASE_PROJECT_ID":  "dikasa-dev",
		"FIREBASE_WEB_API_KEY": "api-key",
		"OTP_SECRET":           "0123456789abcdef0123456789abcdef",
		"SMTP_HOST":            "smtp.example.com",
		"SMTP_PORT":            "587",
		"SMTP_USERNAME":        "mailer@example.com",
		"SMTP_PASSWORD":        "smtp-secret",
		"SMTP_FROM":            "Dikasa <mailer@example.com>",
	}

	configuration, err := Load(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if configuration.Port != "9090" || configuration.Firebase.ProjectID != "dikasa-dev" || configuration.Mail.Host != "smtp.example.com" || configuration.OTPSecret == "" {
		t.Fatalf("unexpected config: %+v", configuration)
	}
}

func TestLoadUsesDefaultPort(t *testing.T) {
	values := map[string]string{
		"DATABASE_URL":         "postgres://localhost/dikasa",
		"FIREBASE_PROJECT_ID":  "dikasa-dev",
		"FIREBASE_WEB_API_KEY": "api-key",
		"OTP_SECRET":           "0123456789abcdef0123456789abcdef",
		"SMTP_HOST":            "smtp.example.com",
		"SMTP_PORT":            "587",
		"SMTP_FROM":            "mailer@example.com",
	}

	configuration, err := Load(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if configuration.Port != "8080" {
		t.Fatalf("expected default port 8080, got %s", configuration.Port)
	}
}

func TestLoadRequiresOTPAndSMTPConfiguration(t *testing.T) {
	values := map[string]string{
		"DATABASE_URL":         "postgres://localhost/dikasa",
		"FIREBASE_PROJECT_ID":  "dikasa-dev",
		"FIREBASE_WEB_API_KEY": "api-key",
	}

	_, err := Load(func(key string) string { return values[key] })

	if err == nil {
		t.Fatal("expected missing OTP and SMTP configuration error")
	}
}
