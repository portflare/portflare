# Discovery Naming Scout Report

> Status: To be completed. This reconnaissance report supports the configurable discovery naming prefixes and optional protocol-aware naming work.

## Files Retrieved
1. `client/cmd/portflare/main.go` (lines 1-45) - `Config` fields include discovery enable/interval/grace/allow/deny/name-by-port settings.
2. `client/cmd/portflare/main.go` (lines 120-126) - CLI usage; no discovery-specific command or naming flags are exposed today.
3. `client/cmd/portflare/main.go` (lines 230-268) - daemon config reads discovery env vars, including `PORTFLARE_CLIENT_DISCOVER_NAMES`.
4. `client/cmd/portflare/main.go` (lines 388-545) - local API app listing/grouping and `POST /discovery/rescan` handler.
5. `client/cmd/portflare/main.go` (lines 547-644) - discovery lifecycle; discovered candidates become `AppRegistration` records and register messages.
6. `client/cmd/portflare/main.go` (lines 1040-1089) - current discovered app naming (`app-{port}`), per-port overrides, normalization, sorting.
7. `client/cmd/portflare/main.go` (lines 1215-1238) - parser for `PORTFLARE_CLIENT_DISCOVER_NAMES` port-to-name map.
8. `client/cmd/portflare/main_test.go` (lines 57-109) - tests for port range parsing, name map parsing, allow/deny helpers, candidate normalization.
9. `client/cmd/portflare/main_test.go` (lines 112-190) - discovery lifecycle/local API tests for offline marking, source grouping, disabled rescan.
10. `client/README.md` (lines 49-56) - README discovery env examples.
11. `client/docs/usage.md` (lines 38-47) - usage docs for discovery settings.
12. `docs/learnings.md` (lines 319-337) - broader docs explaining discovery behavior and config.
13. `protocol/types/types.go` (lines 17-28) - shared `AppRegistration` fields currently persisted/returned for discovered apps.
14. `server/cmd/portflare-server/main.go` (lines 690-718) - server receives only slugged `AppName` and optional `PublicPort`; discovery metadata/protocol is not sent.

## Key Code

### Discovery config today

`client/cmd/portflare/main.go` lines 30-45:

```go
type Config struct {
    ServerURL          string
    ClientKey          string
    LocalAPIAddr       string
    StatePath          string
    ReconnectDelay     time.Duration
    HTTPTimeout        time.Duration
    DiscoverEnabled    bool
    DiscoverInterval   time.Duration
    DiscoverGrace      time.Duration
    DiscoverAllow      []portRange
    DiscoverDeny       []portRange
    DiscoverNameByPort map[int]string
}
```

`client/cmd/portflare/main.go` lines 255-268:

```go
DiscoverEnabled:    envBool("PORTFLARE_CLIENT_DISCOVER", false),
DiscoverInterval:   envDuration("PORTFLARE_CLIENT_DISCOVER_INTERVAL", 5*time.Second),
DiscoverGrace:      envDuration("PORTFLARE_CLIENT_DISCOVER_GRACE", 10*time.Minute),
DiscoverAllow:      mustParsePortRanges(env("PORTFLARE_CLIENT_DISCOVER_ALLOW", "")),
DiscoverDeny:       mustParsePortRanges(env("PORTFLARE_CLIENT_DISCOVER_DENY", "22,2375,2376")),
DiscoverNameByPort: mustParsePortNameMap(env("PORTFLARE_CLIENT_DISCOVER_NAMES", "")),
```

Current env knobs:

- `PORTFLARE_CLIENT_DISCOVER=true|false`
- `PORTFLARE_CLIENT_DISCOVER_INTERVAL`, default `5s`
- `PORTFLARE_CLIENT_DISCOVER_GRACE`, default `10m`
- `PORTFLARE_CLIENT_DISCOVER_ALLOW`, comma/ranges, default empty means all ports
- `PORTFLARE_CLIENT_DISCOVER_DENY`, default `22,2375,2376`
- `PORTFLARE_CLIENT_DISCOVER_NAMES`, format `3000=web,8080=admin`

There are no CLI flags for discovery naming. `printUsage` currently lists only `daemon`, `register`, `expose`, `list`, `version` (`client/cmd/portflare/main.go` lines 120-126).

