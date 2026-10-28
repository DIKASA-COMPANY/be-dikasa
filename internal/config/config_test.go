package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRejectsLegacyLocalAuthAndStorageModes(t *testing.T) {
	base := map[string]string{
		"DATABASE_URL": "postgres://localhost/dikasa", "FIREBASE_PROJECT_ID": "dikasa-dev",
		"FIREBASE_WEB_API_KEY": "api-key", "FIREBASE_STORAGE_BUCKET": "dikasa-dev.firebasestorage.app",
		"OTP_SECRET": "0123456789abcdef0123456789abcdef", "SMTP_HOST": "smtp.example.com",
		"SMTP_PORT": "587", "SMTP_FROM": "Dikasa <mailer@example.com>",
	}
	tests := []struct {
		name, key, value, want string
	}{
		{name: "local auth", key: "AUTH_MODE", value: "local", want: "AUTH_MODE=firebase"},
		{name: "local storage", key: "IMAGE_STORAGE_MODE", value: "local", want: "IMAGE_STORAGE_MODE=firebase"},
		{name: "disabled storage", key: "IMAGE_STORAGE_MODE", value: "disabled", want: "IMAGE_STORAGE_MODE=firebase"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			values := make(map[string]string, len(base)+1)
			for key, value := range base {
				values[key] = value
			}
			values[test.key] = test.value
			_, err := Load(func(key string) string { return values[key] })
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q rejection containing %q, got %v", test.value, test.want, err)
			}
		})
	}
}

func TestLoadRequiresFirebaseStorageBucketByDefault(t *testing.T) {
	values := map[string]string{
		"DATABASE_URL": "postgres://localhost/dikasa", "FIREBASE_PROJECT_ID": "dikasa-dev",
		"FIREBASE_WEB_API_KEY": "api-key", "OTP_SECRET": "0123456789abcdef0123456789abcdef",
		"SMTP_HOST": "smtp.example.com", "SMTP_PORT": "587", "SMTP_FROM": "Dikasa <mailer@example.com>",
	}
	_, err := Load(func(key string) string { return values[key] })
	if err == nil || !strings.Contains(err.Error(), "FIREBASE_STORAGE_BUCKET") {
		t.Fatalf("expected required Firebase bucket, got %v", err)
	}
}

func TestLoadRequiresApplicationConfiguration(t *testing.T) {
	_, err := Load(func(string) string { return "" })
	if err == nil {
		t.Fatal("expected missing configuration error")
	}
}

func TestLoadReturnsTypedConfiguration(t *testing.T) {
	values := map[string]string{
		"PORT":                    "9090",
		"DATABASE_URL":            "postgres://localhost/dikasa",
		"FIREBASE_PROJECT_ID":     "dikasa-dev",
		"FIREBASE_WEB_API_KEY":    "api-key",
		"FIREBASE_STORAGE_BUCKET": "dikasa-dev.firebasestorage.app",
		"OTP_SECRET":              "0123456789abcdef0123456789abcdef",
		"SMTP_HOST":               "smtp.example.com",
		"SMTP_PORT":               "587",
		"SMTP_USERNAME":           "mailer@example.com",
		"SMTP_PASSWORD":           "smtp-secret",
		"SMTP_FROM":               "Dikasa <mailer@example.com>",
	}

	configuration, err := Load(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if configuration.Port != "9090" || configuration.Firebase.ProjectID != "dikasa-dev" || configuration.Firebase.StorageBucket != "dikasa-dev.firebasestorage.app" || configuration.Mail.Host != "smtp.example.com" || configuration.OTPSecret == "" {
		t.Fatalf("unexpected config: %+v", configuration)
	}
}

func TestLoadRequiresFirebaseBucket(t *testing.T) {
	values := map[string]string{
		"DATABASE_URL":         "postgres://localhost/dikasa",
		"FIREBASE_PROJECT_ID":  "dikasa-dev",
		"FIREBASE_WEB_API_KEY": "api-key",
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
		"DATABASE_URL":            "postgres://localhost/dikasa",
		"FIREBASE_PROJECT_ID":     "dikasa-dev",
		"FIREBASE_WEB_API_KEY":    "api-key",
		"FIREBASE_STORAGE_BUCKET": "dikasa-dev.firebasestorage.app",
		"OTP_SECRET":              "0123456789abcdef0123456789abcdef",
		"SMTP_HOST":               "smtp.example.com",
		"SMTP_PORT":               "587",
		"SMTP_FROM":               "mailer@example.com",
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
		"DATABASE_URL":            "postgres://localhost/dikasa",
		"FIREBASE_PROJECT_ID":     "dikasa-dev",
		"FIREBASE_WEB_API_KEY":    "api-key",
		"FIREBASE_STORAGE_BUCKET": "dikasa-dev.firebasestorage.app",
	}

	_, err := Load(func(key string) string { return values[key] })

	if err == nil {
		t.Fatal("expected missing OTP and SMTP configuration error")
	}
}
