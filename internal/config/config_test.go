package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear env
	os.Unsetenv("PORT")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("API_KEY")
	os.Unsetenv("BCRYPT_COST")

	cfg := Load()
	if cfg.Port != "8080" {
		t.Errorf("expected default port 8080, got %s", cfg.Port)
	}
	if cfg.BcryptCost != 12 {
		t.Errorf("expected default bcrypt cost 12, got %d", cfg.BcryptCost)
	}
	if cfg.DatabaseURL != "" {
		t.Errorf("expected empty DATABASE_URL, got %s", cfg.DatabaseURL)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	os.Setenv("PORT", "9090")
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("JWT_SECRET", "mysecret")
	os.Setenv("API_KEY", "ak_test123")
	os.Setenv("BCRYPT_COST", "8")
	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("API_KEY")
		os.Unsetenv("BCRYPT_COST")
	}()

	cfg := Load()
	if cfg.Port != "9090" {
		t.Errorf("expected port 9090, got %s", cfg.Port)
	}
	if cfg.DatabaseURL != "postgres://localhost/test" {
		t.Errorf("expected DATABASE_URL, got %s", cfg.DatabaseURL)
	}
	if cfg.JWTSecret != "mysecret" {
		t.Errorf("expected JWT_SECRET, got %s", cfg.JWTSecret)
	}
	if cfg.APIKey != "ak_test123" {
		t.Errorf("expected API_KEY, got %s", cfg.APIKey)
	}
	if cfg.BcryptCost != 8 {
		t.Errorf("expected bcrypt cost 8, got %d", cfg.BcryptCost)
	}
}

func TestGetenv(t *testing.T) {
	os.Setenv("TEST_GETENV_KEY", "value")
	defer os.Unsetenv("TEST_GETENV_KEY")

	if v := getenv("TEST_GETENV_KEY", "fallback"); v != "value" {
		t.Errorf("expected 'value', got '%s'", v)
	}
	if v := getenv("NONEXISTENT_KEY", "fallback"); v != "fallback" {
		t.Errorf("expected 'fallback', got '%s'", v)
	}
}

func TestAtoi(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"12", 12},
		{"0", 12},  // zero falls back to default
		{"abc", 12}, // non-numeric falls back
		{"16", 16},
		{"", 12},
	}
	for _, tt := range tests {
		if got := atoi(tt.input); got != tt.expected {
			t.Errorf("atoi(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}
