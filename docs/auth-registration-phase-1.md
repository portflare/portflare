# Phase 1: Auth-backed CLI registration

## Goal

Add an auth-backed registration flow for the Portflare CLI while keeping the auth provider logic isolated in a dedicated `auth/` backend service.

The CLI registration flow should prefer the auth service when the Portflare server knows it is configured and healthy. If the auth service is not configured/running, the existing direct `POST /api/register` fallback remains available.

## Scope

Phase 1 introduces:

- a new `auth/` service, intended to build as `portflare-auth`
- Google OAuth as the first real auth provider
- a redirect-based CLI handoff, not polling
- a secure authorization-code exchange between CLI, Portflare server, and auth service
- server-side verification that only the configured trusted auth service can complete registrations
- documentation and tests for the handoff/security behavior

Out of scope for Phase 1:

- GitHub OAuth provider implementation
- username/password provider implementation
- persistent database-backed auth service storage, unless needed for Google OAuth/session correctness
- full hosted account-management UI
- long-lived browser sessions beyond what the auth flow requires

## High-level architecture

```text
CLI -> Portflare server -> auth service -> Google OAuth -> auth service -> CLI callback -> Portflare server -> auth service verification -> Portflare user/key
```

Responsibilities:

### CLI

- starts registration with the Portflare server
- starts a temporary localhost callback listener
- generates strong one-time values:
  - `state`
  - `nonce`
  - `code_verifier`
  - `code_challenge`
- opens the browser when possible
- always prints the auth URL for headless/server environments
- validates callback `state`
- exchanges the returned one-time code with the Portflare server
- prints resulting `PORTFLARE_SERVER_URL` and `PORTFLARE_CLIENT_KEY` exports

### Portflare server

- remains the authority for Portflare users and client API keys
- detects whether the configured auth service is available
- tells the CLI whether to use auth-backed registration or direct fallback
- exchanges CLI authorization codes by calling the trusted auth service over a backend channel
- creates the Portflare user only after successful auth-service verification
- never accepts arbitrary/auth-service URLs from the CLI

### Auth service

- owns authentication provider logic
- implements Google OAuth first
- stores short-lived CLI auth transactions
- validates Google identity
- issues short-lived, single-use authorization codes to the CLI redirect URI
- verifies authorization codes for the Portflare server
- exposes provider interfaces so GitHub/password can be added later without changing the CLI flow

## CLI flow

### 1. User starts registration

```bash
portflare register --server https://r.myw.io
```

Optional flags may still be supported for fallback/direct registration:

```bash
portflare register --server https://r.myw.io --user alice --email alice@example.com
```

### 2. CLI asks the server how to register

```http
POST /api/register/start
Content-Type: application/json

{
  "redirect_uri": "http://127.0.0.1:49152/callback",
  "state": "...",
  "nonce": "...",
  "code_challenge": "...",
  "code_challenge_method": "S256"
}
```

If auth service is configured and healthy:

```json
{
  "mode": "auth",
  "auth_url": "https://auth.example.com/cli/start?...",
  "expires_in": 300
}
```

If auth service is unavailable/not configured and fallback is allowed:

```json
{
  "mode": "direct"
}
```

### 3. CLI opens/prints auth URL

The CLI should try to open the system browser, but must always print the URL:

```text
Open this URL to complete registration:

https://auth.example.com/cli/start?...
```

This keeps the flow usable over SSH or on a server.

### 4. Auth service handles Google OAuth

The auth service redirects the user to Google with its configured OAuth client credentials.

After successful Google authentication, the auth service validates at least:

- OAuth `state`
- ID token signature via Google JWKS or official library
- token issuer
- audience/client ID
- expiration
- email and email verification status
- nonce, if included in the ID-token flow

### 5. Auth service redirects to CLI callback

```http
302 Location: http://127.0.0.1:49152/callback?code=...&state=...
```

The redirect must contain only a short-lived one-time authorization code. It must not contain the Portflare API key.

### 6. CLI validates callback and exchanges code

CLI validates:

- callback is received by the local listener
- `state` matches exactly
- code is present
- callback is accepted once

Then it exchanges the code with the Portflare server:

```http
POST /api/register/exchange
Content-Type: application/json

{
  "code": "...",
  "code_verifier": "...",
  "redirect_uri": "http://127.0.0.1:49152/callback"
}
```

### 7. Portflare server verifies with trusted auth service

The Portflare server calls its configured auth service over a backend endpoint:

```http
POST /internal/cli/verify
Authorization: Bearer <server-to-auth secret or signed assertion>
Content-Type: application/json

{
  "code": "...",
  "code_verifier": "...",
  "redirect_uri": "http://127.0.0.1:49152/callback"
}
```

The auth service verifies:

- backend caller is the configured Portflare server
- code exists
- code is unexpired
- code is single-use and unused
- redirect URI matches the original transaction
- PKCE verifier matches stored challenge
- authenticated identity is complete and valid

Then returns identity claims:

```json
{
  "subject": "google-oauth-sub",
  "provider": "google",
  "email": "alice@example.com",
  "email_verified": true,
  "display_name": "Alice Smith",
  "suggested_user_name": "alice"
}
```

### 8. Portflare server creates user/key

The server creates or resolves the Portflare user according to existing registration rules and returns the client key to the CLI:

```json
{
  "user_name": "alice",
  "public_user_label": "alice",
  "email": "alice@example.com",
  "api_key": "pf_..."
}
```

## Trust and security requirements

### Auth service trust must be pinned/configured by the server

The Portflare server must never accept an auth-service URL from the CLI or browser callback.

Use server-side configuration only:

```bash
PORTFLARE_AUTH_URL=https://auth.example.com
PORTFLARE_AUTH_REQUIRED=false
PORTFLARE_AUTH_SHARED_SECRET=...
```

