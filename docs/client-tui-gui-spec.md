# Client TUI + GUI Product and Technical Specification

> Status: To be completed. This is the approved planning/specification document for the upcoming client refactor, TUI, GUI, and tool-based app approval work.

## Goal

Refactor the Portflare client into a cleaner, testable architecture and add polished TUI and GUI experiences that help individuals and teams register, expose, monitor, and approve their applications without relying only on the server dashboard.

## Principles

1. **Daemon as product core**: `portflare daemon` owns tunnel connectivity, local app state, discovery, stats, local API, and server interactions.
2. **Interfaces are thin clients**: CLI, TUI, and GUI should consume shared client packages and/or the local daemon API instead of duplicating tunnel logic.
3. **TDD first**: each refactor or feature slice starts with failing tests, then implementation, then reviewer validation.
4. **Security by default**: approval and server actions must be server-authorized; the local UI must not widen access to the client key or privileged actions.
5. **Modern, professional UX**: TUI and GUI should feel intentionally designed, not just wrappers around JSON output.
6. **Headless friendly**: GUI/browser-open commands should always print URLs for SSH/server environments.
7. **Approval in the tool**: users must be able to approve their own applications from CLI/TUI/GUI when server policy allows it.

## Current State Summary

The current client is mostly implemented in `client/cmd/portflare/main.go`, combining:

- CLI parsing and commands
- daemon startup
- local API handlers
- state persistence
- discovery
- websocket tunnel lifecycle
- proxy handling
- stats/logging

Current commands:

```text
portflare daemon
portflare register --server <url> --user <name> [--email <email>]
portflare expose --app <name> --target <url> [--public-port <port>]
portflare list
portflare version
```

Current local API:

```text
GET    /healthz
GET    /apps
POST   /apps
GET    /apps/{name}
DELETE /apps/{name}
GET    /stats
POST   /discovery/rescan
```

Current server approval API exists only under browser/dashboard auth:

```text
GET  /api/me/state
POST /api/me/approve
GET  /api/me/traffic
```

The client has `PORTFLARE_CLIENT_KEY`, but `/api/me/*` currently does not accept that key. Therefore, a new server-side client-key scoped API is needed for approval from the client tool.

## Product Scope

### In scope

- Refactor client into internal packages.
- Add typed local API DTOs and local API client.
- Add server client-key scoped endpoints for user app state and approval.
- Add local daemon endpoints that proxy safe server-backed actions.
- Add `portflare tui` using Bubble Tea/Lip Gloss/Bubbles.
- Add `portflare gui` as a local web GUI served by the daemon.
- Add approval UX in CLI/TUI/GUI.
- Add tests for refactor, server approval API, local API, TUI models, GUI handlers, and security boundaries.
- Update docs.

### Out of scope for first implementation

- Native desktop app packaging with Wails/Fyne/Gio.
- Admin approval from the client tool unless explicitly authenticated as admin through a later flow.
- Full auth-service browser session reuse inside the client tool.
- Prometheus/SQLite/Postgres metrics backends.
- Multi-user local daemon access.

## Recommended UI Technology

### TUI

Use:

- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/lipgloss`
- `github.com/charmbracelet/bubbles`

Rationale:

- Go-native.
- Single binary.
- Good over SSH.
- Strong visual design ecosystem.
- Suitable for a modern operator dashboard.

### GUI

Implement the first GUI as a local web UI served by the daemon:

```text
http://127.0.0.1:9901/ui
```

Add:

```bash
portflare gui
```

Behavior:

- try to open the browser
- always print the local URL
- use same local API as TUI

Defer Wails/native desktop until local web UI workflows stabilize.

## Desired Commands

### Core commands

```text
portflare daemon
portflare register --server <url> [--user <name>] [--email <email>] [--no-browser]
portflare expose --app <name> --target <url> [--public-port <port>]
portflare list
portflare stats
portflare approve --app <name>
portflare discovery rescan
portflare tui
portflare gui [--no-browser]
portflare version
portflare help
```

### Command behavior

#### `portflare tui`

Starts a terminal dashboard.

Initial behavior:

- connects to local daemon API at `PORTFLARE_CLIENT_API`, default `http://127.0.0.1:9901`
- if daemon is unreachable, shows a friendly offline screen with:
  - daemon start command
  - configured API URL
  - retry keybinding

