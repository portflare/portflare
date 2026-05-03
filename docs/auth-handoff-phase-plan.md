# Implementation Plan

## Goal
Deliver Phase 1 auth-backed CLI registration and Phase 2 auth-service browser/dashboard handoff while keeping Portflare server authoritative for users/API keys and isolating OAuth/provider logic in a new `auth/` service.

## Tasks

### Commit 1: Add shared auth handoff protocol types
1. **Add wire DTOs for registration and browser handoff**
   - File: `protocol/types/auth.go`
   - Changes: Add structs for `RegisterStartRequest`, `RegisterStartResponse`, `RegisterExchangeRequest`, `RegistrationResponse`, `AuthIdentityClaims`, `InternalVerifyRequest`, `BrowserStartRequest/Response` if needed, and typed constants for modes (`auth`, `direct`) and providers (`google`). Include JSON tags matching docs.
   - Acceptance: `cd protocol && go test ./...` succeeds.

2. **Add protocol validation helpers for PKCE, redirect URIs, and return paths**
   - File: `protocol/validation/auth.go`
   - Changes: Add helpers for S256 code challenge format, loopback CLI redirect URI validation, relative same-origin browser return URL validation, and provider/subject non-empty validation.
   - Acceptance: Unit tests cover valid/invalid loopback redirects, invalid external redirects, and unsafe return paths like `//evil.test`.

3. **Update dependent module versions or local replace strategy for implementation**
   - File: `server/go.mod`, `client/go.mod`, new `auth/go.mod`
   - Changes: Decide whether to use a released `github.com/portflare/protocol` version or temporary `replace ../protocol` during multi-repo development.
   - Acceptance: All modules build with the same protocol API.

### Commit 2: Introduce the `portflare-auth` service skeleton and trusted backend signing
4. **Create new auth module and binary skeleton**
   - File: `auth/go.mod`, `auth/cmd/portflare-auth/main.go`, `auth/README.md`, `auth/Dockerfile`, `auth/Makefile`
   - Changes: New Go module `github.com/portflare/auth`; binary parses config, starts HTTP server, exposes `GET /healthz`, and logs version/config without secrets.
   - Acceptance: `cd auth && go test ./... && go build ./cmd/portflare-auth` succeeds; `docker build -t portflare-auth:dev ./auth` succeeds.

5. **Add auth-service config/env parsing**
   - File: `auth/internal/config/config.go`
   - Changes: Add env vars: `PORTFLARE_AUTH_LISTEN_ADDR`, `PORTFLARE_AUTH_PUBLIC_URL`, `PORTFLARE_AUTH_SHARED_SECRET`, `PORTFLARE_AUTH_ALLOWED_SERVER_ISSUER` or server identifier, `PORTFLARE_AUTH_GOOGLE_CLIENT_ID`, `PORTFLARE_AUTH_GOOGLE_CLIENT_SECRET`, `PORTFLARE_AUTH_GOOGLE_REDIRECT_URL`, `PORTFLARE_AUTH_CODE_TTL`, `PORTFLARE_AUTH_TX_TTL`, `PORTFLARE_AUTH_BACKEND_SKEW`.
   - Acceptance: Tests verify required Google/shared-secret fields are enforced when Google/provider routes are enabled.

6. **Implement HMAC-signed backend request verification**
   - File: `auth/internal/handoff/signing.go`, `server/cmd/portflare-server/auth_signing.go`
   - Changes: Use `PORTFLARE_AUTH_SHARED_SECRET` for HMAC over method, path, timestamp, nonce, and SHA-256 body hash. Add auth-service replay nonce cache with expiry. Reject timestamp skew > default 60s and reused nonces.
   - Acceptance: Auth tests cover valid signature, bad body, stale timestamp, nonce reuse, missing header; server tests verify signer generates accepted headers.