### Current app naming behavior

`client/cmd/portflare/main.go` lines 1040-1078:

```go
func discoverListeningHTTPCandidates(allow, deny []portRange, names map[int]string) ([]discoverCandidate, error) {
    ports := map[int]struct{}{}
    // reads /proc/net/tcp and /proc/net/tcp6
    // keeps only LISTEN entries and local/wildcard bind addresses
    // filters through allow/deny

    ordered := make([]int, 0, len(ports))
    for port := range ports {
        ordered = append(ordered, port)
    }
    sort.Ints(ordered)

    out := make([]discoverCandidate, 0, len(ordered))
    for _, port := range ordered {
        appName := fmt.Sprintf("app-%d", port)
        if configured, ok := names[port]; ok && configured != "" {
            appName = configured
        }
        out = append(out, discoverCandidate{
            AppName:   appName,
            TargetURL: fmt.Sprintf("http://127.0.0.1:%d", port),
            Port:      port,
        })
    }
    return normalizeDiscoverCandidates(out), nil
}
```

So the default is exactly `app-{port}`. A port-specific name mapping can replace the whole app name, but there is no general descriptor/prefix template.

`client/cmd/portflare/main.go` lines 1081-1089:

```go
func normalizeDiscoverCandidates(in []discoverCandidate) []discoverCandidate {
    out := make([]discoverCandidate, 0, len(in))
    for _, candidate := range in {
        candidate.AppName = slug(candidate.AppName)
        if candidate.AppName == "" {
            candidate.AppName = fmt.Sprintf("app-%d", candidate.Port)
        }
        out = append(out, candidate)
    }
```

Candidate names are slugged. Empty/invalid names fall back to `app-{port}`.

### Existing exact-name override

`client/cmd/portflare/main.go` lines 1215-1238:

```go
func parsePortNameMap(raw string) (map[int]string, error) {
    raw = strings.TrimSpace(raw)
    if raw == "" {
        return map[int]string{}, nil
    }
    out := map[int]string{}
    for _, part := range strings.Split(raw, ",") {
        portRaw, nameRaw, ok := strings.Cut(part, "=")
        // port must be positive int
        name := slug(nameRaw)
        if name == "" {
            return nil, fmt.Errorf("invalid app name in mapping %q", part)
        }
        out[port] = name
    }
    return out, nil
}
```

This is useful for a few known ports (`3000=web`), but poor for 15+ apps where users want one descriptor across many discovered ports.

### Discovery lifecycle and server registration

`client/cmd/portflare/main.go` lines 562-593:

```go
func (s *Service) refreshDiscovery() {
    candidates, err := discoverListeningHTTPCandidates(s.cfg.DiscoverAllow, s.cfg.DiscoverDeny, s.cfg.DiscoverNameByPort)
    // ...
    for _, candidate := range candidates {
        port := candidate.Port
        appName := candidate.AppName
        targetURL := candidate.TargetURL
        // ...
        if !ok {
            app = &AppRegistration{
                AppName:        appName,
                TargetURL:      targetURL,
                Source:         "discovery",
                DiscoveredPort: port,
                LastSeenAt:     now,
                CreatedAt:      now,
                UpdatedAt:      now,
            }
            s.apps[appName] = app
            // save state and send register message
            _ = s.sendIfConnected(ConnectMessage{Type: protocoltypes.MessageTypeRegister, AppName: appName})
```

The generated name becomes:

- local state map key
- `AppRegistration.AppName`
- local API response name
- websocket `register` `AppName`
- server-side app name/public URL input

Changing naming is therefore not just cosmetic. Existing discovered apps under old names will remain in persisted state until deleted/offlined, unless migration/rename behavior is designed.

### Local API endpoints relevant to future UI/CLI

`client/cmd/portflare/main.go` lines 360-365:

```go
mux.HandleFunc("/apps", s.handleApps)
mux.HandleFunc("/apps/", s.handleAppByName)
mux.HandleFunc("/stats", s.handleStats)
mux.HandleFunc("/discovery/rescan", s.handleDiscoveryRescan)
```

`POST /discovery/rescan` exists, but only triggers a scan using daemon config. It does not accept request-body naming options. `GET /apps` groups `manual_apps` and `discovery_apps` and supports `?source=` filtering (`client/cmd/portflare/main.go` lines 388-414).