#### `portflare gui`

Opens the local web GUI.

Behavior:

```text
Portflare GUI: http://127.0.0.1:9901/ui
Opening browser...
```

If browser open fails:

```text
Could not open browser automatically. Open this URL manually:
http://127.0.0.1:9901/ui
```

#### `portflare approve --app <name>`

Approves a pending app owned by the current client-key user when server policy allows user approval.

Expected output:

```text
Approved app "web"
Public URL: https://web-alice.r.example.dev
```

If not allowed:

```text
Approval is not allowed by this server policy. Ask an administrator to approve this app.
```

## UX Specification

## TUI Experience

### Layout

```text
┌ Portflare ─ connected as alice ─ https://r.example.dev ─ 3 apps ─ 124 req ┐
│ Apps | Approvals | Traffic | Discovery | Logs | Settings                 │
├────────────────────────────────────────────────────────────────────────────┤
│                                                                            │
│ Main table/list                                                            │
│                                                                            │
├────────────────────────────────────────┬───────────────────────────────────┤
│ Status / hint bar                      │ Selected item details/actions     │
└────────────────────────────────────────┴───────────────────────────────────┘
```

### Global keybindings

```text
q          quit
?          help
r          refresh
tab        next tab
shift+tab  previous tab
/          filter/search
enter      open selected detail/action
esc        close modal / clear filter
```

### Apps tab

Columns:

- app
- target
- source
- connected
- approved
- status
- public URL

Actions:

```text
a  add/expose app
e  edit selected app
d  delete selected app
o  open public URL
c  copy public URL
p  approve selected app when can_approve=true
```

States:

- `approved`: green
- `pending`: amber
- `offline`: muted/red
- `blocked`: red with reason

### Approvals tab

Purpose: focused queue for apps needing user action.

Sections:

- Pending apps I can approve
- Pending apps requiring admin approval
- Recently approved

Actions:

```text
p      approve selected
P      bulk approve allowed selected/filter result, with confirmation
enter  details
```

Bulk approval must require confirmation:

```text
Approve 4 apps? This will make them publicly reachable through Portflare. [y/N]
```

### Traffic tab

Displays:

- total requests
- success/failure counts
- bytes in/out
- last request time
- per-app interval bucket table

Charts can be simple terminal sparklines in the first version.

### Discovery tab

Displays:

- discovery enabled/disabled
- allow/deny ranges
- discovered apps
- offline grace status

Actions:

```text
r  rescan
+  expose selected discovered app manually
```

### Logs tab

Displays recent daemon events:

- server connected/disconnected
- app registered/acknowledged
- proxy request failed
- discovery changes
- approval success/failure

Initial implementation may use an in-memory ring buffer.

### Settings tab

Displays:

- server URL
- local API URL
- state path
- discovery config
- current user
- client key redacted
- version/build metadata

The full client key should not be shown by default.

## GUI Experience

### Design direction

Modern, technical, professional. It should match the marketing site direction:

- dark theme
- teal/blue accent palette
- clear status indicators
- card-based overview
- crisp table interactions
- no raw JSON unless explicitly requested

### Navigation

Sidebar:

```text
Overview
Apps
Approvals
Traffic
Discovery
Logs
Settings
```

Top bar:

- Portflare logo
- server URL
- connected user
- websocket status
- refresh button

### Overview screen

Cards:

- Connection status
- Apps total / approved / pending
- Requests in last minute
- Discovery status
- Pending approvals

Primary actions:

- Expose app
- Approve pending apps
- Open docs

### Apps screen

Features:

- table and card view toggle
- filter by source/status
- copy public URL
- open public URL
- add/edit/delete app
- approval status pill
- approve button when allowed

### Approvals screen

Features:

- approval queue
- policy explanation
- one-click approve
- bulk approve with confirmation
- public URL preview

### Traffic screen

Features:

- per-app request counters
- interval bucket table
- simple charts using CSS/SVG/canvas only if necessary
- no external JS framework required in first version unless explicitly chosen later

### Discovery screen

Features:

- discovery status
- allowed/denied ranges
- discovered apps list
- rescan button
- expose discovered app action

### Logs screen

Features:

- recent event stream
- severity filters
- copy diagnostics

### Settings screen

Features:

- server URL
- local API bind address
- state path
- redacted client key
- docs links
- version/build metadata

Do not allow changing env-driven config in the first version unless persistence semantics are designed.

## Data Model Specification

## Local API DTOs

Add shared client-local DTOs, preferably in `client/internal/localapi` first. If they become cross-repo contracts, move stable shapes to `protocol/types`.

```go
type AppView struct {
    AppName       string    `json:"app_name"`
    TargetURL     string    `json:"target_url"`
    Source        string    `json:"source,omitempty"`
    DiscoveredPort int      `json:"discovered_port,omitempty"`
    Offline       bool      `json:"offline"`
    Approved      bool      `json:"approved"`
    Connected     bool      `json:"connected"`
    PublicPort    int       `json:"public_port,omitempty"`
    PublicURL     string    `json:"public_url,omitempty"`
    Status        string    `json:"status"`
    CanApprove    bool      `json:"can_approve"`
    UpdatedAt     time.Time `json:"updated_at,omitempty"`
}

type AppsResponse struct {
    Connected     bool      `json:"connected"`
    User          string    `json:"user,omitempty"`
    ServerURL     string    `json:"server_url"`
    Apps          []AppView `json:"apps"`
    ManualApps    []AppView `json:"manual_apps"`
    DiscoveryApps []AppView `json:"discovery_apps"`
}

type ServerStateResponse struct {
    Connected             bool      `json:"connected"`
    UserName              string    `json:"user_name,omitempty"`
    PublicUserLabel       string    `json:"public_user_label,omitempty"`
    RegistrationOpen      bool      `json:"registration_open"`
    AllowUserAppApproval  bool      `json:"allow_user_app_approval"`
    Apps                  []AppView `json:"apps"`
}

type ApproveAppRequest struct {
    AppName string `json:"app_name"`
}

type ApproveAppResponse struct {
    Approved  bool   `json:"approved"`
    AppName   string `json:"app_name"`
    PublicURL string `json:"public_url,omitempty"`
}

type ClientEvent struct {
    ID        string    `json:"id"`
    Type      string    `json:"type"`
    Level     string    `json:"level"`
    Message   string    `json:"message"`
    AppName   string    `json:"app_name,omitempty"`
    CreatedAt time.Time `json:"created_at"`
}
```

## Server client-key API DTOs

Add DTOs for user-scoped server APIs. Strong candidate for `protocol/types/client_api.go`.

```go
type ClientUserStateResponse struct {
    UserName             string            `json:"user_name"`
    PublicUserLabel      string            `json:"public_user_label"`
    BaseDomain           string            `json:"base_domain"`
    AllowUserAppApproval bool              `json:"allow_user_app_approval"`
    Apps                 []ClientAppStatus `json:"apps"`
}

type ClientAppStatus struct {
    UserName    string `json:"user_name"`
    AppName     string `json:"app_name"`
    Approved    bool   `json:"approved"`
    Connected   bool   `json:"connected"`
    PublicPort  int    `json:"public_port,omitempty"`
    PublicURL   string `json:"public_url"`
    Status      string `json:"status"`
    CanApprove  bool   `json:"can_approve"`
}
```

## Server API Changes

## New client-key scoped server endpoints

Add endpoints that authenticate using `Authorization: Bearer <PORTFLARE_CLIENT_KEY>`.

Recommended routes:

```text
GET  /api/client/me/state
POST /api/client/me/apps/{app}/approve
GET  /api/client/me/traffic
```

Alternative route for approval if path parsing should stay simple:

```text
POST /api/client/me/approve
{
  "app_name": "web"
}
```

### Authentication

Use existing `findUserByKey(key)` logic.

Accepted key locations:

- `Authorization: Bearer pf_...`
- optional `?key=` should not be accepted for new REST endpoints to avoid URL logging; keep query key only for websocket `/connect` compatibility.

### Authorization

For non-admin client-key user:

- can view only own apps
- can approve own apps only when `AllowUserAppApproval=true`
- cannot approve other users' apps
- cannot toggle server settings
- cannot access admin state

For admin client-key user:

Decision needed: allow admin-scoped client-key actions or keep `/api/client/me/*` strictly own-user only. Recommendation for first version: own-user only, even for admins, to avoid expanding client-key authority.

### Approval policy

`POST /api/client/me/approve` should apply the same policy as `/api/me/approve`:

```text
if !AllowUserAppApproval:
  403 approval not allowed
if app.UserName != keyUser.UserName:
  403 approval not allowed
if app does not exist:
  404 app not found
if already approved:
  200 approved=true idempotent response
else:
  mark approved, save state, notify UI, ensure dynamic listener if public port set
```

### Tests

Add server tests for:

- missing bearer token -> 401
- invalid bearer token -> 401
- valid key gets only own apps
- public URL and can_approve returned correctly
- approval succeeds when `AllowUserAppApproval=true`
- approval fails when false
- approval cannot cross users
- already approved is idempotent
- app not found -> 404
- dynamic listener behavior can be unit-isolated/mocked or left to existing helper tests

## Local Daemon API Changes

Introduce versioned endpoints while keeping existing endpoints as compatibility aliases.

### New endpoints

```text
GET    /v1/healthz
GET    /v1/apps
POST   /v1/apps
GET    /v1/apps/{app}
DELETE /v1/apps/{app}
GET    /v1/stats
GET    /v1/server/state
POST   /v1/server/apps/{app}/approve
GET    /v1/server/traffic
POST   /v1/discovery/rescan
GET    /v1/events
GET    /ui
GET    /ui/*
```

### Compatibility endpoints

Keep current routes:

```text
GET    /healthz
GET    /apps
POST   /apps
GET    /apps/{app}
DELETE /apps/{app}
GET    /stats
POST   /discovery/rescan
```

These can call the new handler methods.

### Server-backed local endpoints

`GET /v1/server/state`:

- daemon calls `GET <server>/api/client/me/state`
- uses `PORTFLARE_CLIENT_KEY` as bearer token
- merges remote app state with local apps by app name
- returns unified view model

`POST /v1/server/apps/{app}/approve`:

- daemon calls server client-key approval endpoint
- updates local app approved state if successful
- emits event

`GET /v1/server/traffic`:

- daemon calls `GET <server>/api/client/me/traffic`
- used by TUI/GUI traffic views

### Event stream

Add initial Server-Sent Events endpoint:

```text
GET /v1/events
```

Event types:

```text
server_connected
server_disconnected
app_registered
app_acknowledged
app_approved
app_deleted
discovery_changed
proxy_request_completed
proxy_request_failed
error
```

TUI can poll first; GUI can use SSE once available. If eventing is too large for first slice, implement an in-memory `/v1/events/recent` JSON endpoint first.

## Local API Security

The current local API is unauthenticated. Adding approval and server-backed actions increases risk.

### Requirements

1. Keep default bind address `127.0.0.1:9901`.
2. Detect non-loopback bind addresses and warn loudly.
3. Add local UI/API token before exposing powerful browser GUI mutations.
4. Add CSRF protection or same-origin token for GUI POST/DELETE actions.
5. Do not enable wildcard CORS.
6. Do not render the full client key in GUI/TUI by default.
7. Redact keys in logs/events.

### Suggested local UI token

