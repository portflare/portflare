# Client TUI + GUI Scout Report

> Status: To be completed. This reconnaissance report supports the upcoming client refactor, TUI, GUI, and approval tooling work.

# Code Context

## Files Retrieved
1. `client/cmd/portflare/main.go` (lines 1-260) - current monolithic client entrypoint, config, CLI command parsing, direct registration, and daemon setup.
2. `client/cmd/portflare/main.go` (lines 260-590) - daemon startup, local API routes, app CRUD, stats, discovery rescan.
3. `client/cmd/portflare/main.go` (lines 740-910) - server websocket lifecycle, register-ack handling, proxy request handling.
4. `client/cmd/portflare/main_test.go` (lines 1-220) - current client tests for persistence, local API app grouping/delete, discovery, validation.
5. `client/cmd/portflare/register_test.go` (lines 1-77) - current direct registration CLI tests.
6. `client/README.md` (lines 1-74) - current documented client install/run/register/expose/discovery flow.
7. `client/docs/usage.md` (lines 1-154) - current CLI/local API/discovery usage and Docker sidecar constraints.
8. `server/cmd/portflare-server/main.go` (lines 138-147, 449-469, 596-681) - registration structs/routes and direct registration endpoint.
9. `server/cmd/portflare-server/main.go` (lines 770-890, 1020-1290) - admin/user state, user app state, approve/rotate/label update APIs.
10. `server/cmd/portflare-server/main.go` (lines 1838-1848) - bearer token helper and JSON response detection.
11. `protocol/types/types.go` (lines 1-58) - shared app registration and websocket message types.
12. `client/go.mod` (lines 1-8) - client currently only depends on gorilla/websocket and protocol.
13. `docs/auth-registration-phase-1.md` (lines 1-160) - planned auth-backed CLI registration and browser handoff primitives.
14. `docs/auth-dashboard-phase-2.md` (lines 1-180) - planned auth-service dashboard session handoff and API-vs-browser auth behavior.
15. `docs/auth-handoff-phase-plan.md` (lines 1-220) - implementation plan touching CLI registration, server auth APIs, and browser handoff.
16. `docs/auth-handoff-scout-review.md` (lines 1-160) - prior architecture review highlighting current auth/API limitations.

## Key Code

### Current client is a single `main.go` with CLI, daemon, local API, discovery, tunnel, and stats mixed together

```go
// client/cmd/portflare/main.go:27-44
type Config struct {
    ServerURL string
    ClientKey string
    LocalAPIAddr string
    StatePath string
    ReconnectDelay time.Duration
    HTTPTimeout time.Duration
    DiscoverEnabled bool
    DiscoverInterval time.Duration
    DiscoverGrace time.Duration
    DiscoverAllow []portRange
    DiscoverDeny []portRange
    DiscoverNameByPort map[int]string
}
```

```go
// client/cmd/portflare/main.go:76-113
func main() {
    args := os.Args[1:]
    ...
    if len(args) > 0 && args[0] != "daemon" {
        os.Exit(runCLI(args))
    }
    runDaemon()
}

func printUsage(w io.Writer) {
    fmt.Fprintln(w, "  portflare daemon")
    fmt.Fprintln(w, "  portflare register --server <url> --user <name> [--email <email>]")
    fmt.Fprintln(w, "  portflare expose --app <name> --target <url> [--public-port <port>]")
    fmt.Fprintln(w, "  portflare list")
    fmt.Fprintln(w, "  portflare version")
}
```

`runCLI` is hand-rolled flag parsing and currently only supports direct register, expose, list, version, help. There is no command abstraction, no reusable API client, and no interactive mode entrypoint yet.

### Current CLI `register` posts directly to the server; auth-backed flow is planned but not wired here yet