### Protocol/server visibility

`protocol/types/types.go` lines 17-28:

```go
type AppRegistration struct {
    AppName        string    `json:"app_name"`
    TargetURL      string    `json:"target_url"`
    PublicPort     int       `json:"public_port,omitempty"`
    Approved       bool      `json:"approved"`
    Source         string    `json:"source,omitempty"`
    DiscoveredPort int       `json:"discovered_port,omitempty"`
    Offline        bool      `json:"offline,omitempty"`
    LastSeenAt     time.Time `json:"last_seen_at,omitempty"`
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`
}
```

No discovered protocol/service type field exists. Server registration receives only `AppName` and `PublicPort` from websocket register messages (`server/cmd/portflare-server/main.go` lines 701-718). Protocol-aware naming can be entirely client-side initially if it only affects `AppName` and `TargetURL`, but displaying/searching protocol separately would need new local/protocol DTO fields.

## Architecture

Discovery is currently embedded inside the monolithic client command package:

1. `portflare daemon` builds `Config` from env vars.
2. If `PORTFLARE_CLIENT_DISCOVER=true`, `runDiscovery` starts a ticker.
3. `refreshDiscovery` calls `discoverListeningHTTPCandidates`.
4. Scanner reads `/proc/net/tcp` and `/proc/net/tcp6` via `parseProcNetTCP`.
5. Scanner keeps listening ports on local/wildcard addresses only.
6. Scanner applies allow/deny ranges.
7. Scanner names each candidate as:
   - exact `PORTFLARE_CLIENT_DISCOVER_NAMES[port]`, if present; else
   - `app-{port}`.
8. `normalizeDiscoverCandidates` slugifies/sorts/fallbacks names.
9. `refreshDiscovery` upserts `AppRegistration` under `apps[appName]`, marks source `discovery`, stores `DiscoveredPort`, persists JSON state, and sends websocket `register` to server if connected.
10. Server slugifies and persists the app name as the public routing identifier.

## Current Behavior

- Discovery is disabled by default.
- Discovery only looks in the client's own network namespace, not Docker-wide.
- Default discovered target URL is always `http://127.0.0.1:{port}`.
- Default discovered app name is always `app-{port}`.
- Exact port-to-name override already exists with `PORTFLARE_CLIENT_DISCOVER_NAMES=3000=web,8080=admin`.
- There is no descriptor/prefix option such as `my-stack-{port}`.
- There is no naming template option.
- There is no protocol detection or protocol override map.
- There is no CLI subcommand for discovery settings; only env vars and local `POST /discovery/rescan`.
- Discovered names are slugged with lowercase alnum/dash by `slug`.

## Gaps for Descriptor/Protocol-Aware Naming

1. **No general prefix/descriptor field**
   - Add likely config field such as `DiscoverDescriptor string` or `DiscoverNamePrefix string`.
   - Env candidate: `PORTFLARE_CLIENT_DISCOVER_DESCRIPTOR` or `PORTFLARE_CLIENT_DISCOVER_PREFIX`.
   - User examples imply descriptor should be the semantic group/app stack name, e.g. `billing`, `devbox`, `homelab`.

2. **No naming template**
   - Supporting many patterns cleanly likely wants an enum/template:
     - `port`: `app-{port}` current default
     - `descriptor-port`: `{descriptor}-{port}`
     - `descriptor-proto-port`: `{descriptor}-{proto}-{port}`
     - `descriptor-proto`: `{descriptor}-{proto}`
   - Env candidate: `PORTFLARE_CLIENT_DISCOVER_NAME_TEMPLATE`.
   - Backward-compatible default should be current `app-{port}` unless descriptor is explicitly set and product chooses automatic `{descriptor}-{port}`.

3. **No protocol/service model**
   - Current discovery calls everything HTTP and creates `http://127.0.0.1:{port}` targets.
   - `discoverCandidate` only has `AppName`, `TargetURL`, `Port`.
   - Future protocol-aware naming needs at least `Protocol string` on candidate.
   - Be careful with terms: target URL scheme (`http`, `https`, `tcp`?) may differ from service label (`mysql`, `redis`, `tls`, `http`).