Generate a random token at daemon startup or persist a local UI token in state.

For first version:

- TUI uses direct local API without browser CSRF concerns.
- GUI receives a server-rendered boot token embedded in `/ui` page or stored in an HttpOnly SameSite cookie.
- API requires `X-Portflare-Local-Token` for mutating `/v1/*` endpoints if request comes from browser GUI.

Open decision: whether to require this token for all local API clients or only GUI-origin mutating requests.

## Client Refactor Architecture

## Target package layout

```text
client/
  cmd/portflare/
    main.go              # thin entrypoint
    commands.go          # command dispatch or cobra root
    daemon.go            # daemon command wiring
    register.go          # register command wiring
    expose.go            # expose/list/stats/approve commands
    tui.go               # tui command wiring
    gui.go               # gui command wiring

  internal/config/
    config.go            # env/default loading
    config_test.go

  internal/appstate/
    state.go             # JSON load/save, app mutations
    state_test.go

  internal/localapi/
    server.go            # route setup
    handlers.go
    dto.go
    client.go            # local API client for CLI/TUI
    server_test.go
    client_test.go

  internal/serverapi/
    client.go            # REST client to Portflare server
    dto.go
    client_test.go

  internal/tunnel/
    service.go           # websocket lifecycle
    proxy.go             # proxy request handling
    service_test.go

  internal/discovery/
    discovery.go
    ports.go
    discovery_test.go

  internal/stats/
    stats.go
    stats_test.go

  internal/events/
    ring.go
    sse.go
    events_test.go

  internal/ui/viewmodel/
    model.go             # app/status view model transformation
    model_test.go

  internal/ui/tui/
    model.go
    update.go
    views.go
    keys.go
    styles.go
    tui_test.go

  internal/ui/web/
    assets/              # static HTML/CSS/JS or embedded build output
    handler.go
    handler_test.go
```

## Command parser

Two options:

### Option A: keep lightweight custom parser

Pros:

- minimal dependencies
- smaller migration

Cons:

- growing command surface becomes harder
- manual flag parsing is error-prone

### Option B: adopt Cobra

Pros:

- mature command hierarchy
- better help output
- subcommands like `discovery rescan`
- easier testing of command IO

Cons:

- new dependency
- migration work

Recommendation: adopt Cobra during refactor if the team accepts a dependency. If not, split current parser into command structs/interfaces without Cobra.

## TUI Technical Design

### Model

```go
type Model struct {
    api       localapi.Client
    tab       Tab
    apps      []viewmodel.App
    stats     viewmodel.Stats
    events    []viewmodel.Event
    loading   bool
    err       error
    selected  int
    filter    string
    modal     Modal
}
```

### Messages

```go
type appsLoadedMsg struct { apps []viewmodel.App; err error }
type statsLoadedMsg struct { stats viewmodel.Stats; err error }
type approveResultMsg struct { app string; result ApproveResult; err error }
type rescanResultMsg struct { err error }
type eventMsg struct { event viewmodel.Event }
```

### Testing approach

Bubble Tea update logic should be pure and testable:

- key press changes tab
- app load updates model
- approve key triggers command only when `CanApprove=true`
- approve success updates app status
- approve failure shows error
- filter changes visible rows

Avoid snapshot-testing full terminal output initially. Test view model and important strings instead.

## GUI Technical Design

### First implementation

Static assets served from daemon:

```text
GET /ui
GET /ui/assets/*
```

Use dependency-free HTML/CSS/JS initially, matching the marketing site style. Consider a frontend framework later only if complexity grows.

### Frontend API calls

```js
GET    /v1/apps
GET    /v1/stats
GET    /v1/server/state
POST   /v1/server/apps/{app}/approve
POST   /v1/discovery/rescan
GET    /v1/events
```

### GUI tests

Go handler tests:

- `/ui` serves HTML
- static assets have correct content type
- local token/cookie behavior if added

JS tests are optional in first implementation unless a JS test runner is introduced.

