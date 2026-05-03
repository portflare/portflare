# Code Context

## Files Retrieved
1. `docs/auth-registration-phase-1.md` (lines 1-390) - proposed CLI auth-backed registration architecture, trust model, fallback policy, provider layout.
2. `docs/auth-dashboard-phase-2.md` (lines 1-351) - proposed dashboard/browser auth-service handoff, session cookie, identity resolution, route policy.
3. `docs/README.md` (lines 1-30) - confirms docs are top-level cross-repo hub; implementation lives under `server/`, `client/`, and new `auth/`.
4. `server/cmd/portflare-server/main.go` (lines 35-148, 449-651, 1430-1510, 1846-1862) - current server config/state/user/registration/auth-header logic and route table.
5. `client/cmd/portflare/main.go` (lines 52-186, 255-272) - current CLI registration and daemon config entrypoint.
6. `server/cmd/portflare-server/registration_test.go` (lines 1-111) - current direct registration behavior tests.
7. `server/cmd/portflare-server/main_test.go` (lines 1-150) - current identity, host matching, and user auto-provisioning tests.
8. `client/cmd/portflare/register_test.go` (lines 1-64) - current CLI registration tests.

## Key Code

Current server has no auth-service config yet; `Config` only contains server, registration, header-auth/dev-auth, and timeout settings:

```go
// server/cmd/portflare-server/main.go:35-54
type Config struct {
    ListenAddr string
    PublicBaseDomain string
    StatePath string
    AdminUsers map[string]struct{}
    RegistrationOpen bool
    TrustedProxyOnly bool
    DisableAuth bool
    LocalDevUser string
    LocalDevEmail string
    ...
}
```

User state currently has a single `APIKey` and no external identity links:

```go
// server/cmd/portflare-server/main.go:79-96
type State struct {
    Users map[string]*User `json:"users"`
    Apps map[string]*App `json:"apps"`
}
type User struct {
    UserName string `json:"user_name"`
    PublicUserLabel string `json:"public_user_label"`
    PublicUserAliases []string `json:"public_user_aliases,omitempty"`
    Email string `json:"email"`
    APIKey string `json:"api_key"`
}
```

Routes are currently all in `main.go`; only direct `/api/register` exists and protected UI/API routes call `requireIdentity` individually:

```go
// server/cmd/portflare-server/main.go:449-469
mux.HandleFunc("/api/register", s.handleRegister)
mux.HandleFunc("/connect", s.handleConnect)
mux.HandleFunc("/ws/ui", s.handleUIWebSocket)
mux.HandleFunc("/admin", s.handleAdminPage)
...
mux.HandleFunc("/me", s.handleUserPage)
...
```

Direct registration creates a new user and new `pf_` key, guarded only by `RegistrationOpen`, duplicate username, and public-label collision checks:

```go
// server/cmd/portflare-server/main.go:596-651
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
    ... decode RegistrationRequest ...
    userName := slug(req.UserName)
    publicUserLabel := userLabel(userName)
    ... validate, lock state ...
    if !s.state.RegistrationOpen { ... }
    if _, ok := s.state.Users[userName]; ok { ... }
    ... check label collision ...
    user := &User{UserName: userName, PublicUserLabel: publicUserLabel, Email: email, APIKey: newAPIKey(), ...}
    s.state.Users[userName] = user
    ...
    writeJSON(w, http.StatusCreated, RegistrationResponse{..., APIKey: user.APIKey})
}
```

Dashboard/browser auth today is strictly local-dev-or-header based:

```go
// server/cmd/portflare-server/main.go:1430-1461
func (s *Server) requireIdentity(w http.ResponseWriter, r *http.Request) (authIdentity, bool) {
    if s.cfg.DisableAuth { ... return local dev admin ... }
    rawUserName := strings.TrimSpace(r.Header.Get("X-Auth-Request-User"))
    id := authIdentity{UserName: rawUserName, PublicUserLabel: userLabel(rawUserName), Email: ...}
    id.IsAdmin = isAdmin(id.UserName, id.Email, s.cfg.AdminUsers)
    if id.UserName == "" { writeError(..., "missing X-Auth-Request-User header"); return ..., false }
    return id, true
}
```

`ensureUser` auto-creates dashboard users from trusted identity when registration is open, also without external identity links:

```go
// server/cmd/portflare-server/main.go:1463-1510
if user, ok := s.state.Users[identity.UserName]; ok { ... update email ...; return user, nil }
if !s.state.RegistrationOpen { return nil, errors.New("registration is closed") }
... validate/collision check ...
user := &User{UserName: identity.UserName, PublicUserLabel: identity.PublicUserLabel, Email: identity.Email, APIKey: newAPIKey(), ...}
s.state.Users[user.UserName] = user
```