### Commit 3: Implement Phase 1 auth-service CLI flow
7. **Add provider abstraction and Google provider**
   - File: `auth/internal/providers/provider.go`, `auth/internal/providers/google/google.go`
   - Changes: Define `Provider` and `Identity` interfaces from docs. Implement Google OAuth start/callback with state validation and ID-token validation using Google libraries/JWKS. Require `email_verified=true` for identities that use email.
   - Acceptance: Provider tests use mocked token verifier/JWKS to cover issuer, audience, expiry, nonce, and email verification failures.

8. **Add CLI transaction and authorization code store**
   - File: `auth/internal/store/memory.go`
   - Changes: In-memory stores for CLI transactions and hashed single-use authorization codes. Store state, nonce, redirect URI, code challenge, provider identity, expiry, used flag, and created timestamps. Restarts invalidate active attempts by design.
   - Acceptance: Tests cover expiry, single-use, code hash lookup, and cleanup.

9. **Add public CLI auth routes**
   - File: `auth/cmd/portflare-auth/main.go`, `auth/internal/handoff/cli.go`
   - Changes: Implement `GET /cli/start`, `GET /auth/google/start`, `GET /auth/google/callback` or one coherent route map. Validate redirect URI is loopback and PKCE is S256. Redirect to Google, then redirect back to CLI callback with only `code` and `state`.
   - Acceptance: Handler tests verify external redirect URI rejection, missing PKCE rejection, callback includes no API key/secrets, state mismatch failure.

10. **Add internal CLI verification endpoint**
   - File: `auth/internal/handoff/cli.go`
   - Changes: Implement `POST /internal/cli/verify`; require HMAC; validate code, redirect URI, code verifier, expiry, and single-use. Return `AuthIdentityClaims`.
   - Acceptance: Tests cover success, wrong verifier, wrong redirect URI, expired code, duplicate verify, and unsigned requests.

### Commit 4: Add Phase 1 server-side auth registration support
11. **Extend server config for auth service**
   - File: `server/cmd/portflare-server/main.go` or `server/cmd/portflare-server/config.go`
   - Changes: Add `AuthURL`, `AuthRequired`, `AuthSharedSecret`, `AuthHealthTTL`, `AuthRequestTimeout` to `Config`. Parse env vars: `PORTFLARE_AUTH_URL`, `PORTFLARE_AUTH_REQUIRED`, `PORTFLARE_AUTH_SHARED_SECRET`, `PORTFLARE_AUTH_HEALTH_TTL`, `PORTFLARE_AUTH_REQUEST_TIMEOUT`. Validate that configured auth URL is absolute http/https and shared secret is present when auth URL is configured.
   - Acceptance: Existing tests that instantiate `Server` directly still pass; config tests cover env parsing and invalid URL/secret.

12. **Add server auth-service client and cached health checks**
   - File: `server/cmd/portflare-server/auth_client.go`
   - Changes: Client calls configured `PORTFLARE_AUTH_URL` only, never user-supplied URLs. Add cached `GET /healthz` health with short timeout/TTL so UI requests do not synchronously block on every request. Add methods `VerifyCLI` and later `VerifyBrowser` using HMAC signer.
   - Acceptance: Tests use `httptest.Server` to verify URL pinning, timeout behavior, healthy/unhealthy cache, and signed requests.

13. **Add external identity state model before creating auth-backed users**
   - File: `server/cmd/portflare-server/main.go` or `server/cmd/portflare-server/state.go`
   - Changes: Add:
     ```go
     type UserExternalIdentity struct {
       Provider string `json:"provider"`
       Subject string `json:"subject"`
       Email string `json:"email,omitempty"`
       LinkedAt time.Time `json:"linked_at"`
     }
     ```
     Add `ExternalIdentities []UserExternalIdentity `json:"external_identities,omitempty"`` to `User`. Preserve backwards compatibility for existing JSON state files.
   - Acceptance: Existing state tests pass; new load/save test verifies missing field migrates cleanly and field persists.