4. **No protocol detection mechanism**
   - Current scanner only parses listening TCP sockets. It does not inspect process names, banners, Docker labels, service ports, or perform handshakes.
   - Initial safe design could support protocol overrides by port before active probing:
     - `PORTFLARE_CLIENT_DISCOVER_PROTOCOLS=3306=mysql,6379=redis,5432=postgres,8443=https`
   - Later active probing can be a separate opt-in feature because probing can have side effects/security implications.

5. **No collision handling beyond map overwrite risk**
   - Current default includes port, so collisions are unlikely.
   - `{descriptor}-{proto}` can collide when multiple ports have the same proto, e.g. `api-http` on ports 3000 and 8080.
   - Need deterministic collision policy: reject template without port when collision occurs, append `-{port}`, or keep first and mark others skipped. Recommendation: fallback/append port on collision with warning.

6. **No persisted metadata for generated name inputs**
   - `AppRegistration` stores `DiscoveredPort`, but not descriptor/proto/name-template.
   - If config changes from `app-{port}` to `team-{port}`, old discovered app remains and new one is created, while old is eventually marked offline after grace. This may be acceptable but should be documented.

7. **No TUI/GUI integration yet**
   - Existing TUI/GUI spec includes a Discovery tab but not this naming feature.
   - Future UI should display descriptor/template/protocol map and preview discovered names before registering.

## Recommended Integration Points

### Minimal client-only slice

Likely changes:

- `client/cmd/portflare/main.go`
  - Extend `Config` with:
    - `DiscoverDescriptor string`
    - `DiscoverNameTemplate string`
    - optional `DiscoverProtocolByPort map[int]string`
  - Load env vars in `runDaemon`.
  - Replace `DiscoverNameByPort map[int]string` argument with a struct, e.g. `DiscoveryNamingConfig`.
  - Add helper function, e.g. `discoverAppName(port int, proto string, cfg DiscoveryNamingConfig) string`.
  - Add protocol field to `discoverCandidate` if implementing protocol labels.
  - Keep `PORTFLARE_CLIENT_DISCOVER_NAMES` as highest-precedence exact override.

### Better refactor-aligned slice

Because there is already a TUI/GUI refactor plan, consider extracting discovery first:

- `client/internal/discovery/discovery.go`
  - scanner and candidate generation
- `client/internal/discovery/naming.go`
  - descriptor/template/exact/protocol naming rules
- `client/internal/discovery/config.go`
  - parser functions for ranges, name maps, protocol maps

This avoids expanding the existing monolithic `main.go` further.

### Suggested precedence

1. Exact port name from `PORTFLARE_CLIENT_DISCOVER_NAMES` wins, for backward compatibility.
2. Else apply template with descriptor/proto/port.
3. Else fallback to current `app-{port}`.
4. Always slug final name; if slug empty, fallback to `app-{port}`.
5. Detect duplicate generated names in one scan; append port or error deterministically.

### Suggested env design

Backward-compatible examples:

```env
# Existing behavior
PORTFLARE_CLIENT_DISCOVER=true

# Prefix all discovered apps by descriptor: devbox-3000, devbox-8080
PORTFLARE_CLIENT_DISCOVER_DESCRIPTOR=devbox
PORTFLARE_CLIENT_DISCOVER_NAME_TEMPLATE=descriptor-port

# Add protocol labels: devbox-http-3000, devbox-redis-6379
PORTFLARE_CLIENT_DISCOVER_NAME_TEMPLATE=descriptor-proto-port
PORTFLARE_CLIENT_DISCOVER_PROTOCOLS=3000=http,6379=redis,3306=mysql

# Short names where safe: devbox-redis; collision fallback should append port
PORTFLARE_CLIENT_DISCOVER_NAME_TEMPLATE=descriptor-proto

# Exact legacy overrides still win
PORTFLARE_CLIENT_DISCOVER_NAMES=3000=web
```

Potential template strings could be either enum values (`descriptor-proto-port`) or literal templates (`{descriptor}-{proto}-{port}`). Literal templates are flexible but need more validation. Enum values are simpler and safer.

## Likely Test Targets

### Unit tests in `client/cmd/portflare/main_test.go` or future `client/internal/discovery/*_test.go`

Existing tests to extend:

- `TestParsePortNameMap` (`client/cmd/portflare/main_test.go` lines 70-80)
- `TestDiscoverCandidatesApplyNamingAndDeny` (`client/cmd/portflare/main_test.go` lines 97-109)

New tests recommended:

1. `TestDiscoverAppNameDefaultUsesAppPort`
   - no descriptor/template/proto/name override -> `app-3000`.

2. `TestDiscoverAppNameExactPortNameWins`
   - `names[3000]=web` plus descriptor/template -> `web`.

3. `TestDiscoverAppNameDescriptorPort`
   - descriptor `devbox`, template `descriptor-port`, port 3000 -> `devbox-3000`.

4. `TestDiscoverAppNameDescriptorProtoPort`
   - descriptor `devbox`, protocol `redis`, port 6379 -> `devbox-redis-6379`.

5. `TestDiscoverAppNameDescriptorProtoFallsBackWhenProtoMissing`
   - decide expected behavior: `devbox-3000`, `devbox-unknown-3000`, or fallback to `app-3000`. Recommendation: `devbox-3000` for missing proto in proto templates.

6. `TestDiscoverNameTemplateCollisionAppendsPort`
   - ports 3000/8080 both proto `http`, template `descriptor-proto` -> `devbox-http-3000` and `devbox-http-8080`, or other decided deterministic behavior.

7. `TestParsePortProtocolMap`
   - parse `3306=mysql,6379=redis,8443=tls`; slug labels; reject empty/invalid ports.

8. `TestNormalizeDiscoverCandidatesSlugifiesDescriptorProtocolNames`
   - descriptor/proto with spaces/caps -> lowercase dash slug.

9. `TestRunDaemonLoadsDiscoveryNamingEnv`
   - if env loading gets factored into a load-config function. Current `runDaemon` is harder to test because it starts services; refactor recommended.

### Local API tests

Existing:

- `TestHandleAppsGroupsBySource` (`client/cmd/portflare/main_test.go` lines 134-155)
- `TestHandleDiscoveryRescanDisabled` (`client/cmd/portflare/main_test.go` lines 181-190)

Future tests:

- `GET /apps?source=discovery` returns apps with new generated names.
- `POST /discovery/rescan` uses daemon naming config; no per-request override unless explicitly added.

### Docs tests/manual validation

Update and manually verify:

- `client/README.md` discovery section.
- `client/docs/usage.md` discovery section.
- `docs/learnings.md` discovery behavior section.

## Risks and Constraints

1. **Backward compatibility**
   - Current users may rely on `app-{port}` and `PORTFLARE_CLIENT_DISCOVER_NAMES`. Keep defaults and exact override behavior unchanged.

2. **Rename churn**
   - Changing descriptor/template creates new app names rather than renaming old ones. Old apps may remain offline/pending on server. Need docs and maybe a future cleanup command.

3. **Approval/public URL churn**
   - Since app name feeds public URLs and server approval state, a new generated name may require re-approval.

4. **Protocol detection side effects**
   - Active probing Redis/MySQL/TLS/HTTP can trigger logs, auth failures, rate limits, or unexpected application behavior. Prefer explicit port-protocol map first; make probing opt-in later.

5. **Non-HTTP targets**
   - The current proxy target is `http://127.0.0.1:{port}`. Naming a service `mysql` or `redis` does not make TCP proxying work if Portflare only proxies HTTP/WebSocket traffic. Product wording should distinguish protocol label for naming from actual supported traffic protocol.

6. **Collision handling**
   - Templates without `{port}` are attractive but unsafe for multiple same-proto ports. Must define deterministic handling before implementation.

7. **Slug validation/server constraints**
   - Server also slugifies app names on register. Use same slug rules client-side and test names do not collapse unexpectedly.

8. **Config panic behavior**
   - Current `mustParsePortRanges` / `mustParsePortNameMap` panic on invalid env. Adding `mustParsePortProtocolMap` would likely follow existing behavior, but better long-term config loading should return user-friendly errors.

## Start Here

Start in `client/cmd/portflare/main.go` around lines 1040-1089. That is where discovered app names are generated (`app-{port}`), exact per-port overrides are applied, and candidates are normalized. For a clean implementation, extract this logic into a dedicated discovery/naming helper first, then add descriptor/template/protocol config around it.
