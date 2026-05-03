package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("PORTFLARE_AUTH_LISTEN_ADDR", "")
	t.Setenv("PORTFLARE_AUTH_PUBLIC_URL", "")
	t.Setenv("PORTFLARE_AUTH_SHARED_SECRET", "")
	t.Setenv("PORTFLARE_AUTH_GOOGLE_CLIENT_ID", "")
	t.Setenv("PORTFLARE_AUTH_GOOGLE_CLIENT_SECRET", "")
	t.Setenv("PORTFLARE_AUTH_GOOGLE_REDIRECT_URL", "")
	t.Setenv("PORTFLARE_AUTH_CODE_TTL", "")
	t.Setenv("PORTFLARE_AUTH_TX_TTL", "")
	t.Setenv("PORTFLARE_AUTH_BACKEND_SKEW", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ListenAddr != ":8081" {
		t.Fatalf("unexpected listen addr: %q", cfg.ListenAddr)
	}
	if cfg.CodeTTL != 5*time.Minute || cfg.TransactionTTL != 5*time.Minute || cfg.BackendSkew != time.Minute {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
	if cfg.GoogleEnabled() {
		t.Fatal("google should be disabled without credentials")
	}
}

func TestLoadConfiguredValues(t *testing.T) {
	t.Setenv("PORTFLARE_AUTH_LISTEN_ADDR", ":9090")
	t.Setenv("PORTFLARE_AUTH_PUBLIC_URL", "https://auth.example.test/")
	t.Setenv("PORTFLARE_AUTH_SHARED_SECRET", strings.Repeat("s", 32))
	t.Setenv("PORTFLARE_AUTH_GOOGLE_CLIENT_ID", "google-client")
	t.Setenv("PORTFLARE_AUTH_GOOGLE_CLIENT_SECRET", "google-secret")
	t.Setenv("PORTFLARE_AUTH_GOOGLE_REDIRECT_URL", "https://auth.example.test/auth/google/callback")
	t.Setenv("PORTFLARE_AUTH_CODE_TTL", "2m")
	t.Setenv("PORTFLARE_AUTH_TX_TTL", "3m")
	t.Setenv("PORTFLARE_AUTH_BACKEND_SKEW", "45s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.PublicURL != "https://auth.example.test" {
		t.Fatalf("unexpected public url: %q", cfg.PublicURL)
	}
	if cfg.SharedSecret != strings.Repeat("s", 32) {
		t.Fatal("shared secret not loaded")
	}
	if !cfg.GoogleEnabled() {
		t.Fatal("google should be enabled when all google config is present")
	}
	if cfg.CodeTTL != 2*time.Minute || cfg.TransactionTTL != 3*time.Minute || cfg.BackendSkew != 45*time.Second {
		t.Fatalf("unexpected durations: %#v", cfg)
	}
}

func TestValidateRejectsPartialGoogleConfig(t *testing.T) {
	cfg := Config{GoogleClientID: "google-client"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected partial google config to be rejected")
	}
}

func TestValidateRejectsWeakSharedSecret(t *testing.T) {
	cfg := Config{SharedSecret: "short"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected weak shared secret to be rejected")
	}
}

func TestValidateRequiresSharedSecretWhenGoogleEnabled(t *testing.T) {
	cfg := Config{
		ListenAddr:         ":8081",
		GoogleClientID:     "google-client",
		GoogleClientSecret: "google-secret",
		GoogleRedirectURL:  "https://auth.example.test/auth/google/callback",
		CodeTTL:            time.Minute,
		TransactionTTL:     time.Minute,
		BackendSkew:        time.Minute,
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected google-enabled config without shared secret to be rejected")
	}
}

func TestLoadRejectsInvalidDurations(t *testing.T) {
	t.Setenv("PORTFLARE_AUTH_CODE_TTL", "nope")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid duration error")
	}
}
