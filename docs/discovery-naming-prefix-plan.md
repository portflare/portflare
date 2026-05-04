# Discovery Naming Prefix and Protocol-Aware Naming Plan

> Status: To be completed. This is the planning/specification document for configurable discovery naming prefixes and optional protocol-aware discovered app names.

## Goal
Add backward-compatible discovery naming controls so users can prefix discovered app names with a descriptor and optionally include protocol labels, while keeping exact per-port overrides and current `app-{port}` defaults intact.

## Tasks
1. **Define discovery naming product contract**: Lock the supported naming modes and precedence before implementation.
   - File: `docs/discovery-naming-prefix-plan.md`
   - Changes: Treat this document as the implementation spec; confirm the following contract:
     - default remains `app-{port}`
     - exact per-port names still win via `PORTFLARE_CLIENT_DISCOVER_NAMES`
     - descriptor prefix is optional
     - protocol labels are explicit first, detected later
     - generated names are slugged
     - collisions are resolved deterministically
   - Acceptance: Maintainers agree on env names, template names, collision behavior, and the distinction between protocol labels and actual proxy support.

2. **Introduce typed discovery naming config**: Add a small config type rather than passing multiple loosely related maps/strings through scanner functions.
   - File: `client/cmd/portflare/main.go` initially, or `client/internal/discovery/config.go` if discovery extraction starts first
   - Changes: Add a struct equivalent to:
     - `ExactNameByPort map[int]string`
     - `ProtocolByPort map[int]string`
     - `Descriptor string`
     - `NameTemplate string`
     - later optional `ProtocolDetection string`
   - Acceptance: Discovery scanner can receive one naming config object and existing behavior is unchanged when new fields are empty.

3. **Add env configuration loading**: Load new daemon env vars without changing existing env behavior.
   - File: `client/cmd/portflare/main.go`
   - Changes: Extend `Config` with naming fields or nested config. Add env vars:
     - `PORTFLARE_CLIENT_DISCOVER_DESCRIPTOR` - slugged descriptor such as `devbox`, `billing`, `homelab`
     - `PORTFLARE_CLIENT_DISCOVER_NAME_TEMPLATE` - enum value; default empty/current behavior
     - `PORTFLARE_CLIENT_DISCOVER_PROTOCOLS` - comma map like `3000=http,6379=redis,3306=mysql,8443=https`
   - Acceptance: With no new env vars, discovery still names apps `app-{port}` and `PORTFLARE_CLIENT_DISCOVER_NAMES=3000=web` still names port 3000 `web`.

4. **Define supported templates and defaults**: Implement a small enum parser instead of arbitrary templates for the first slice.
   - File: `client/cmd/portflare/main.go` or `client/internal/discovery/naming.go`
   - Changes: Support these `PORTFLARE_CLIENT_DISCOVER_NAME_TEMPLATE` values:
     - empty / `port` / `app-port`: current default `app-{port}`
     - `descriptor-port`: `{descriptor}-{port}`
     - `descriptor-proto-port`: `{descriptor}-{proto}-{port}`
     - `descriptor-proto`: `{descriptor}-{proto}`, with collision fallback described below
   - Acceptance: Invalid template values fail daemon startup consistently with existing invalid discovery env parsing behavior.

5. **Add protocol map parser**: Parse explicit port-to-protocol labels.
   - File: `client/cmd/portflare/main.go` near `parsePortNameMap`, or `client/internal/discovery/config.go`
   - Changes: Add parser for `PORTFLARE_CLIENT_DISCOVER_PROTOCOLS` with same shape as name map: `port=label,port=label`. Slug protocol labels. Reject empty labels and invalid/non-positive ports.
   - Acceptance: Tests cover valid map, whitespace, slugging, duplicate port last-write behavior or explicit duplicate rejection, invalid ports, and invalid empty protocol labels.

6. **Implement app-name generation helper**: Centralize all naming rules.
   - File: `client/cmd/portflare/main.go` or `client/internal/discovery/naming.go`
   - Changes: Add helper such as `discoverAppName(port int, naming DiscoveryNamingConfig) string` or `buildDiscoveryName(candidate, naming) string`.
   - Rules:
     1. If exact name exists for port, use it.
     2. Else apply selected template.
     3. Slug final value.
     4. If final value is empty, fallback to `app-{port}`.
     5. If proto is required by the template but missing, prefer `{descriptor}-{port}` for `descriptor-proto-port` and `{descriptor}-{port}` for `descriptor-proto` rather than generating `unknown` names.
     6. If descriptor is required but missing, fallback to current `app-{port}`.
   - Acceptance: Unit tests prove all precedence and fallback cases.