14. **Refactor user creation/resolution into shared helpers**
   - File: `server/cmd/portflare-server/registration.go` or `main.go`
   - Changes: Extract direct user creation, public-label collision checks, and auth identity creation into helpers. Add `findUserByExternalIdentityLocked(provider, subject)`. Auth-backed policy: provider+subject match returns existing user/API key; email-only match without linked provider returns `409` conflict; no match creates user if registration is open; suggested username collisions return `409` with clear message.
   - Acceptance: Tests cover direct creation parity, external identity match, email-only conflict, label collision, registration closed.

15. **Add `POST /api/register/start` and `POST /api/register/exchange`**
   - File: `server/cmd/portflare-server/main.go`, `server/cmd/portflare-server/auth_registration.go`
   - Changes: Route `/api/register/start` decides mode: auth if configured+healthy; direct if not configured or configured unhealthy and `PORTFLARE_AUTH_REQUIRED=false`; error if required and unhealthy. It returns auth URL built from server-configured auth URL and CLI-supplied redirect/state/nonce/challenge. `/api/register/exchange` verifies the code through auth client and returns `RegistrationResponse` after safe user resolution.
   - Acceptance: Handler tests verify auth mode, direct mode, required/unhealthy failure, exchange success, verification rejection, and that client cannot choose auth-service URL.

16. **Gate legacy `POST /api/register` fallback**
   - File: `server/cmd/portflare-server/main.go` or `server/cmd/portflare-server/auth_registration.go`
   - Changes: Keep direct registration only when auth service is not configured or is configured but unhealthy and `PORTFLARE_AUTH_REQUIRED=false`. When configured+healthy or required, return `403`/`409` with guidance to use `/api/register/start`.
   - Acceptance: Existing direct registration tests updated for no-auth-config case; new tests cover configured healthy blocks direct fallback.

### Commit 5: Update Phase 1 CLI registration flow
17. **Restructure `portflare register` command**
   - File: `client/cmd/portflare/main.go` or `client/cmd/portflare/register.go`
   - Changes: Usage becomes `portflare register --server <url> [--user <name>] [--email <email>] [--no-browser] [--timeout <duration>]`. Start with `/api/register/start`; if server returns `mode=direct`, require/use `--user` and optional `--email` to call legacy `/api/register`.
   - Acceptance: Existing direct fallback tests pass after updates; missing `--user` only fails in direct mode.

18. **Add CLI PKCE/state and localhost callback listener**
   - File: `client/cmd/portflare/register.go`
   - Changes: Generate cryptographically random `state`, `nonce`, `code_verifier`, S256 `code_challenge`. Bind temporary listener to `127.0.0.1:0` with `/callback`; accept exactly one callback; validate state; reject missing code/state mismatch; cleanly shut down on timeout or interrupt.
   - Acceptance: Unit tests cover state mismatch, double callback, timeout cleanup, and verifier/challenge format.

19. **Open/print browser URL and exchange code**
   - File: `client/cmd/portflare/register.go`
   - Changes: Always print auth URL. Try OS browser opener unless `--no-browser`; failures are non-fatal. Exchange callback code at `/api/register/exchange` with verifier and redirect URI. Print same exports as today; never print secrets from URL.
   - Acceptance: Tests using `httptest.Server` cover auth start/exchange success, exchange failure, direct fallback, and stdout format.

### Commit 6: Implement Phase 2 server session/browser handoff foundation
20. **Add Phase 2 server config and startup validation**
   - File: `server/cmd/portflare-server/main.go` or `server/cmd/portflare-server/config.go`
   - Changes: Add `SessionSecret`, `SessionTTL`, `BrowserAuthTxTTL`, `CookieSecure`, `AuthBrowserEnabled`. Env vars: `PORTFLARE_SESSION_SECRET`, `PORTFLARE_SESSION_TTL`, `PORTFLARE_BROWSER_AUTH_TX_TTL`, `PORTFLARE_SESSION_COOKIE_SECURE`, optional `PORTFLARE_AUTH_BROWSER_ENABLED`. If auth URL is configured and browser auth enabled, require session secret with at least 32 bytes/strong base64 or clear documented minimum.
   - Acceptance: Config tests cover missing/weak session secret and local HTTP cookie-secure override.