```go
// client/cmd/portflare/main.go:120-186
case "register":
    serverURL := strings.TrimRight(env("PORTFLARE_SERVER_URL", "http://host.docker.internal:8080"), "/")
    userName := ""
    email := ""
    ... parse --server/--user/--email ...
    if serverURL == "" || userName == "" { printUsage(...); return 1 }
    payload, _ := json.Marshal(map[string]string{"user_name": userName, "email": email})
    resp, err := http.Post(serverURL+"/api/register", "application/json", bytes.NewReader(payload))
    ...
    fmt.Printf("export PORTFLARE_SERVER_URL=%q\n", serverURL)
    fmt.Printf("export PORTFLARE_CLIENT_KEY=%q\n", registration.APIKey)
```

This matters for TUI/GUI because registration/auth flow should be extracted into a reusable package/flow that can be driven by CLI, TUI, and GUI.

### Local client API already provides local app CRUD and stats, but only local state

```go
// client/cmd/portflare/main.go:360-382
func (s *Service) serveLocalAPI(ctx context.Context) error {
    mux := http.NewServeMux()
    mux.HandleFunc("/apps", s.handleApps)
    mux.HandleFunc("/apps/", s.handleAppByName)
    mux.HandleFunc("/stats", s.handleStats)
    mux.HandleFunc("/discovery/rescan", s.handleDiscoveryRescan)
    mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
        writeJSON(w, http.StatusOK, map[string]any{"ok": true})
    })
    ...
}
```

`GET /apps` returns local apps grouped by manual/discovery plus `connected` and `user`; `POST /apps` adds/updates a local app and sends a websocket `register` message if connected; `DELETE /apps/{name}` removes a local app. There are no local routes for server-side app approval, server-side user state, server-side traffic stats, config editing, logs/events, or auth/session state.

```go
// client/cmd/portflare/main.go:388-475
case http.MethodGet:
    ... writeJSON(w, http.StatusOK, map[string]any{
        "connected": connected,
        "user": currentUser,
        "apps": out,
        "manual_apps": manual,
        "discovery_apps": discovery,
    })
case http.MethodPost:
    ... validate AppRegistration ...
    if conn != nil {
        _ = s.send(ConnectMessage{Type: protocoltypes.MessageTypeRegister, AppName: req.AppName, PublicPort: req.PublicPort})
    }
```

### App approval state currently reaches the client only through websocket register ack

```go
// client/cmd/portflare/main.go:806-833
case protocoltypes.MessageTypeRegisterAck:
    s.mu.Lock()
    if app, ok := s.apps[msg.AppName]; ok {
        app.Approved = msg.Approved
        if msg.PublicPort > 0 { app.PublicPort = msg.PublicPort }
        app.UpdatedAt = time.Now().UTC()
    }
    s.mu.Unlock()
    _ = s.saveState()
    s.logger.Info("app registration acknowledged", "app", msg.AppName, "approved", msg.Approved, "public_port", msg.PublicPort)
```

The client knows whether its locally registered apps are approved, but it does not know `can_approve`, real `public_url`, approval policy, or server-side traffic buckets unless it calls the server API.

### Server has user-facing app state and approval APIs, but they are dashboard-authenticated, not client-key authenticated

Route table:

```go
// server/cmd/portflare-server/main.go:449-469
mux.HandleFunc("/api/register", s.handleRegister)
mux.HandleFunc("/connect", s.handleConnect)
mux.HandleFunc("/ws/ui", s.handleUIWebSocket)
mux.HandleFunc("/admin", s.handleAdminPage)
mux.HandleFunc("/api/admin/state", s.handleAdminState)
mux.HandleFunc("/api/admin/traffic", s.handleAdminTraffic)
...
mux.HandleFunc("/me", s.handleUserPage)
mux.HandleFunc("/api/me/state", s.handleUserState)
mux.HandleFunc("/api/me/traffic", s.handleUserTraffic)
mux.HandleFunc("/api/me/approve", s.handleApproveApp)
mux.HandleFunc("/api/me/rotate-key", s.handleRotateKey)
```

