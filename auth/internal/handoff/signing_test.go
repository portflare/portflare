package handoff

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestVerifierAcceptsValidSignedRequest(t *testing.T) {
	secret := strings.Repeat("s", 32)
	nonces := NewNonceCache(time.Minute)
	verifier := NewVerifier(secret, time.Minute, nonces)
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	req := newSignedRequest(t, secret, now, "nonce-1", []byte(`{"code":"abc"}`))

	if err := verifier.Verify(req, now); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
}

func TestVerifierRejectsMissingHeaders(t *testing.T) {
	verifier := NewVerifier(strings.Repeat("s", 32), time.Minute, NewNonceCache(time.Minute))
	req, err := http.NewRequest(http.MethodPost, "https://auth.example.test/internal/cli/verify", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatal(err)
	}
	if err := verifier.Verify(req, time.Now()); err == nil {
		t.Fatal("expected missing headers to be rejected")
	}
}

func TestVerifierRejectsTamperedBody(t *testing.T) {
	secret := strings.Repeat("s", 32)
	verifier := NewVerifier(secret, time.Minute, NewNonceCache(time.Minute))
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	req := newSignedRequest(t, secret, now, "nonce-1", []byte(`{"code":"abc"}`))
	req.Body = ioNopCloser{bytes.NewReader([]byte(`{"code":"different"}`))}

	if err := verifier.Verify(req, now); err == nil {
		t.Fatal("expected tampered body to be rejected")
	}
}

func TestVerifierRejectsStaleTimestamp(t *testing.T) {
	secret := strings.Repeat("s", 32)
	verifier := NewVerifier(secret, time.Minute, NewNonceCache(time.Minute))
	signedAt := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	req := newSignedRequest(t, secret, signedAt, "nonce-1", []byte(`{}`))

	if err := verifier.Verify(req, signedAt.Add(2*time.Minute)); err == nil {
		t.Fatal("expected stale timestamp to be rejected")
	}
}

func TestVerifierRejectsNonceReplay(t *testing.T) {
	secret := strings.Repeat("s", 32)
	nonces := NewNonceCache(time.Minute)
	verifier := NewVerifier(secret, time.Minute, nonces)
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	req1 := newSignedRequest(t, secret, now, "nonce-1", []byte(`{}`))
	req2 := newSignedRequest(t, secret, now, "nonce-1", []byte(`{}`))

	if err := verifier.Verify(req1, now); err != nil {
		t.Fatalf("first Verify() error = %v", err)
	}
	if err := verifier.Verify(req2, now); err == nil {
		t.Fatal("expected replayed nonce to be rejected")
	}
}

func TestVerifierRejectsWrongSecret(t *testing.T) {
	req := newSignedRequest(t, strings.Repeat("a", 32), time.Now(), "nonce-1", []byte(`{}`))
	verifier := NewVerifier(strings.Repeat("b", 32), time.Minute, NewNonceCache(time.Minute))
	if err := verifier.Verify(req, time.Now()); err == nil {
		t.Fatal("expected wrong secret to be rejected")
	}
}

func TestNonceCacheExpiresEntries(t *testing.T) {
	cache := NewNonceCache(time.Minute)
	now := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	if !cache.Mark("nonce-1", now) {
		t.Fatal("expected first nonce mark to succeed")
	}
	if cache.Mark("nonce-1", now.Add(30*time.Second)) {
		t.Fatal("expected duplicate nonce to be rejected")
	}
	if !cache.Mark("nonce-1", now.Add(2*time.Minute)) {
		t.Fatal("expected expired nonce to be reusable after ttl")
	}
}

func newSignedRequest(t *testing.T, secret string, ts time.Time, nonce string, body []byte) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "https://auth.example.test/internal/cli/verify", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if err := SignRequest(req, secret, ts, nonce, body); err != nil {
		t.Fatal(err)
	}
	return req
}

type ioNopCloser struct{ *bytes.Reader }

func (ioNopCloser) Close() error { return nil }