21. **Add signed session cookie helpers**
   - File: `server/cmd/portflare-server/session.go`
   - Changes: Implement HMAC-signed authenticated cookie containing minimal claims: `user_name`, `email`, `provider`, `subject`, `issued_at`, `expires_at`. Use `HttpOnly`, `SameSite=Lax`, `Secure` according to config. Reject expired/tampered cookies. Cookie name e.g. `pf_session`.
   - Acceptance: Tests cover round trip, tamper rejection, expiry, `Secure` flag behavior, no API key in cookie.

22. **Add browser auth transaction store**
   - File: `server/cmd/portflare-server/browser_auth.go`
   - Changes: Store state, nonce or PKCE verifier/challenge, safe return path, redirect URI, expiry. In-memory only; restart invalidates pending browser login.
   - Acceptance: Tests cover expiry, unsafe return URL rejection, and state lookup single use.

23. **Add browser auth routes**
   - File: `server/cmd/portflare-server/main.go`, `server/cmd/portflare-server/browser_auth.go`
   - Changes: Register `GET /auth/start`, `GET /auth/callback`, `POST /auth/logout`. `/auth/start` accepts/derives a relative `return_to`, creates transaction, and redirects to configured `PORTFLARE_AUTH_URL/browser/start`. `/auth/callback` validates state/code, verifies with auth service `/internal/browser/verify`, maps/creates user via external identity helpers, issues session cookie, redirects to original relative path. `/auth/logout` clears cookie.
   - Acceptance: Handler tests verify redirects only to configured auth URL, callback success issues cookie, bad state/code fails, unsafe return path rejected, logout clears cookie.

### Commit 7: Add Phase 2 auth-service browser flow
24. **Add browser transaction/code support in auth service**
   - File: `auth/internal/store/memory.go`, `auth/internal/handoff/browser.go`
   - Changes: Separate browser transactions from CLI transactions but share code issuance, provider completion, HMAC verification, and identity claims.
   - Acceptance: Store tests show CLI and browser codes are scoped and cannot be cross-verified.

25. **Add browser public and internal auth-service routes**
   - File: `auth/cmd/portflare-auth/main.go`, `auth/internal/handoff/browser.go`
   - Changes: Implement `GET /browser/start`, Google browser callback (`/browser/callback/google` or consistent provider route), and `POST /internal/browser/verify`. Validate server callback redirect URI points to configured/allowed Portflare server origin; return only short-lived code through URL; verify code with HMAC backend call.
   - Acceptance: Handler tests cover redirect validation, state handling, code single-use, no secrets in URL, signed internal verify.

### Commit 8: Refactor server identity resolution for Phase 2
26. **Implement layered identity resolution**
   - File: `server/cmd/portflare-server/identity.go` or `main.go`
   - Changes: Split current `requireIdentity` into: local dev identity, trusted auth headers, session cookie, browser handoff, final 401. Preserve `PORTFLARE_DISABLE_AUTH=true` behavior first. Implement `PORTFLARE_TRUST_AUTH_HEADERS`; if false, ignore `X-Auth-Request-*` headers except in local dev mode. Preserve admin calculation via `isAdmin`.
   - Acceptance: Tests cover precedence: local dev > headers > session > browser handoff > 401; header trust disabled; admin from username/email still works.

27. **Classify browser vs API/websocket auth responses**
   - File: `server/cmd/portflare-server/identity.go`, route handlers in `main.go`
   - Changes: For HTML/dashboard routes (`/me`, `/admin`, host-aware user pages), redirect to auth-service when available. For JSON API (`/api/me/*`, `/api/admin/*`) return `401` JSON with `auth_url` unless explicit session exists. For `/ws/ui`, return clean 401/error unless session/header/local identity exists; do not redirect websockets.
   - Acceptance: Tests verify `/me` gets 302, `/api/me/state` gets JSON 401 with `auth_url`, `/ws/ui` does not redirect, public routes `/healthz`, `/connect`, registration endpoints, and app proxy do not trigger login.

