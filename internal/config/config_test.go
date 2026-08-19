package config

import (
	"os"
	"path/filepath"
	"testing"
)

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
		"IMAGE_STORAGE_MODE":   "disabled",
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
	if configuration.Port != "9090" || configuration.Firebase.ProjectID != "dikasa-dev" || configuration.Firebase.StorageMode != "disabled" || configuration.Mail.Host != "smtp.example.com" || configuration.OTPSecret == "" {
		t.Fatalf("unexpected config: %+v", configuration)
	}
}

func TestLoadRequiresFirebaseBucketOnlyWhenStorageEnabled(t *testing.T) {
	values := map[string]string{
		"DATABASE_URL":         "postgres://localhost/dikasa",
		"FIREBASE_PROJECT_ID":  "dikasa-dev",
		"FIREBASE_WEB_API_KEY": "api-key",
		"IMAGE_STORAGE_MODE":   "firebase",
		"OTP_SECRET":           "0123456789abcdef0123456789abcdef",
		"SMTP_HOST":            "smtp.example.com",
		"SMTP_PORT":            "587",
		"SMTP_FROM":            "mailer@example.com",
	}

	if _, err := Load(func(key string) string { return values[key] }); err == nil {
		t.Fatal("expected missing FIREBASE_STORAGE_BUCKET error")
	}
	values["FIREBASE_STORAGE_BUCKET"] = "dikasa-dev.firebasestorage.app"
	configuration, err := Load(func(key string) string { return values[key] })
	if err != nil || configuration.Firebase.StorageBucket != "dikasa-dev.firebasestorage.app" {
		t.Fatalf("expected firebase storage config, got %+v, %v", configuration, err)
	}
}

func TestLoadDotEnvDoesNotOverrideExistingEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("DIKASA_DOTENV_TEST=file-value\n"), 0o600); err != nil {
		t.Fatalf("write test env: %v", err)
	}
	t.Setenv("DIKASA_DOTENV_TEST", "environment-value")

	if err := LoadDotEnv(path); err != nil {
		t.Fatalf("load dotenv: %v", err)
	}
	if actual := os.Getenv("DIKASA_DOTENV_TEST"); actual != "environment-value" {
		t.Fatalf("dotenv must not override environment, got %q", actual)
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