User state response includes app approval capability:

```go
// server/cmd/portflare-server/main.go:1036-1074
canApprove := identity.IsAdmin || (allowUserAppApproval && !cp.Approved)
apps = append(apps, map[string]any{
    "app_name": cp.AppName,
    "approved": cp.Approved,
    "connected": cp.Connected,
    "public_port": cp.PublicPort,
    "public_url": fmt.Sprintf("https://%s-%s.%s", cp.AppName, user.PublicUserLabel, s.cfg.PublicBaseDomain),
    "status": ...,
    "can_approve": canApprove,
    "user_name": user.UserName,
})
```

Approval endpoint:

```go
// server/cmd/portflare-server/main.go:1199-1268
func (s *Server) handleApproveApp(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost { ... }
    identity, ok := s.requireIdentity(w, r)
    ...
    userName := slug(r.Form.Get("user"))
    appName := slug(r.Form.Get("app"))
    if !identity.IsAdmin {
        allow := s.state.AllowUserAppApproval
        if !allow || identity.UserName != userName {
            writeError(w, http.StatusForbidden, "approval not allowed")
            return
        }
    }
    app.Approved = true
    ...
    if wantsJSON(r) { writeJSON(w, http.StatusOK, map[string]any{"approved": true}); return }
}
```

Important gap: `requireIdentity` does not use `PORTFLARE_CLIENT_KEY`; `/api/me/state` and `/api/me/approve` require local dev auth, trusted `X-Auth-Request-*` headers, or future browser session auth. The client daemon uses the client key only for websocket `/connect` auth.

### Server direct registration and auth handoff are actively evolving

Direct registration exists:

```go
// server/cmd/portflare-server/main.go:596-651
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
    ... decode user_name/email ...
    if !s.state.RegistrationOpen { writeError(..., "registration is closed") }
    if _, ok := s.state.Users[userName]; ok { writeError(..., "user already exists") }
    ...
    user := &User{..., APIKey: newAPIKey()}
    s.state.Users[userName] = user
    writeJSON(w, http.StatusCreated, RegistrationResponse{..., APIKey: user.APIKey})
}
```

Planning docs define upcoming `/api/register/start` and `/api/register/exchange`, redirect + PKCE, and auth-service-backed browser sessions. A TUI/GUI spec should align with those rather than deepening direct-only registration.

### Protocol types are shared for tunnel/app registration, but not UI/state APIs

```go
// protocol/types/types.go:16-30
type AppRegistration struct {
    AppName string `json:"app_name"`
    TargetURL string `json:"target_url"`
    PublicPort int `json:"public_port,omitempty"`
    Approved bool `json:"approved"`
    Source string `json:"source,omitempty"`
    DiscoveredPort int `json:"discovered_port,omitempty"`
    Offline bool `json:"offline,omitempty"`
    ...
}
```

Server `/api/me/state` currently returns `map[string]any`, not protocol DTOs. For a polished TUI/GUI, shared response structs for local client state, server user state, approve responses, and traffic stats would reduce fragile map decoding.

## Architecture

### Current client architecture

The client binary currently combines these responsibilities in one `package main` file:

1. CLI parsing and command execution (`register`, `expose`, `list`, `version`).
2. Daemon config/env loading.
3. Local app state persistence to JSON (`PORTFLARE_CLIENT_STATE_PATH`).
4. Local HTTP API on `PORTFLARE_CLIENT_LISTEN_ADDR`.
5. Discovery scanner and discovered app lifecycle.
6. Websocket connection to server `/connect?key=...`.
7. Proxy request handling and local upstream HTTP forwarding.
8. Client-wide stats tracking/logging.

This is workable for a small CLI but will be painful for a TUI/GUI. The TUI/GUI needs a cleaner domain/API layer that can be used by multiple frontends without importing terminal or HTTP handler details.

Recommended refactor seams:

- `internal/config`: env/default loading and validation.
- `internal/state`: client state load/save and app mutation.
- `internal/localapi`: local HTTP server handlers and client-facing DTOs.
- `internal/serverapi`: server REST client for registration, user state, approval, traffic stats.
- `internal/tunnel`: websocket connect/register/proxy loop.
- `internal/discovery`: port scan/candidate normalization.
- `internal/stats`: counters and snapshots.
- `internal/ui/tui`: terminal UI.
- `internal/ui/web` or `internal/gui`: local browser GUI/static assets and/or native desktop GUI.
- `cmd/portflare`: thin command dispatch only.

### Current CLI/local API shape

Commands:

- `portflare daemon` - runs daemon, local API, discovery, stats logging, websocket loop.
- `portflare register --server <url> --user <name> [--email <email>]` - direct server registration only today.
- `portflare expose --app <name> --target <url> [--public-port <port>]` - POSTs to local daemon `/apps`.
- `portflare list` - GETs local daemon `/apps`.
- `portflare version`.

Local API:

- `GET /healthz` -> `{ok:true}`.
- `GET /apps?source=` -> connected/user + local app list/manual/discovery grouping.
- `POST /apps` -> add/update local app; sends websocket `register` if connected.
- `GET /apps/{name}` -> local app by name.
- `DELETE /apps/{name}` -> delete local app.
- `GET /stats` -> connected/user/app count/client stats.
- `POST /discovery/rescan` -> manual scan when discovery enabled.

Gaps for TUI/GUI:

- No local API endpoint for server user state (`/api/me/state`).
- No local API endpoint to approve an app.
- No local API endpoint for server traffic stats (`/api/me/traffic`).
- No event stream/websocket from daemon to UI; TUI/GUI must poll.
- No config introspection endpoint.
- No logs/recent events endpoint.
- No endpoint to rotate key/update user label from tool.
- Local `/apps` public URL example is hardcoded to `reverse.example.test`, not the configured server base domain.

### Server API capabilities for approving apps from the tool

Existing server capability:

- User page and `/api/me/state` expose `can_approve` and app status for the authenticated dashboard user.
- `/api/me/approve` can approve a user app if either:
  - identity is admin, or
  - server setting `AllowUserAppApproval` is true and identity user matches app user.
- JSON responses are supported via `Accept: application/json`.

Critical gap:

- These endpoints are authenticated by `requireIdentity`, not the client key. Current `requireIdentity` accepts local dev or auth headers/session (future), while client-key auth is only used by `/connect` through `findUserByKey`.
- A local TUI/GUI running on the client machine has `PORTFLARE_CLIENT_KEY`; it does not have `X-Auth-Request-*` headers. Therefore it cannot call `/api/me/state` or `/api/me/approve` today unless the user separately authenticates in browser and the tool can reuse a server session cookie.

Likely server-side solution options:

1. **Client-key scoped REST endpoints** (best fit for CLI/TUI approval): add endpoints such as `GET /api/client/me/state`, `POST /api/client/me/approve`, and `GET /api/client/me/traffic`, authenticated by `Authorization: Bearer pf_...`. They should map key -> user via existing `findUserByKey`, never allow arbitrary users, and apply the same `AllowUserAppApproval` policy. This lets the local tool approve the current user's apps without browser session complexity.
2. **Teach `/api/me/*` to accept Bearer client key** for API requests only. This is smaller but mixes browser identity/session semantics with client-key API semantics. Must be careful not to weaken admin/user boundaries.
3. **Browser auth handoff/session reuse**: TUI/GUI opens auth flow and stores a server session cookie. This aligns with Phase 2 but is more complex and unnecessary if the tool already has a scoped client key.

For the user's requirement "approve their applications from this tool not just UI", option 1 is the cleanest and safest initial spec.

### TUI/GUI product direction

A detailed spec should decide whether "GUI" means:

- **Local web GUI** hosted by the daemon, e.g. `http://127.0.0.1:9901/ui` with static assets. This is easiest to ship cross-platform and can reuse local API routes.
- **Native desktop GUI** using Wails/Fyne. This is more polished but adds build/release complexity.
- **Terminal UI** using Bubble Tea/Lip Gloss. This fits the Go CLI and operational/server environments.

Recommended phased UX:

1. Refactor core client/domain/API first.
2. Add TUI command: `portflare tui` or `portflare dashboard`.
3. Add local web GUI hosted by daemon: `GET /ui`, with `portflare gui` opening browser to the local UI. Always print URL for headless environments, like auth flow.
4. Consider Wails/native desktop later if product needs packaged desktop app.

Suggested tool capabilities:

- Connection overview: server URL, connected user, websocket status, reconnect state, uptime.
- Apps table: app name, target URL, source, discovered port, offline, approved, connected, public port, public URL, status.
- Actions: expose app, edit target/public port, delete app, rescan discovery, approve app if `can_approve`, open public URL, copy URL.
- Stats: client request totals, bytes in/out, last request, server-side per-app traffic buckets if available.
- Registration/auth: register/login flow status; show env exports; do not auto-load `.env`.
- Diagnostics: local API health, server health, key format, last websocket error, recent events/logs.

## Start Here

Start with `client/cmd/portflare/main.go` and split it along frontend-independent seams before adding UI code. The first implementation spec should define typed interfaces for local app state, server API access, and frontend view models. The server approval capability should be designed next because the current server API cannot be used by the client tool with only `PORTFLARE_CLIENT_KEY`.

## Refactor seams and likely files to touch

### Client files likely to change/add

Existing:

- `client/cmd/portflare/main.go` - split into thin command entrypoint plus imported internal packages.
- `client/cmd/portflare/main_test.go` - move tests alongside extracted packages.
- `client/cmd/portflare/register_test.go` - update when registration flow becomes shared between CLI/TUI/GUI.
- `client/README.md`, `client/docs/usage.md` - document `tui`, `gui`, approval from tool.
- `client/go.mod` - add TUI/GUI dependencies if chosen.

Likely new:

- `client/internal/config/config.go`
- `client/internal/appstate/state.go`
- `client/internal/localapi/server.go`
- `client/internal/serverapi/client.go`
- `client/internal/tunnel/service.go`
- `client/internal/discovery/discovery.go`
- `client/internal/stats/stats.go`
- `client/internal/ui/viewmodel.go`
- `client/internal/ui/tui/...`
- `client/internal/ui/web/...` or `client/web/` static assets
- `client/cmd/portflare/root.go`, `daemon.go`, `register.go`, `expose.go`, `tui.go`, `gui.go` if staying without Cobra; or a Cobra-based command tree if accepted.

### Server files likely to change/add

Existing:

- `server/cmd/portflare-server/main.go` - route table, auth helpers, user state, app approval handler.
- `server/cmd/portflare-server/main_test.go`, `registration_test.go`, `traffic_test.go` - add approval API coverage.

Likely new or split:

- `server/cmd/portflare-server/client_api.go` - client-key authenticated `/api/client/me/*` endpoints.
- `server/cmd/portflare-server/client_api_test.go` - tests for key auth, user scoping, approval policy, no admin escalation.
- `protocol/types/...` - shared DTOs for user/app state and approval responses if avoiding map decoding.

### Protocol files likely to touch

- `protocol/types/types.go` or new `protocol/types/client_api.go` for shared app/user state DTOs.
- Existing `AppRegistration` is client-local and websocket-facing; server state app DTO should probably be separate because it includes `public_url`, `can_approve`, `status`, `connected`, user label/user name.

## Risks and constraints