28. **Ensure dashboard user provisioning uses external identities**
   - File: `server/cmd/portflare-server/identity.go`, `server/cmd/portflare-server/registration.go`
   - Changes: Update `ensureUser` to use provider+subject when identity comes from session/auth-service; header identities continue current username-based behavior but do not create external identity links unless provider/subject is present. Registration closed blocks new auth-service dashboard users with 403.
   - Acceptance: Tests cover session identity creates user when open, fails when closed, existing provider+subject maps to user, email-only conflict is not takeover.

### Commit 9: Documentation, compose, and release integration
29. **Document deployment and env vars**
   - File: `docs/auth-registration-phase-1.md`, `docs/auth-dashboard-phase-2.md`, `server/README.md`, `client/README.md`, `auth/README.md`
   - Changes: Update docs from proposal to implementation details: route names, env vars, fallback policy, session TTL, Google callback URLs, direct registration gating, and warnings about in-memory auth transactions.
   - Acceptance: Docs list all required env vars and exact commands for local/manual validation.

30. **Add auth service to local compose example**
   - File: `s41nn0n-homelab-portflare/docker-compose.yml`
   - Changes: Add optional `portflare-auth` service, auth-related env vars for `portflare`, shared secret wiring via `.env`, Caddy labels/routes for auth public URL if this compose is intended as deployment example.
   - Acceptance: `docker compose -f s41nn0n-homelab-portflare/docker-compose.yml config` succeeds with placeholder env vars.

31. **Add Docker/release build integration**
   - File: `.github/workflows/*` if present, `auth/Dockerfile`, `auth/Makefile`
   - Changes: Ensure auth image and binary can be built similarly to server/client. Do not break existing server/client image builds.
   - Acceptance: Docker build commands below succeed.

## Files to Modify
- `protocol/types/auth.go` - shared request/response/identity structs for auth handoff.
- `protocol/validation/auth.go` - PKCE, redirect URI, return path, and identity validation helpers.
- `server/cmd/portflare-server/main.go` - config fields/env parsing if not split, route registration, `User` model, fallback gating, identity integration.
- `server/cmd/portflare-server/auth_client.go` - trusted auth-service client, health cache, HMAC-signed verify calls.
- `server/cmd/portflare-server/auth_signing.go` - HMAC signer shared by CLI/browser verify calls.
- `server/cmd/portflare-server/registration.go` or `auth_registration.go` - registration start/exchange handlers and shared user creation/resolution helpers.
- `server/cmd/portflare-server/session.go` - signed browser session cookie helpers.
- `server/cmd/portflare-server/browser_auth.go` - browser handoff transaction store and `/auth/*` handlers.
- `server/cmd/portflare-server/identity.go` - layered identity resolution and browser/API auth response behavior.
- `server/cmd/portflare-server/registration_test.go` - direct fallback, auth start/exchange, user creation policy tests.
- `server/cmd/portflare-server/main_test.go` - identity precedence, session, route behavior, state migration tests.
- `client/cmd/portflare/main.go` - register command dispatch/usage; preferably move flow to `register.go`.
- `client/cmd/portflare/register.go` - auth-backed CLI registration implementation.
- `client/cmd/portflare/register_test.go` - direct fallback and browser callback/exchange tests.
- `server/go.mod`, `client/go.mod` - protocol dependency update/replace strategy.
- `s41nn0n-homelab-portflare/docker-compose.yml` - optional auth service and env documentation.
- `docs/auth-registration-phase-1.md`, `docs/auth-dashboard-phase-2.md`, `server/README.md`, `client/README.md` - implementation docs.