Current CLI `register` requires `--user`, posts directly to `/api/register`, and prints the returned API key:

```go
// client/cmd/portflare/main.go:120-186
portflare register --server <url> --user <name> [--email <email>]
...
if serverURL == "" || userName == "" { printUsage(...); return 1 }
payload := {"user_name": userName, "email": email}
resp, err := http.Post(serverURL+"/api/register", ...)
...
fmt.Printf("export PORTFLARE_CLIENT_KEY=%q\n", registration.APIKey)
```

## Architecture

### Strengths

- Phase split is sound: Phase 1 isolates provider/OAuth complexity in a new `auth/` service while keeping Portflare server authoritative for users/API keys (`docs/auth-registration-phase-1.md:51-58`). Phase 2 reuses that trusted backend verification path for dashboard sessions (`docs/auth-dashboard-phase-2.md:198-218`).
- The docs correctly pin trust to server-side `PORTFLARE_AUTH_URL` and forbid client/browser-supplied auth-service URLs (`docs/auth-registration-phase-1.md:225-246`, `docs/auth-dashboard-phase-2.md:198-218`). This matches the current risk surface because the CLI currently talks directly to whatever `--server` points at, and the server has no auth-service state.
- The redirect-code exchange avoids putting the Portflare API key in browser URLs (`docs/auth-registration-phase-1.md:143-149`, `307-316`), which is important because current CLI output includes the long-lived key only after direct server response.
- Phase 2 identity-resolution order preserves existing `X-Auth-Request-*` deployments and local dev mode (`docs/auth-dashboard-phase-2.md:135-169`), matching current `requireIdentity` behavior.
- The proposed route exclusions are consistent with current public/control-plane routes: `/healthz`, `/connect`, public app proxy routes, and registration endpoints should not trigger browser login (`docs/auth-dashboard-phase-2.md:288-305`).

### Gaps / risks

- Current server is a single large `main.go`; adding auth clients, transaction stores, HMAC, cookie sessions, and provider identity mapping inline will make it hard to test and review. Strong recommendation: carve out server internals (`internal/authclient`, `internal/session`, or at minimum separate files in package main) even if the binary remains unchanged.
- Phase 1 says provider+subject should avoid unsafe linking (`docs/auth-registration-phase-1.md:336-344`), and Phase 2 explicitly requires `ExternalIdentities` (`docs/auth-dashboard-phase-2.md:250-274`), but current `User` has no place for it. If Phase 1 ships without identity links, repeated auth registration can only conflict by suggested username/email or mint/return keys unsafely. Decide whether external identity state is a Phase 1 prerequisite.
- The current direct `/api/register` will remain open whenever `RegistrationOpen` is true. Phase 1 fallback policy requires disabling or gating direct fallback when auth service is configured/healthy/required (`docs/auth-registration-phase-1.md:318-334`). Implementation must alter `handleRegister` or wrap the route; adding `/api/register/start` alone is not enough.
- Health detection is underspecified operationally. If each `/api/register/start` or `requireIdentity` synchronously probes auth health, a slow auth service can block protected UI requests. Needs caching/timeouts/fail-open/fail-closed semantics.
- HMAC request signing is recommended (`docs/auth-registration-phase-1.md:257-269`) but requires nonce replay storage on the auth service and clock-skew handling. Bearer secret is simpler but weaker; choose before implementation because it affects both Phase 1 and Phase 2 verification clients.
- CLI localhost callback needs careful port/listener lifecycle and timeout behavior. Current CLI is simple synchronous `http.Post`; tests capture stdout only. The callback path will need deterministic tests around state mismatch, double callback, exchange failure, and listener cleanup.
- Browser sessions create a new trusted identity source. Current `authIdentity` only has user/email/admin; it lacks provider/subject. If session cookie stores provider/subject but downstream code ignores it, account mapping may silently degrade to username/email behavior.
- `TrustedProxyOnly`/`PORTFLARE_TRUST_AUTH_HEADERS` exists in config (`server/cmd/portflare-server/main.go:44,66`) but is not used in current `requireIdentity`; Phase 2 should clarify whether headers are always trusted, disabled, or only trusted behind proxies.
- Current dashboard templates expose the API key on `/me`; Phase 2 sessions make browser auth first-class, so cookie/session CSRF boundaries matter for key rotation and label update forms. Existing POST forms have no CSRF token.
- `wantsJSON` only checks `Accept: application/json` (`server/cmd/portflare-server/main.go:1846-1848`). Phase 2 API-vs-browser redirect behavior may need stronger route/method classification because browser fetches set JSON Accept but HTML navigations may use broad Accept values.

