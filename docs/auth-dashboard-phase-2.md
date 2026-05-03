# Phase 2: Server auth-service handoff for dashboard/browser auth

## Goal

Refactor `portflare-server` so browser/dashboard requests can use the configured `portflare-auth` service when trusted `X-Auth-Request-*` headers are not present.

Today the server expects an upstream auth proxy to inject headers such as:

```text
X-Auth-Request-User
X-Auth-Request-Email
```

In Phase 2, if these headers are missing and an auth service is configured/running, the server should redirect the browser into the auth-service login flow and complete a secure handoff back to the server.

## Desired behavior

### Existing header-auth behavior remains supported

If trusted auth headers are present, the server keeps using them:

```text
X-Auth-Request-User: alice
X-Auth-Request-Email: alice@example.com
```

This preserves compatibility with Caddy, oauth2-proxy, Authelia, Cloudflare Access, etc.

### Auth-service fallback when headers are missing

When all of the following are true:

- request is for a browser/dashboard route
- no valid trusted auth headers are present
- `PORTFLARE_AUTH_URL` is configured
- auth service health check succeeds

Then the server redirects to the auth service instead of returning `401 missing X-Auth-Request-User header`.

Example:

```http
GET /me
```

Response:

```http
302 Location: https://auth.example.com/browser/start?...
```

### API behavior should remain non-surprising

For JSON/API requests, the server should not blindly redirect unless the endpoint is explicitly browser-oriented.

Recommended behavior:

- browser HTML routes: `302` to auth service
- API routes with `Accept: application/json`: `401` JSON with auth URL metadata
- websocket routes: `401` JSON/error unless a valid server session exists

Example API response:

```json
{
  "error": "authentication required",
  "auth_url": "https://auth.example.com/browser/start?..."
}
```

## High-level browser flow

```text
Browser -> portflare-server protected route
        -> missing X headers
        -> server creates auth handoff transaction
        -> browser redirected to portflare-auth
        -> auth service authenticates user via Google/provider
        -> auth service redirects back to server callback
        -> server verifies callback with auth service
        -> server creates signed session cookie
        -> browser redirected to original URL
```

## Server routes to add

Suggested routes:

```text
GET  /auth/start
GET  /auth/callback
POST /auth/logout
```

Or internal-only helpers invoked by `requireIdentity`/middleware.

### `/auth/start`

Starts a browser auth transaction.

Inputs:

- desired return URL, constrained to same-origin relative paths
- generated state
- generated nonce
- PKCE challenge or signed transaction ID

Redirects to auth service:

```text
https://auth.example.com/browser/start?state=...&redirect_uri=https://server.example.com/auth/callback&return_to=/me
```

### `/auth/callback`

Receives a one-time auth code from the auth service:

```text
/auth/callback?code=...&state=...
```

Server validates:

- state exists and matches transaction
- transaction is unexpired
- return URL is safe/relative
- auth code is present

Then server verifies the code with the configured auth service over the backend trusted channel.

### `/auth/logout`

Clears server session cookie. Later it may also redirect to auth-service logout/provider logout.

## Identity resolution order

Refactor identity resolution into a layered flow:

1. local dev identity when `PORTFLARE_DISABLE_AUTH=true`
2. existing trusted auth headers, if present and valid
3. server session cookie created from auth-service callback
4. auth-service browser handoff when configured/healthy and request can redirect
5. `401`/`403` fallback

Pseudo-code:

```go
func (s *Server) requireIdentity(w http.ResponseWriter, r *http.Request) (authIdentity, bool) {
    if s.cfg.DisableAuth {
        return s.localDevIdentity(w, r)
    }

    if id, ok := s.identityFromTrustedHeaders(r); ok {
        return id, true
    }

    if id, ok := s.identityFromSessionCookie(r); ok {
        return id, true
    }

    if s.authServiceAvailable() {
        s.beginBrowserAuth(w, r)
        return authIdentity{}, false
    }

    writeError(w, http.StatusUnauthorized, "missing X-Auth-Request-User header")
    return authIdentity{}, false
}
```

## Session cookie requirements

Once the server verifies a successful auth-service browser callback, it should issue its own server session cookie.

Cookie requirements:

- `HttpOnly`
- `Secure` in production
- `SameSite=Lax`
- short-ish TTL, e.g. 8–24 hours
- signed and authenticated, not plaintext-trusted
- contains minimal identity claims, or references server-side session storage

Recommended Phase 2 implementation:

- signed encrypted cookie or HMAC-signed cookie
- identity fields:
  - `user_name`
  - `email`
  - `provider`
  - `subject`
  - `issued_at`
  - `expires_at`
- signed with `PORTFLARE_SESSION_SECRET`

