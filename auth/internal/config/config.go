package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

const minSharedSecretLength = 32

type Config struct {
	ListenAddr         string
	PublicURL          string
	SharedSecret       string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	CodeTTL            time.Duration
	TransactionTTL     time.Duration
	BackendSkew        time.Duration
}

func Load() (Config, error) {
	codeTTL, err := envDuration("PORTFLARE_AUTH_CODE_TTL", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	txTTL, err := envDuration("PORTFLARE_AUTH_TX_TTL", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	backendSkew, err := envDuration("PORTFLARE_AUTH_BACKEND_SKEW", time.Minute)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		ListenAddr:         env("PORTFLARE_AUTH_LISTEN_ADDR", ":8081"),
		PublicURL:          strings.TrimRight(strings.TrimSpace(os.Getenv("PORTFLARE_AUTH_PUBLIC_URL")), "/"),
		SharedSecret:       strings.TrimSpace(os.Getenv("PORTFLARE_AUTH_SHARED_SECRET")),
		GoogleClientID:     strings.TrimSpace(os.Getenv("PORTFLARE_AUTH_GOOGLE_CLIENT_ID")),
		GoogleClientSecret: strings.TrimSpace(os.Getenv("PORTFLARE_AUTH_GOOGLE_CLIENT_SECRET")),
		GoogleRedirectURL:  strings.TrimSpace(os.Getenv("PORTFLARE_AUTH_GOOGLE_REDIRECT_URL")),
		CodeTTL:            codeTTL,
		TransactionTTL:     txTTL,
		BackendSkew:        backendSkew,
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.ListenAddr) == "" {
		return fmt.Errorf("PORTFLARE_AUTH_LISTEN_ADDR cannot be empty")
	}
	if c.PublicURL != "" {
		u, err := url.Parse(c.PublicURL)
		if err != nil || u.Scheme == "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("PORTFLARE_AUTH_PUBLIC_URL must be an absolute http(s) URL")
		}
	}
	if c.SharedSecret != "" && len(c.SharedSecret) < minSharedSecretLength {
		return fmt.Errorf("PORTFLARE_AUTH_SHARED_SECRET must be at least %d characters", minSharedSecretLength)
	}
	googleFields := 0
	for _, value := range []string{c.GoogleClientID, c.GoogleClientSecret, c.GoogleRedirectURL} {
		if strings.TrimSpace(value) != "" {
			googleFields++
		}
	}
	if googleFields > 0 && googleFields < 3 {
		return fmt.Errorf("google auth requires PORTFLARE_AUTH_GOOGLE_CLIENT_ID, PORTFLARE_AUTH_GOOGLE_CLIENT_SECRET, and PORTFLARE_AUTH_GOOGLE_REDIRECT_URL")
	}
	if c.GoogleRedirectURL != "" {
		u, err := url.Parse(c.GoogleRedirectURL)
		if err != nil || u.Scheme == "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("PORTFLARE_AUTH_GOOGLE_REDIRECT_URL must be an absolute http(s) URL")
		}
	}
	if googleFields == 3 && c.SharedSecret == "" {
		return fmt.Errorf("PORTFLARE_AUTH_SHARED_SECRET is required when Google auth is configured")
	}
	if c.CodeTTL <= 0 || c.TransactionTTL <= 0 || c.BackendSkew <= 0 {
		return fmt.Errorf("auth durations must be positive")
	}
	return nil
}

func (c Config) GoogleEnabled() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != "" && c.GoogleRedirectURL != ""
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return value, nil
}