7. **Add deterministic collision handling**: Avoid overwrites when a template omits the port.
   - File: `client/cmd/portflare/main.go` or `client/internal/discovery/naming.go`
   - Changes: During one scan, detect duplicate generated names before returning candidates. Recommended policy:
     - if duplicate names are generated and the candidate did not use an exact per-port override, append `-{port}` to each colliding generated name
     - exact per-port overrides should remain exact; if an exact override collides with another exact override, log/return a validation error or keep the first and suffix the later generated non-exact candidate
     - sort by port first so output is stable
   - Acceptance: Two HTTP ports with `descriptor-proto` become `devbox-http-3000` and `devbox-http-8080`, not one overwritten app.

8. **Thread naming config through discovery scanning**: Replace the existing `names map[int]string` scanner argument.
   - File: `client/cmd/portflare/main.go`
   - Changes: Change `discoverListeningHTTPCandidates(allow, deny, names)` to accept the naming config. Preserve target URL generation as `http://127.0.0.1:{port}` for now.
   - Acceptance: Existing discovery tests still pass with updated call sites; new tests confirm scanner output names use descriptor/template/protocol config.

9. **Add optional candidate protocol metadata locally**: Store protocol labels on candidates for naming and future TUI/GUI display, but avoid server/protocol changes in the first slice unless needed.
   - File: `client/cmd/portflare/main.go`
   - Changes: Extend internal `discoverCandidate` with `Protocol string`. Do not change `protocol/types.AppRegistration` in the first implementation unless UI display requires persisted protocol metadata.
   - Acceptance: Protocol labels influence generated names but do not imply TCP/MySQL/Redis proxy support.

10. **Add logging for generated naming decisions**: Make discovery behavior understandable when many apps are found.
    - File: `client/cmd/portflare/main.go`
    - Changes: On discovery refresh, log descriptor/template and any collision suffixing or missing proto fallback at debug/info level without spamming every tick unnecessarily.
    - Acceptance: Users can diagnose why names became `devbox-http-3000` instead of `devbox-http`.

11. **Update CLI/help surface carefully**: Since discovery is currently daemon-env driven, document env vars first; only add CLI flags if there is an existing daemon flag pattern to follow.
    - File: `client/cmd/portflare/main.go`
    - Changes: Update usage text to mention discovery naming env vars or add a `portflare discovery` help section if command parsing is being refactored.
    - Acceptance: `portflare help` or invalid command output points users to descriptor/template/protocol config.

12. **Add docs and examples**: Explain simple prefixing, protocol-aware naming, and compatibility.
    - File: `client/README.md`
    - File: `client/docs/usage.md`
    - File: `docs/learnings.md`
    - Changes: Add examples:
      - current default: `app-3000`
      - descriptor only: `PORTFLARE_CLIENT_DISCOVER_DESCRIPTOR=devbox`, `PORTFLARE_CLIENT_DISCOVER_NAME_TEMPLATE=descriptor-port` -> `devbox-3000`
      - protocol map: `PORTFLARE_CLIENT_DISCOVER_PROTOCOLS=3000=http,6379=redis`
      - protocol names: `descriptor-proto-port` -> `devbox-http-3000`, `devbox-redis-6379`
      - short names: `descriptor-proto` with collision fallback
      - exact override still wins: `PORTFLARE_CLIENT_DISCOVER_NAMES=3000=web`
    - Acceptance: Docs explicitly warn that protocol labels do not add non-HTTP/TCP proxying support by themselves.

13. **Add TDD coverage for naming rules**: Write tests before implementation.
    - File: `client/cmd/portflare/main_test.go` initially, or `client/internal/discovery/naming_test.go` after extraction
    - Changes: Add tests:
      - default uses `app-{port}`
      - exact port name wins over descriptor/template/protocol
      - descriptor-port names correctly
      - descriptor-proto-port names correctly
      - descriptor-proto names correctly when unique
      - descriptor-proto collision appends port deterministically
      - missing descriptor falls back to `app-{port}`
      - missing proto in proto template falls back to descriptor-port
      - slugifies descriptor and protocol labels
      - invalid template rejected
    - Acceptance: Tests fail before implementation and pass after.

