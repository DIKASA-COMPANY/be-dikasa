package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validConfig() map[string]string {
	return map[string]string{
		"DATABASE_URL": "postgres://localhost/dikasa", "SUPABASE_URL": "https://project.supabase.co",
		"SUPABASE_PUBLISHABLE_KEY": "publishable-key", "SUPABASE_SECRET_KEY": "secret-key", "SUPABASE_STORAGE_BUCKET": "dikasa-images",
		"OTP_SECRET": "0123456789abcdef0123456789abcdef", "SMTP_HOST": "smtp.example.com", "SMTP_PORT": "587", "SMTP_FROM": "Dikasa <mailer@example.com>",
	}
}

func TestLoadRejectsNonSupabaseAuthAndStorageModes(t *testing.T) {
	tests := []struct{ key, value, want string }{
		{key: "AUTH_MODE", value: "local", want: "AUTH_MODE=supabase"},
		{key: "IMAGE_STORAGE_MODE", value: "local", want: "IMAGE_STORAGE_MODE=supabase"},
		{key: "IMAGE_STORAGE_MODE", value: "disabled", want: "IMAGE_STORAGE_MODE=supabase"},
	}
	for _, test := range tests {
		values := validConfig()
		values[test.key] = test.value
		if _, err := Load(func(key string) string { return values[key] }); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("expected %q rejection containing %q, got %v", test.value, test.want, err)
		}
	}
}

func TestLoadRequiresAllSupabaseConfiguration(t *testing.T) {
	for _, key := range []string{"SUPABASE_URL", "SUPABASE_PUBLISHABLE_KEY", "SUPABASE_SECRET_KEY", "SUPABASE_STORAGE_BUCKET"} {
		values := validConfig()
		delete(values, key)
		if _, err := Load(func(name string) string { return values[name] }); err == nil || !strings.Contains(err.Error(), key) {
			t.Fatalf("expected missing %s error, got %v", key, err)
		}
	}
}

func TestLoadReturnsTypedConfigurationAndDefaultPort(t *testing.T) {
	values := validConfig()
	values["SMTP_USERNAME"] = "mailer@example.com"
	values["SMTP_PASSWORD"] = "smtp-secret"
	configuration, err := Load(func(key string) string { return values[key] })
	if err != nil {
		t.Fatal(err)
	}
	if configuration.Port != "8080" || configuration.Supabase.URL != values["SUPABASE_URL"] || configuration.Supabase.StorageBucket != "dikasa-images" || configuration.Mail.Host != "smtp.example.com" {
		t.Fatalf("unexpected config: %+v", configuration)
	}
}

func TestLoadRequiresApplicationOTPAndSMTPConfiguration(t *testing.T) {
	if _, err := Load(func(string) string { return "" }); err == nil {
		t.Fatal("expected missing configuration error")
	}
	values := validConfig()
	delete(values, "OTP_SECRET")
	delete(values, "SMTP_HOST")
	if _, err := Load(func(key string) string { return values[key] }); err == nil {
		t.Fatal("expected missing OTP and SMTP configuration error")
	}
}

func TestLoadDotEnvDoesNotOverrideExistingEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("DIKASA_DOTENV_TEST=file-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DIKASA_DOTENV_TEST", "environment-value")
	if err := LoadDotEnv(path); err != nil {
		t.Fatal(err)
	}
	if actual := os.Getenv("DIKASA_DOTENV_TEST"); actual != "environment-value" {
		t.Fatalf("dotenv must not override environment, got %q", actual)
	}
}