### Implementation constraints

- Registration and identity state is JSON file-backed (`StatePath`) with in-memory lock and whole-file save. Adding `ExternalIdentities` is straightforward as an optional JSON field, but migration/collision handling must preserve existing state files.
- Existing user and public-label normalization is strict: username uses `slug`, public label uses `userLabel`, min length 3, reserved labels include `admin`, `api`, `www`, `static`, `assets`, `me`. Auth-derived `suggested_user_name`/display name must pass these checks or produce a clear conflict.
- `ensureUser` currently auto-provisions from header auth when registration is open. Phase 2 account creation should probably share a refactored helper with `handleRegister` and auth exchange to avoid three subtly different user-creation paths.
- Current tests instantiate `Server` directly without `newServer` in places; adding required config validation (e.g., session secret) must not break unit tests unexpectedly. Prefer validation in `newServer` and helpers that tests can opt into.
- Client and server both define their own `RegistrationResponse`; new start/exchange DTOs can either duplicate in each binary or move to `protocol` if cross-repo stability is desired. Docs/README present `protocol` as shared types repo (`docs/README.md:16-20`).

## Likely files/modules to touch

- `server/cmd/portflare-server/main.go`: config fields/env parsing, route table, direct registration gating/refactor, identity resolution, user state structs, auth callback routes if not split.
- New server files under `server/cmd/portflare-server/` or `server/internal/...`: auth-service client, HMAC signer, auth health cache, CLI/browser transaction store, session cookie helpers, external identity mapping helpers.
- `server/cmd/portflare-server/registration_test.go`: fallback/auth-required/direct registration tests.
- `server/cmd/portflare-server/main_test.go` plus likely new auth/session tests: header precedence, missing header redirect/JSON behavior, callback validation, account linking.
- `client/cmd/portflare/main.go`: `register` flow, flags/usage, PKCE/state generation, localhost callback server, browser-open/print URL, exchange request.
- `client/cmd/portflare/register_test.go`: start-mode auth/direct fallback tests, callback state mismatch, exchange success/failure.
- New `auth/` module: `auth/go.mod`, `auth/cmd/portflare-auth/main.go`, `auth/internal/config`, `auth/internal/providers`, `auth/internal/providers/google`, `auth/internal/store`, `auth/internal/handoff` per Phase 1 doc (`docs/auth-registration-phase-1.md:377-389`).
- Possibly `protocol/`: shared request/response structs for `/api/register/start`, `/api/register/exchange`, identity claims, and verification payloads if multi-repo compatibility is desired.

## Specific questions / decisions needed

1. Should `User.ExternalIdentities` be introduced in Phase 1, or deferred to Phase 2? Deferring makes safe repeated CLI auth/account resolution difficult.
2. What is the exact policy for existing users: provider+subject match returns existing key, rotates key, creates additional key, or conflicts? Current `User` supports only one API key.
3. Should direct `/api/register` be disabled whenever `PORTFLARE_AUTH_URL` is configured, or only when auth is healthy/required? How should unhealthy auth behave in production defaults?
4. Choose backend auth mechanism now: HMAC (recommended but more code/state), bearer shared secret (simpler), JWT, or mTLS.
5. What are exact auth-service route names and callback URLs? Docs mention `/cli/start`, `/auth/google/start`, `/auth/google/callback`, `/browser/start`, `/browser/callback/google`; implementation needs one coherent route map.
6. What health-check endpoint and cache TTL should server use for auth-service availability?
7. How should auth-derived usernames be generated when Google display/email yields a reserved/too-short/colliding label?
8. Should browser session cookie be signed-only with embedded claims or server-side sessions? What TTL and `Secure` behavior for local HTTP dev?
9. For Phase 2, which `/api/me/*` and `/api/admin/*` endpoints return JSON auth metadata vs redirect? Existing dashboard JS fetches JSON and websocket `/ws/ui` starts immediately.
10. Is `PORTFLARE_TRUST_AUTH_HEADERS` intended to actually disable header trust? If yes, Phase 2 should implement it while preserving compatibility defaults.

## Start Here

Start with `server/cmd/portflare-server/main.go` around `Config`, `routes`, `handleRegister`, `requireIdentity`, and `ensureUser`. Those are the existing choke points for Phase 1 registration fallback/exchange and Phase 2 dashboard identity/session behavior.