14. **Add parser tests**: Cover new env parsing separately from scanner behavior.
    - File: `client/cmd/portflare/main_test.go` or `client/internal/discovery/config_test.go`
    - Changes: Add tests for `parsePortProtocolMap` and `parseDiscoverNameTemplate`.
    - Acceptance: Invalid env values produce clear errors consistent with current parser patterns.

15. **Add scanner integration tests**: Prove real candidate normalization applies new naming config.
    - File: `client/cmd/portflare/main_test.go`
    - Changes: Extend existing discovery candidate tests to call scanner/candidate normalization with a naming config.
    - Acceptance: Sorting by port remains stable; allow/deny still applies before naming; generated names are stable.

16. **Preserve local API behavior**: Ensure `/discovery/rescan` uses daemon config only.
    - File: `client/cmd/portflare/main.go`
    - File: `client/cmd/portflare/main_test.go`
    - Changes: No per-request naming override in first slice. Future TUI/GUI can preview settings but should not mutate naming per rescan unless a config API is added.
    - Acceptance: `POST /discovery/rescan` returns discovered apps using daemon naming config; disabled discovery still returns existing disabled response.

17. **Define rename/churn behavior explicitly**: Avoid hidden migrations in the first implementation.
    - File: `client/docs/usage.md`
    - Changes: Document that changing descriptor/template changes app names and therefore may create new server apps/public URLs requiring approval; old names can remain until cleaned up/offlined.
    - Acceptance: No automatic renaming/migration is attempted in the first slice.

18. **Consider later extraction into internal discovery package**: Align with planned client refactor without blocking this small feature.
    - File: `client/internal/discovery/naming.go` and related files in a later slice
    - Changes: Move range parsing, proc parsing, candidate normalization, and naming helpers out of monolithic `main.go`.
    - Acceptance: Refactor has no behavior change and all discovery tests move with the package.

## Files to Modify
- `client/cmd/portflare/main.go` - add discovery naming config fields, env loading, protocol map parser, template parser, centralized generated-name helper, scanner threading, collision handling, and usage text updates.
- `client/cmd/portflare/main_test.go` - add TDD coverage for naming defaults, exact overrides, descriptor templates, protocol labels, parser behavior, and collisions.
- `client/README.md` - document new env vars and examples.
- `client/docs/usage.md` - document discovery naming behavior, examples, and rename/public URL churn.
- `docs/learnings.md` - update broader discovery behavior notes.

## New Files
- `docs/discovery-naming-prefix-plan.md` - this plan/spec.
- `client/internal/discovery/naming.go` - optional later refactor target for naming rules.
- `client/internal/discovery/naming_test.go` - optional later refactor target for focused naming tests.
- `client/internal/discovery/config.go` - optional later refactor target for parser/config helpers.
- `client/internal/discovery/config_test.go` - optional later refactor target for parser tests.

## Dependencies
- Task 1 must be agreed before implementation because env/template/collision choices affect public UX.
- Tasks 2-6 are the core implementation foundation and should happen before scanner integration.
- Task 7 depends on Task 6 because collision handling needs to know whether a name was exact or generated.
- Task 8 depends on Tasks 2-7.
- Task 9 can happen with Task 8 but should stay internal in the first slice.
- Tasks 13-15 should be written before or alongside Tasks 3-8 for TDD.
- Docs Tasks 11-12 and 17 depend on the final accepted env/template names.
- Task 18 should be separate from the feature slice unless the implementer chooses to extract discovery first as a no-behavior-change refactor.

## Risks
- **Backward compatibility**: Changing the default would surprise users. Keep `app-{port}` unless descriptor/template is explicitly configured.
- **Public URL churn**: App names feed server app identity and public URLs. Changing naming config creates new server apps and may require re-approval.
- **Protocol wording**: `mysql`, `redis`, `tls`, or `https` labels in names do not mean Portflare supports raw TCP proxying. Docs and UI must be clear.
- **Protocol detection side effects**: Active probing can trigger service logs, failed auth attempts, or unexpected behavior. First implementation should use explicit `PORTFLARE_CLIENT_DISCOVER_PROTOCOLS`; active detection should be opt-in later.
- **Collisions**: `descriptor-proto` is likely to collide when many apps use HTTP. Deterministic suffixing is required to avoid overwriting local state.
- **Exact override collisions**: Two exact names can still collide after slugging. Decide whether to fail config load or suffix one. Recommendation: fail config load for duplicate exact names after slugging because users explicitly configured them.
- **Monolithic client file**: Discovery already lives in `client/cmd/portflare/main.go`. Adding this feature there is quickest but increases refactor pressure. A separate discovery package is cleaner but larger.
- **Config error UX**: Existing parser helpers panic through `mustParse*`. Matching that behavior is consistent but not ideal; a future config loader should return friendly errors.

