package handoff

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	HeaderTimestamp = "X-Portflare-Timestamp"
	HeaderNonce     = "X-Portflare-Nonce"
	HeaderSignature = "X-Portflare-Signature"

	minSharedSecretLength = 32
)

type Verifier struct {
	secret string
	skew   time.Duration
	nonces *NonceCache
}

type NonceCache struct {
	mu     sync.Mutex
	ttl    time.Duration
	nonces map[string]time.Time
}

func NewVerifier(secret string, skew time.Duration, nonces *NonceCache) *Verifier {
	if skew <= 0 {
		skew = time.Minute
	}
	if nonces == nil {
		nonces = NewNonceCache(skew * 2)
	}
	return &Verifier{secret: secret, skew: skew, nonces: nonces}
}

func NewNonceCache(ttl time.Duration) *NonceCache {
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &NonceCache{ttl: ttl, nonces: map[string]time.Time{}}
}

func (c *NonceCache) Mark(nonce string, now time.Time) bool {
	nonce = strings.TrimSpace(nonce)
	if nonce == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for value, expiresAt := range c.nonces {
		if !expiresAt.After(now) {
			delete(c.nonces, value)
		}
	}
	if _, ok := c.nonces[nonce]; ok {
		return false
	}
	c.nonces[nonce] = now.Add(c.ttl)
	return true
}

func SignRequest(req *http.Request, secret string, ts time.Time, nonce string, body []byte) error {
	if err := validateSecret(secret); err != nil {
		return err
	}
	nonce = strings.TrimSpace(nonce)
	if nonce == "" {
		return errors.New("nonce is required")
	}
	if ts.IsZero() {
		return errors.New("timestamp is required")
	}
	timestamp := strconv.FormatInt(ts.UTC().Unix(), 10)
	bodyHash := hashBody(body)
	sig := computeSignature(secret, req.Method, signaturePath(req), timestamp, nonce, bodyHash)
	req.Header.Set(HeaderTimestamp, timestamp)
	req.Header.Set(HeaderNonce, nonce)
	req.Header.Set(HeaderSignature, sig)
	return nil
}

func (v *Verifier) Verify(req *http.Request, now time.Time) error {
	if err := validateSecret(v.secret); err != nil {
		return err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	timestamp := strings.TrimSpace(req.Header.Get(HeaderTimestamp))
	nonce := strings.TrimSpace(req.Header.Get(HeaderNonce))
	signature := strings.TrimSpace(req.Header.Get(HeaderSignature))
	if timestamp == "" || nonce == "" || signature == "" {
		return errors.New("missing signature headers")
	}
	signedAtUnix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid timestamp: %w", err)
	}
	signedAt := time.Unix(signedAtUnix, 0).UTC()
	if now.Sub(signedAt) > v.skew || signedAt.Sub(now) > v.skew {
		return errors.New("signature timestamp outside allowed skew")
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("read request body: %w", err)
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	bodyHash := hashBody(body)
	expected := computeSignature(v.secret, req.Method, signaturePath(req), timestamp, nonce, bodyHash)
	if subtle.ConstantTimeCompare([]byte(signature), []byte(expected)) != 1 {
		return errors.New("invalid signature")
	}
	if !v.nonces.Mark(nonce, now) {
		return errors.New("replayed nonce")
	}
	return nil
}

func validateSecret(secret string) error {
	if len(secret) < minSharedSecretLength {
		return fmt.Errorf("shared secret must be at least %d characters", minSharedSecretLength)
	}
	return nil
}

func signaturePath(req *http.Request) string {
	if req.URL == nil || req.URL.EscapedPath() == "" {
		return "/"
	}
	return req.URL.EscapedPath()
}

func hashBody(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func computeSignature(secret, method, path, timestamp, nonce, bodyHash string) string {
	payload := strings.Join([]string{strings.ToUpper(method), path, timestamp, nonce, bodyHash}, "\n")
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
