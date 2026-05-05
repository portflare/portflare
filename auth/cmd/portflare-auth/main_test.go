package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/portflare/auth/internal/config"
)

func TestHealthz(t *testing.T) {
	h := routes(config.Config{ListenAddr: ":0", PublicURL: "https://auth.example.test"})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["ok"] != true || body["service"] != "portflare-auth" {
		t.Fatalf("unexpected health response: %#v", body)
	}
}

func TestHealthzRejectsNonGet(t *testing.T) {
	h := routes(config.Config{})
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected method not allowed, got %d", rr.Code)
	}
}

func TestReadyzIncludesBuildInfo(t *testing.T) {
	h := routes(config.Config{})
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", rr.Code, rr.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["ok"] != true || body["application"] != "portflare-auth" || body["version"] == "" || body["build_time"] == "" || body["go_version"] == "" {
		t.Fatalf("unexpected ready response: %#v", body)
	}
	if body["build_info_ok"] != true || body["main"] == nil {
		t.Fatalf("expected debug build info in response: %#v", body)
	}
}

func TestReadyzRejectsNonGet(t *testing.T) {
	h := routes(config.Config{})
	req := httptest.NewRequest(http.MethodPost, "/readyz", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected method not allowed, got %d", rr.Code)
	}
}