## User Approval Flow

### CLI approval flow

```text
User runs: portflare approve --app web
CLI calls local daemon: POST /v1/server/apps/web/approve
Daemon calls server: POST /api/client/me/approve with bearer client key
Server verifies key -> user
Server verifies app belongs to user and policy allows approval
Server marks app approved
Daemon updates local state and emits event
CLI prints success and public URL
```

### TUI approval flow

```text
User selects pending app
TUI shows details panel with Approve button/keybinding
User presses p
Confirmation modal appears
User confirms
TUI calls local daemon approval endpoint
Success updates row and event log
Failure shows policy-aware error
```

### GUI approval flow

```text
User opens Approvals screen
Pending apps show status and policy label
User clicks Approve
Confirmation dialog appears
GUI POSTs local daemon approval endpoint with local UI token
Success updates app card/table and queue
Failure shows actionable error
```

## Security Considerations

### Server-side

- Client-key APIs must not accept `?key=`.
- Client-key APIs must not allow user override in request body.
- Client-key APIs must scope all operations to the key owner.
- Approval must enforce `AllowUserAppApproval` unless product explicitly changes policy.
- Admin privilege through client key should be intentionally limited in first version.
- Server remains the authority; UI state is advisory.

### Client-side

- Local API remains loopback by default.
- Add local API/UI token before enabling browser GUI mutations.
- Warn on non-loopback bind.
- No full client key display except explicit reveal action.
- No CORS wildcard.
- No secrets in URLs.
- Redact keys from event logs.

### GUI/browser

- Mutating requests require same-origin and token/CSRF protection.
- Do not rely only on obscurity of localhost port.
- Use `SameSite=Lax` or Strict local UI cookie if cookie token is used.

## TDD and Delegation Workflow

Each implementation slice should follow:

1. **Scout/context** when needed.
2. **Worker writes failing tests first**.
3. Worker implements minimal code to pass tests.
4. Worker runs targeted validation through Docker where needed.
5. **Reviewer reviews diff** for correctness, tests, security, and UX regressions.
6. Reviewer may apply small fixes if authorized by parent workflow.
7. Parent commits with conventional commit message.
8. Artifacts are saved under `docs/`, not `/tmp` only.

Reviewer checklist:

- Did tests exist before implementation?
- Does the server enforce authorization rather than trusting UI/client state?
- Are local API mutations protected or explicitly limited to loopback-safe context?
- Are DTOs typed and documented?
- Did refactor preserve current CLI behavior?
- Are docs updated?

## Implementation Phases and Commits

## Phase 0: Specification and decisions

### Commit: `docs(client): specify tui and gui refactor`

Tasks:

- Add this spec.
- Decide GUI approach: local web GUI first.
- Decide TUI library: Bubble Tea/Lip Gloss/Bubbles.
- Decide server approval API shape.
- Decide command parser: Cobra or custom split.

Acceptance:

- Spec reviewed and open questions triaged.

## Phase 1: Client refactor foundation

### Commit 1: `refactor(client): split config and state packages`

Files:

- `client/internal/config/config.go`
- `client/internal/appstate/state.go`
- move relevant tests from `main_test.go`

Changes:

- Extract env parsing from `runDaemon`.
- Extract state load/save and app mutation logic.
- Keep behavior unchanged.

Acceptance:

- Existing client tests pass.
- New package tests cover defaults, env parsing, state migration, app CRUD.

### Commit 2: `refactor(client): extract local api server and client`

Files:

- `client/internal/localapi/server.go`
- `client/internal/localapi/client.go`
- `client/internal/localapi/dto.go`

Changes:

- Move `/apps`, `/stats`, `/discovery/rescan`, `/healthz` handlers.
- Add typed local API client used by CLI commands.
- Keep compatibility endpoints.

Acceptance:

- Existing `expose` and `list` behavior unchanged.
- Handler tests and client tests pass.

### Commit 3: `refactor(client): extract tunnel discovery and stats services`

Files:

- `client/internal/tunnel`
- `client/internal/discovery`
- `client/internal/stats`

Changes:

- Move websocket lifecycle/proxy handling.
- Move discovery scanner and port parsing.
- Move stats counters/snapshots.
- Keep `cmd/portflare` thin.

Acceptance:

- Current tests pass.
- New package tests cover tunnel register ack state update, discovery normalization, stats increments.

## Phase 2: Server client-key control API

### Commit 4: `feat(server): add client-key user state api`

Files:

- `server/cmd/portflare-server/client_api.go`
- `server/cmd/portflare-server/client_api_test.go`
- possibly `protocol/types/client_api.go`

Changes:

- Add `GET /api/client/me/state`.
- Authenticate bearer client key.
- Return own app state, public URLs, status, `can_approve`.

Acceptance:

- Tests cover key auth, own-user scoping, invalid key, response shape.

### Commit 5: `feat(server): add client-key app approval api`

Files:

- `server/cmd/portflare-server/client_api.go`
- `server/cmd/portflare-server/client_api_test.go`

Changes:

- Add `POST /api/client/me/approve` or `/api/client/me/apps/{app}/approve`.
- Enforce same policy as user dashboard approval.
- Return typed JSON.

Acceptance:

- Tests cover allowed approval, denied policy, cross-user rejection, idempotent approval.

### Commit 6: `feat(client): proxy server state and approval through local api`

Files:

- `client/internal/serverapi/client.go`
- `client/internal/localapi/server_handlers.go`

Changes:

- Add server API client using `PORTFLARE_CLIENT_KEY` bearer auth.
- Add local endpoints:
  - `GET /v1/server/state`
  - `POST /v1/server/apps/{app}/approve`
  - optional `GET /v1/server/traffic`

Acceptance:

- Tests use `httptest.Server` for server API.
- Local API tests verify approval success/failure and local state update.

## Phase 3: CLI improvements

### Commit 7: `feat(client): add approve and stats commands`

Files:

- `client/cmd/portflare/approve.go` or split command files

Changes:

- Add `portflare approve --app <name>`.
- Add `portflare stats` if not already a command.
- Use local API client.

Acceptance:

- Tests cover success, server policy denied, daemon unreachable.

## Phase 4: TUI

### Commit 8: `feat(client): add tui dashboard shell`

Files:

- `client/internal/ui/tui`
- `client/cmd/portflare/tui.go`
- `client/go.mod`

Changes:

- Add Bubble Tea/Lip Gloss/Bubbles.
- Add `portflare tui`.
- Implement read-only overview/apps/stats tabs.

Acceptance:

- TUI model tests pass.
- Daemon unreachable screen tested.

### Commit 9: `feat(client): add tui app actions and approvals`

Files:

- `client/internal/ui/tui`

Changes:

- Add add/edit/delete/rescan actions.
- Add approval queue and approve action.
- Add confirmation modal.

Acceptance:

- Tests cover approve key, confirmation, success/failure model updates.

## Phase 5: Local web GUI

### Commit 10: `feat(client): serve local web gui`

Files:

- `client/internal/ui/web`
- `client/internal/ui/web/assets/*`
- `client/cmd/portflare/gui.go`

Changes:

- Add `/ui` route.
- Add `portflare gui` command.
- Implement overview/apps screens.
- Try browser open and always print URL.

Acceptance:

- Handler tests for static assets.
- Command test for printed URL and browser-open failure behavior.

### Commit 11: `feat(client): add gui approvals and traffic views`

Files:

- `client/internal/ui/web/assets/*`

Changes:

- Add approvals screen.
- Add traffic screen.
- Add discovery screen.
- Add settings/logs basics.

Acceptance:

- Go handler/local API tests cover required endpoints.
- Manual UI smoke documented.

## Phase 6: Events and polish

### Commit 12: `feat(client): add local event stream for interfaces`

Files:

- `client/internal/events`
- `client/internal/localapi`

Changes:

- Add event ring buffer.
- Add `/v1/events` SSE or `/v1/events/recent` JSON.
- Emit important daemon events.

Acceptance:

- Tests cover ring capacity, SSE format, event redaction.

### Commit 13: `docs(client): document tui gui and approvals`

Files:

- `client/README.md`
- `client/docs/usage.md`
- root docs if needed

Changes:

- Document commands.
- Document local API security.
- Document approval policy.
- Add screenshots later if available.

Acceptance:

- Docs include quickstart for TUI and GUI.

## Test Strategy

### Unit tests

- config parsing
- state load/save/mutations
- local API DTOs/handlers
- server API client
- approval permission logic
- TUI update/model logic
- event ring buffer
- web handler routing

### Handler/integration tests

- local daemon API with fake service
- server client-key API with seeded state
- local API proxying to fake server
- CLI commands against `httptest.Server`

### Security tests

- invalid/missing client key rejected
- cross-user approval rejected
- approval denied when server policy disabled
- local API mutation token/CSRF behavior
- no key leakage in event logs/UI DTOs
- non-loopback bind emits warning

### Manual smoke tests

```bash
# client daemon
PORTFLARE_SERVER_URL=http://127.0.0.1:8080 \
PORTFLARE_CLIENT_KEY=pf_... \
portflare daemon

# TUI
portflare tui

# GUI
portflare gui

# approval
portflare approve --app web
```

### Validation commands

Use Docker for Go tooling in this workspace:

```bash
cd client && docker run --rm -v "$PWD:/src" -w /src golang:1.23 go test ./...
cd server && docker run --rm -v "$PWD:/src" -w /src golang:1.23 go test ./...
cd protocol && docker run --rm -v "$PWD:/src" -w /src golang:1.23 go test ./...
```

## Risks

1. **Scope creep**: TUI, GUI, refactor, and server approval API are large together. Keep slices small.
2. **Approval authorization**: never rely on client-side `can_approve`; server must enforce policy.
3. **Local API security**: browser GUI increases risk from local web pages and non-loopback binds.
4. **Refactor regressions**: preserve current CLI and daemon behavior through tests before UI work.
5. **UI dependency size**: Bubble Tea dependencies are acceptable, but native GUI frameworks should be deferred.
6. **Auth changes overlap**: planned auth-backed registration may change registration UX; keep registration logic reusable.
7. **Public URL accuracy**: local-only app state may not know server base domain; remote state should be source of truth.
8. **Eventing complexity**: polling is acceptable initially; SSE can be added after core workflows.

## Open Questions

1. Should command parsing migrate to Cobra now?
2. Should `PORTFLARE_CLIENT_KEY` grant own-app approval only when `AllowUserAppApproval=true`, or should possession of the key always allow own-app approval? Recommendation: respect server policy.
3. Should admin users with client keys get admin approval powers from the tool? Recommendation: no in first version.
4. Should local API token be required for all local API clients or only GUI mutating requests?
5. Should GUI assets be plain HTML/CSS/JS or use a small framework? Recommendation: plain first.
6. Should server/client API DTOs move to `protocol` immediately? Recommendation: yes for stable server API contracts.
7. Should `portflare tui` require the daemon to be running, or should it be able to start/manage daemon lifecycle? Recommendation: require daemon first, add lifecycle management later.
8. Should `portflare gui` start the daemon if it is not running? Recommendation: not initially; show clear instructions.

## Acceptance Criteria for First Complete Release

- `portflare daemon` behavior remains compatible.
- Existing `expose`, `list`, `register`, `version` commands still work.
- `portflare approve --app <name>` can approve own app when server allows it.
- `portflare tui` provides a polished apps/approvals/stats experience.
- `portflare gui` opens/prints a local web GUI URL.
- GUI and TUI show pending approvals and allow approval with confirmation.
- Server tests prove client-key approval is safely scoped.
- Local API tests prove server proxy endpoints and security behavior.
- Docs explain TUI, GUI, approval policy, and local API security.