## New Files
- `auth/go.mod` - new auth service Go module.
- `auth/README.md` - auth service setup, env vars, provider callback URLs.
- `auth/Dockerfile` - container image for `portflare-auth`.
- `auth/Makefile` - build/release helpers consistent with server/client.
- `auth/cmd/portflare-auth/main.go` - auth service entrypoint and route table.
- `auth/internal/config/config.go` - env/config parsing and validation.
- `auth/internal/handoff/signing.go` - HMAC backend verification and nonce replay cache.
- `auth/internal/handoff/cli.go` - CLI start/callback/internal verify handlers.
- `auth/internal/handoff/browser.go` - browser start/callback/internal verify handlers.
- `auth/internal/providers/provider.go` - provider interface and identity model.
- `auth/internal/providers/google/google.go` - Google OAuth provider implementation.
- `auth/internal/store/memory.go` - in-memory transactions/codes/nonces with expiry.
- Associated `*_test.go` files in `auth/internal/...`, `server/cmd/portflare-server/...`, and `client/cmd/portflare/...`.

## Data Models
- `UserExternalIdentity` in server state:
  - `provider` and `subject` are the stable unique link.
  - `email` is informational and must not be used alone for takeover.
  - `linked_at` records link creation time.
- Extend server `authIdentity` with optional `Provider` and `Subject` so session/auth-service identities do not degrade to username/email-only mapping.
- Auth-service transaction model:
  - CLI transaction: `state`, `nonce`, `redirect_uri`, `code_challenge`, `code_challenge_method`, `provider`, `identity`, `expires_at`.
  - Browser transaction: `state`, `redirect_uri`, safe `return_to`, optional PKCE/nonce, `provider`, `identity`, `expires_at`.
  - Authorization code: hashed code, flow type, transaction ID, redirect URI, PKCE challenge where applicable, identity, `expires_at`, `used_at`.

## Endpoints
- Server Phase 1:
  - `POST /api/register/start` - starts CLI registration, returns `mode=auth` with configured auth URL or `mode=direct`.
  - `POST /api/register/exchange` - exchanges CLI code through trusted auth service and returns Portflare API key.
  - `POST /api/register` - legacy direct fallback, gated by auth availability/required policy.
- Auth service Phase 1:
  - `GET /healthz`
  - `GET /cli/start`
  - `GET /auth/google/start` and `GET /auth/google/callback` or a single consistent provider callback route.
  - `POST /internal/cli/verify`
- Server Phase 2:
  - `GET /auth/start`
  - `GET /auth/callback`
  - `POST /auth/logout`
- Auth service Phase 2:
  - `GET /browser/start`
  - `GET /browser/callback/google` or consistent provider route.
  - `POST /internal/browser/verify`

## Env Vars
- Server existing retained: `PORTFLARE_DISABLE_AUTH`, `PORTFLARE_TRUST_AUTH_HEADERS`, `PORTFLARE_REGISTRATION_OPEN`, `PORTFLARE_ADMIN_USERS`, `PORTFLARE_BASE_DOMAIN`, `PORTFLARE_STATE_PATH`.
- Server new:
  - `PORTFLARE_AUTH_URL`
  - `PORTFLARE_AUTH_REQUIRED` default `false`
  - `PORTFLARE_AUTH_SHARED_SECRET`
  - `PORTFLARE_AUTH_HEALTH_TTL` default e.g. `10s`
  - `PORTFLARE_AUTH_REQUEST_TIMEOUT` default e.g. `3s`
  - `PORTFLARE_SESSION_SECRET`
  - `PORTFLARE_SESSION_TTL` default e.g. `12h`
  - `PORTFLARE_BROWSER_AUTH_TX_TTL` default e.g. `5m`
  - `PORTFLARE_SESSION_COOKIE_SECURE` default true for non-local public URLs; allow false for local HTTP dev.
  - Optional `PORTFLARE_AUTH_BROWSER_ENABLED` default true when auth URL and session secret exist.