Optional hardening:

```bash
PORTFLARE_AUTH_EXPECTED_ISSUER=https://auth.example.com
PORTFLARE_AUTH_PINNED_JWKS_URL=https://auth.example.com/.well-known/jwks.json
```

The server should only talk to this configured auth service. This prevents users/attackers from spinning up their own fake auth service and tricking the server into accepting it.

### Backend verification must be authenticated

The auth service verification endpoint must require authenticated server-to-auth communication. Acceptable Phase 1 options:

1. HMAC-signed requests with timestamp and nonce
2. Bearer shared secret over HTTPS
3. mTLS, if deployment supports it
4. JWT assertions signed by the server

Recommended Phase 1 default: HMAC-signed requests using `PORTFLARE_AUTH_SHARED_SECRET`, including:

- method
- path
- timestamp
- nonce
- body hash

Reject if:

- signature mismatch
- timestamp is outside allowed skew, e.g. 60 seconds
- nonce is reused

### Authorization codes

Authorization codes must be:

- cryptographically random
- short-lived, e.g. 60–300 seconds
- single-use
- stored hashed at rest/in memory, not plaintext if avoidable
- bound to:
  - redirect URI
  - code challenge
  - provider identity
  - transaction ID

### PKCE

Use S256 PKCE:

```text
code_challenge = base64url(SHA256(code_verifier))
```

Do not support `plain` unless there is a strong compatibility reason.

### Redirect URI validation

For CLI registration, allowed redirect URIs should be restricted to loopback addresses:

- `http://127.0.0.1:<ephemeral-port>/callback`
- `http://localhost:<ephemeral-port>/callback`
- optionally `http://[::1]:<ephemeral-port>/callback`

No HTTPS requirement is needed for loopback callback, consistent with OAuth native-app patterns.

The auth service must reject non-loopback redirect URIs for CLI flows unless explicitly configured.

### No secrets in URLs

Never put the Portflare API key in:

- browser redirect query params
- auth URLs
- logs
- referrers

Only a short-lived auth code may appear in the CLI callback URL.

### Fallback behavior

Direct `/api/register` fallback is allowed only when the server is not configured to use a running auth service.

Recommended server behavior:

- if auth service configured and healthy: use auth flow
- if auth service configured but unhealthy:
  - if `PORTFLARE_AUTH_REQUIRED=true`: fail registration
  - if `PORTFLARE_AUTH_REQUIRED=false`: fallback may be allowed, but should log a warning
- if auth service not configured: direct `/api/register` remains available

For stronger production security, deployments should set:

```bash
PORTFLARE_AUTH_REQUIRED=true
```

### Account linking

Phase 1 should avoid unsafe implicit linking.

Recommended behavior:

- if a Portflare user already exists with the same provider+subject mapping, return/rotate/create a client key according to server policy
- if email matches an existing user but provider subject is not linked, do not automatically take over that user unless a safe linking policy exists
- initial implementation may create a new user from verified Google identity and return conflict if the suggested user label is already taken

## Auth service provider interface

The auth service should be provider-oriented so adding GitHub later is straightforward.

Suggested Go interface shape:

```go
type Provider interface {
    Name() string
    Begin(w http.ResponseWriter, r *http.Request, tx AuthTransaction) error
    Complete(w http.ResponseWriter, r *http.Request) (Identity, error)
}

type Identity struct {
    Provider      string
    Subject       string
    Email         string
    EmailVerified bool
    DisplayName   string
}
```

Google is the first provider:

```text
/auth/google/start
/auth/google/callback
```

Later GitHub can be added as another provider without changing CLI/server handoff semantics.

## Initial repository layout

```text
auth/
  go.mod
  README.md
  cmd/portflare-auth/main.go
  internal/config
  internal/providers
  internal/providers/google
  internal/store
  internal/handoff
```

Initial storage can be in-memory for CLI auth transactions, provided the service documents that restarts invalidate active auth attempts. Provider/account persistence can be introduced later if needed for account linking.

## Phase 1 implementation checklist

### Auth service

- [ ] create `auth/` Go module and `portflare-auth` binary
- [ ] add config/env parsing
- [ ] add health endpoint
- [ ] add provider interface
- [ ] add Google OAuth provider
- [ ] add CLI transaction store
- [ ] add `/cli/start`
- [ ] add Google callback route
- [ ] add `/internal/cli/verify` with authenticated backend verification
- [ ] add tests for state, PKCE, code expiry, single-use code, redirect validation, and signature validation

### Server

- [ ] add auth-service config
- [ ] add auth-service health detection
- [ ] add `POST /api/register/start`
- [ ] add `POST /api/register/exchange`
- [ ] keep direct `POST /api/register` fallback only when auth service is unavailable/not configured according to policy
- [ ] create users/API keys only after trusted auth verification
- [ ] add tests for fallback, auth-mode start, exchange success, exchange rejection, unhealthy auth service policy

### Client

- [ ] update `portflare register` to call `/api/register/start`
- [ ] add localhost callback listener
- [ ] generate state/nonce/PKCE values
- [ ] open browser when possible
- [ ] always print auth URL
- [ ] validate callback state
- [ ] exchange code for registration result
- [ ] retain direct fallback mode support
- [ ] add tests for callback validation, state mismatch, exchange success/failure, fallback mode

## Open decisions before implementation

- exact auth service public base URL and route names
- HMAC vs bearer secret vs JWT/mTLS for server-to-auth verification
- whether auth service should expose JWKS/signed identity tokens in addition to `/internal/cli/verify`
- account-linking policy for existing emails/users
- whether successful auth should create a new API key every time or return conflict for existing users