If `PORTFLARE_SESSION_SECRET` is missing while auth-service browser auth is enabled, server should fail startup or disable browser handoff with a clear error.

## Auth-service trust requirements

The server must only trust the configured auth service.

Configuration:

```bash
PORTFLARE_AUTH_URL=https://auth.example.com
PORTFLARE_AUTH_SHARED_SECRET=...
PORTFLARE_SESSION_SECRET=...
```

The server must not accept auth-service URLs from:

- query parameters
- headers
- cookies
- browser redirects
- CLI input

Backend verification must use the same trusted mechanism from Phase 1, preferably HMAC-signed requests with timestamp and nonce.

## Browser callback verification

The server callback should exchange/verify the auth code by calling:

```http
POST /internal/browser/verify
Authorization/signature: ...
Content-Type: application/json

{
  "code": "...",
  "redirect_uri": "https://server.example.com/auth/callback"
}
```

The auth service returns verified identity claims:

```json
{
  "provider": "google",
  "subject": "google-sub",
  "email": "alice@example.com",
  "email_verified": true,
  "display_name": "Alice Smith",
  "suggested_user_name": "alice"
}
```

The server then maps this identity to a Portflare user.

## User mapping and account creation

Recommended Phase 2 policy:

- If provider+subject is already linked to a Portflare user, load that user.
- If no link exists and registration is open, create a user from the verified identity.
- If no link exists and registration is closed, return `403` with a clear message.
- Do not automatically take over an existing user solely because the email matches, unless explicit safe linking exists.

This likely requires extending `User` state with external identity links:

```go
type UserExternalIdentity struct {
    Provider string `json:"provider"`
    Subject  string `json:"subject"`
    Email    string `json:"email,omitempty"`
    LinkedAt time.Time `json:"linked_at"`
}
```

And adding to `User`:

```go
ExternalIdentities []UserExternalIdentity `json:"external_identities,omitempty"`
```

## Auth-service routes needed for Phase 2

The auth service should support browser login handoff separately from CLI registration:

```text
GET  /browser/start
GET  /browser/callback/google
POST /internal/browser/verify
```

This keeps CLI and browser flows separate while sharing providers, transaction storage, and backend verification helpers.

## Protected route handling

Routes that should trigger browser handoff when accessed by a browser:

- `/me`
- `/admin`
- `/api/me/state` if HTML/browser fetch accepts redirect/session behavior
- `/api/admin/state` if HTML/browser fetch accepts redirect/session behavior
- `/ws/ui` only after a session already exists; otherwise fail cleanly

Routes that should not trigger browser handoff:

- `/healthz`
- `/connect`
- `/api/register/start`
- `/api/register/exchange`
- `/api/register` fallback endpoint
- public app proxy routes

## Security checklist

- [ ] Do not trust arbitrary auth-service URLs.
- [ ] Redirect only to configured `PORTFLARE_AUTH_URL`.
- [ ] Validate callback state and expiry.
- [ ] Validate return URL is same-origin relative path.
- [ ] Authenticate server-to-auth verification calls.
- [ ] Use single-use auth codes.
- [ ] Issue `HttpOnly`, `Secure`, `SameSite=Lax` session cookies.
- [ ] Require `PORTFLARE_SESSION_SECRET` for server-managed sessions.
- [ ] Avoid putting API keys or session material in URLs.
- [ ] Avoid implicit email-only account takeover.
- [ ] Keep direct header auth as first-class supported mode.

## Implementation checklist

### Server

- [ ] Add auth-service browser config fields.
- [ ] Add session secret config and validation.
- [ ] Extract identity resolution into helper methods.
- [ ] Add signed session cookie helpers.
- [ ] Add browser auth transaction store.
- [ ] Add `/auth/start`, `/auth/callback`, `/auth/logout`.
- [ ] Add backend verification client for `/internal/browser/verify`.
- [ ] Add external identity mapping to `User` state.
- [ ] Refactor `requireIdentity` to use header/session/auth-service fallback order.
- [ ] Add tests for header precedence, session auth, redirect behavior, API non-redirect behavior, callback validation, and account creation/linking.

### Auth service

- [ ] Add browser transaction support.
- [ ] Add `/browser/start`.
- [ ] Add Google browser callback.
- [ ] Add `/internal/browser/verify`.
- [ ] Share provider abstraction with CLI registration flow.
- [ ] Add tests for redirect validation, state handling, code verification, and single-use behavior.

## Open decisions

- Session TTL default.
- Whether `/api/me/*` should redirect or return JSON auth metadata when called by browser JS.
- Whether server sessions are cookie-only or backed by server-side session state.
- Exact external identity state migration behavior for existing users.
- Whether auth-service health failure should fall back to header-only auth or hard fail when configured.