- Auth service new:
  - `PORTFLARE_AUTH_LISTEN_ADDR`
  - `PORTFLARE_AUTH_PUBLIC_URL`
  - `PORTFLARE_AUTH_SHARED_SECRET`
  - `PORTFLARE_AUTH_CODE_TTL` default `5m`
  - `PORTFLARE_AUTH_TX_TTL` default `5m`
  - `PORTFLARE_AUTH_BACKEND_SKEW` default `60s`
  - `PORTFLARE_AUTH_GOOGLE_CLIENT_ID`
  - `PORTFLARE_AUTH_GOOGLE_CLIENT_SECRET`
  - `PORTFLARE_AUTH_GOOGLE_REDIRECT_URL` or derived public callback URL.
  - Optional allowed Portflare callback origins/URLs for browser callbacks.

## Trust Boundaries
- CLI and browser are untrusted. They may provide state, redirect URI, code, and verifier, but never an auth-service URL.
- Portflare server trusts only `PORTFLARE_AUTH_URL` and only after backend verification succeeds with `PORTFLARE_AUTH_SHARED_SECRET` HMAC.
- Auth service trusts only signed internal verify requests from the Portflare server; public callbacks only mint short-lived one-time codes.
- Provider identity is trusted only after Google token signature, issuer, audience, expiry, nonce, and email verification checks.
- Existing `X-Auth-Request-*` headers are trusted only if `PORTFLARE_TRUST_AUTH_HEADERS=true`; they take precedence over server sessions for compatibility.
- Session cookies are bearer credentials for browser dashboard only; they must be HttpOnly, authenticated, expiring, and contain no API keys.
- Email is not an account-linking authority. Only provider+subject links identify an auth-service user.

## Test Plan
- Protocol unit tests for DTO JSON compatibility and validation helpers.
- Auth service unit/handler tests:
  - health endpoint
  - HMAC valid/invalid/stale/replay
  - loopback and browser redirect validation
  - PKCE success/failure
  - code expiry and single-use
  - Google provider token validation using mocks
  - CLI and browser flows cannot exchange each other’s codes
- Server unit/handler tests:
  - auth config validation
  - auth health cache and timeout behavior
  - `/api/register/start` modes and required/unhealthy policy
  - `/api/register/exchange` success/rejection
  - legacy `/api/register` fallback gating
  - state migration with `external_identities`
  - provider+subject mapping, email-only conflict, registration closed
  - identity resolution precedence and `PORTFLARE_TRUST_AUTH_HEADERS=false`
  - session cookie signing/tamper/expiry
  - `/auth/start`, `/auth/callback`, `/auth/logout`
  - browser HTML redirect vs JSON API 401 metadata vs websocket no-redirect
- Client tests:
  - register direct fallback still works
  - auth start URL printed
  - browser opener failure is non-fatal
  - callback listener validates state and accepts once
  - timeout and interrupt cleanup
  - exchange success/failure output
- Integration/manual tests:
  - happy-path CLI registration with fake auth service
  - happy-path browser login with fake auth service
  - Google OAuth smoke test in a staging environment with real credentials
  - auth service unhealthy with `PORTFLARE_AUTH_REQUIRED=true` blocks registration; with false falls back only as designed.

## Validation Commands
- Go tests/builds:
  - `cd protocol && go test ./...`
  - `cd auth && go test ./... && go build ./cmd/portflare-auth`
  - `cd server && go test ./... && go build ./cmd/portflare-server`
  - `cd client && go test ./... && go build ./cmd/portflare`
- Docker builds:
  - `docker build -t portflare-auth:dev ./auth`
  - `docker build -t portflare-server:dev ./server`
  - `docker build -t portflare-client:dev ./client`
- Compose validation:
  - `docker compose -f s41nn0n-homelab-portflare/docker-compose.yml config`
  - If auth service is added to compose: `docker compose -f s41nn0n-homelab-portflare/docker-compose.yml up --build portflare portflare-auth`