1. **Auth mismatch for approvals**: current server approval APIs are browser-authenticated; client tool has client key. Do not hack around this by asking users to paste auth headers. Add a scoped client-key API or implement Phase 2 browser session integration first.
2. **Permission semantics**: tool approval must respect server policy. If `AllowUserAppApproval=false`, non-admin client-key approval should fail unless product decision changes.
3. **Client key exposure**: a local web GUI must not expose `PORTFLARE_CLIENT_KEY` to arbitrary websites. Bind local API to `127.0.0.1` by default, consider CSRF/origin checks for mutating local API endpoints, and avoid rendering the key in the GUI.
4. **Local API has no auth**: current local API is unauthenticated. This is okay-ish on loopback but risky if `PORTFLARE_CLIENT_LISTEN_ADDR=0.0.0.0`. GUI/TUI spec should require local API auth token or strict loopback/origin protections before adding powerful actions like server approval/key rotation.
5. **Monolithic `main.go` increases regression risk**: TDD refactor should extract behavior with tests before adding UI dependencies.
6. **Cross-platform browser/open behavior**: GUI command should try to open browser but always print the local URL, mirroring auth docs.
7. **TUI dependency/build size**: Bubble Tea/Lip Gloss are common for Go TUI; adding them to client is acceptable but should be deliberate. Native GUI dependencies may complicate Docker/minimal images.
8. **Eventing/polling**: TUI/GUI can initially poll `/apps` and `/stats`, but a nicer interface needs events. Consider local `/events` SSE stream for app ack, connection changes, request stats, discovery changes.
9. **Public URL accuracy**: client local app response currently uses `reverse.example.test`; TUI/GUI needs server base domain from server state or config.
10. **Tests use Docker for Go tooling in this environment**: local host lacks Go in prior work; validation should use Docker `golang:1.23`.
11. **Registration/auth work is in progress**: avoid designing TUI registration around only direct `/api/register`; spec should support upcoming `/api/register/start` auth-backed flow.
12. **Single-key user model**: approving via client key implies the key acts as an API credential for user-scoped control-plane actions. That should be explicit and tested.

## Open questions for the spec/planner

1. Should "GUI" be local web GUI first, native desktop GUI, or both? Recommendation: local web GUI first; native later.
2. Should TUI connect to the local daemon API only, or can it run embedded in-process when daemon is not running? Recommendation: TUI can use local API first; later `portflare daemon --ui` can combine.
3. Should the daemon local API receive an auth token when binding non-loopback? Recommendation: yes before GUI adds powerful mutations.
4. Should client-key authenticated server APIs be new `/api/client/me/*` endpoints or should `/api/me/*` accept Bearer client keys? Recommendation: new `/api/client/me/*` endpoints.
5. Should users be able to approve apps only if server `AllowUserAppApproval=true`, or should possession of `PORTFLARE_CLIENT_KEY` always allow own-app approval? Current server policy says only when allowed; changing this is product/security decision.
6. Should the TUI/GUI manage discovered app approval individually or bulk approve pending apps? Bulk approve is useful but should be permission-gated.
7. Should logs/recent request events be kept in memory for UI display? Current stats are aggregate only.
8. Should server traffic stats be proxied through local daemon or called directly by TUI/GUI? Proxying through daemon centralizes client-key auth but increases local API scope.
9. Should command parsing move to Cobra or remain custom? Cobra helps with a growing command surface (`daemon`, `register`, `expose`, `list`, `tui`, `gui`, `approve`, `discovery rescan`) but adds dependency and migration work.
10. Should shared DTOs live in `protocol` now? Recommendation: yes for server/client API contracts that TUI/GUI consumes.

## Suggested next scout/planner focus

For the detailed spec, plan around three tracks:

1. **Refactor foundation**: extract client core packages and typed DTOs under test, without UI changes.
2. **Server control API**: add client-key authenticated user app state/approval endpoints with strict scoping and tests.
3. **Interfaces**: TUI and local web GUI consume the same local view model/API, with polling first and optional SSE events later.