## Proposed UX

### Backward-compatible default

```env
PORTFLARE_CLIENT_DISCOVER=true
```

Discovered names remain:

```text
app-3000
app-8080
app-6379
```

### Descriptor prefix by port

```env
PORTFLARE_CLIENT_DISCOVER=true
PORTFLARE_CLIENT_DISCOVER_DESCRIPTOR=devbox
PORTFLARE_CLIENT_DISCOVER_NAME_TEMPLATE=descriptor-port
```

Generated names:

```text
devbox-3000
devbox-8080
devbox-6379
```

### Descriptor + explicit protocol labels + port

```env
PORTFLARE_CLIENT_DISCOVER=true
PORTFLARE_CLIENT_DISCOVER_DESCRIPTOR=devbox
PORTFLARE_CLIENT_DISCOVER_NAME_TEMPLATE=descriptor-proto-port
PORTFLARE_CLIENT_DISCOVER_PROTOCOLS=3000=http,8443=https,6379=redis,3306=mysql
```

Generated names:

```text
devbox-http-3000
devbox-https-8443
devbox-redis-6379
devbox-mysql-3306
```

### Descriptor + protocol without port, with collision fallback

```env
PORTFLARE_CLIENT_DISCOVER=true
PORTFLARE_CLIENT_DISCOVER_DESCRIPTOR=devbox
PORTFLARE_CLIENT_DISCOVER_NAME_TEMPLATE=descriptor-proto
PORTFLARE_CLIENT_DISCOVER_PROTOCOLS=3000=http,8080=http,6379=redis
```

Recommended generated names:

```text
devbox-http-3000
devbox-http-8080
devbox-redis
```

### Exact per-port override still wins

```env
PORTFLARE_CLIENT_DISCOVER=true
PORTFLARE_CLIENT_DISCOVER_DESCRIPTOR=devbox
PORTFLARE_CLIENT_DISCOVER_NAME_TEMPLATE=descriptor-proto-port
PORTFLARE_CLIENT_DISCOVER_PROTOCOLS=3000=http,6379=redis
PORTFLARE_CLIENT_DISCOVER_NAMES=3000=web
```

Generated names:

```text
web
devbox-redis-6379
```

## Phased Implementation

### Phase 1: Naming templates without active protocol detection
- Add config/env parsing.
- Add explicit protocol map.
- Add central naming helper and collision handling.
- Update scanner and docs.
- Keep server/protocol structs unchanged.

### Phase 2: Refactor discovery into `client/internal/discovery`
- Move naming, parsing, candidate normalization, and proc scanning out of `main.go`.
- Preserve all Phase 1 behavior.
- Improve config loading errors if feasible.

### Phase 3: TUI/GUI discovery preview
- Show descriptor/template/protocol settings in Discovery tab.
- Preview generated names before registration.
- Surface collisions and missing protocol labels.

### Phase 4: Optional protocol discovery
- Add opt-in `PORTFLARE_CLIENT_DISCOVER_PROTOCOL_DETECTION=off|well-known-port|probe`.
- Start with safe well-known-port inference only:
  - `80=http`, `443=https`, `3306=mysql`, `5432=postgres`, `6379=redis`, `8080=http`, `8443=https`
- Consider active probes only after security review.

## Validation
- Run client tests with Docker because local Go tooling may be unavailable:

```bash
cd client && docker run --rm -v "$PWD:/src" -w /src golang:1.23 go test ./...
```

- Manual validation:
  1. Start daemon with discovery enabled and no new env vars; confirm names are `app-{port}`.
  2. Start daemon with descriptor/template; confirm generated names in `GET /apps?source=discovery`.
  3. Start daemon with protocol map and `descriptor-proto`; confirm collision suffixing.
  4. Confirm exact `PORTFLARE_CLIENT_DISCOVER_NAMES` overrides template-generated names.
  5. Confirm changing descriptor creates new names and does not mutate old persisted registrations automatically.