- Local smoke with fake/stub auth service before real Google:
  - Start server with `PORTFLARE_AUTH_URL=http://127.0.0.1:<auth-port>` and shared secret.
  - Run `portflare register --server http://127.0.0.1:8080 --no-browser` and manually hit the printed callback URL from the fake auth flow.
  - Visit `http://127.0.0.1:8080/me` and verify redirect/session/callback behavior.

## Dependencies
- Commit 1 must happen before server/client/auth code imports shared DTOs.
- Commit 2 must happen before internal verify endpoints and server verify clients.
- Commit 3 must happen before full Phase 1 server/client integration can be exercised end-to-end.
- Commit 4 depends on Commit 1 and Commit 2; it can use fake auth-service tests before Commit 3 is complete.
- Commit 5 depends on Commit 4 endpoint contracts.
- Commit 6 depends on Commit 4 external identity helpers and auth client.
- Commit 7 depends on Commit 2 and shares provider/store code from Commit 3.
- Commit 8 depends on Commit 6 routes/session helpers and Commit 7 browser verify endpoint contract.
- Commit 9 should be last after route names/env vars are finalized.

## Risks
- Account-linking policy must be explicit before implementation. Recommended: provider+subject match returns existing user/API key; email-only match conflicts; no implicit takeover.
- Current server is a large `main.go`; adding this inline will be hard to review. Prefer separate files in package `main` for auth client, registration, sessions, browser auth, and identity.
- HMAC replay protection requires in-memory nonce storage; multi-instance auth service deployments need shared nonce/code storage or sticky routing. Document single-instance limitation if not solving now.
- In-memory auth transactions mean auth-service or server restarts invalidate pending login/registration attempts.
- Google OAuth implementation can be delayed by credential/callback setup; use mocked provider tests and fake provider for integration until real credentials are available.
- Health-check fallback policy is security-sensitive. Production should set `PORTFLARE_AUTH_REQUIRED=true`; otherwise unhealthy auth can allow direct registration fallback.
- `PORTFLARE_SESSION_SECRET` validation may break existing tests if enforced too broadly. Enforce only when browser auth is enabled/configured and provide test defaults.
- Browser route classification can break dashboard JS/websocket behavior. Validate `/me`, `/admin`, `/api/me/*`, `/api/admin/*`, and `/ws/ui` separately.
- Cookie `Secure` defaults need local development handling; secure cookies will not work over plain `http://127.0.0.1`.
- Current `/me` may expose the API key in dashboard HTML. Phase 2 increases browser session use; review whether API key display/rotation forms need CSRF protection before production enablement.

## Open Questions
1. Should repeated auth-backed CLI registration return the existing `APIKey`, rotate it, or fail with conflict? Plan assumes provider+subject returns existing key to match current single-key model.
2. Should direct `/api/register` be disabled whenever `PORTFLARE_AUTH_URL` is configured, even if auth is unhealthy and `PORTFLARE_AUTH_REQUIRED=false`? Plan follows docs: fallback allowed when not required but logs warning.
3. Are HMAC-signed backend requests definitively chosen over bearer secrets/JWT/mTLS? Plan assumes HMAC as recommended.
4. What exact public Google callback URLs will be registered for CLI and browser flows? Route names should be finalized before Google credential setup.
5. Should `PORTFLARE_TRUST_AUTH_HEADERS` default remain true for backwards compatibility? Plan assumes yes but implements false as an actual disable.
6. Should browser sessions be signed-only cookies or encrypted cookies/server-side sessions? Plan assumes signed-only minimal claims; do not include API keys.
7. Which `/api/me/*` and `/api/admin/*` endpoints should return JSON auth metadata versus redirect? Plan assumes JSON 401 metadata for API routes.
8. Is the root repository expected to own `auth/` releases/images, or will `auth` become a separate repository like server/client/protocol?
