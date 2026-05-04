# Discovery Naming Implementation Worker Notes

> Status: Implemented. Reviewers validated the implementation and fixed follow-up collision/parser edge cases.

## Summary

Implemented configurable Portflare client discovery naming with descriptor prefixes, explicit protocol labels, supported naming templates, exact-name precedence, slugging, and deterministic collision handling for generated duplicate names.

## Changed files

Client repo:

- `client/cmd/portflare/main.go`
- `client/cmd/portflare/main_test.go`
- `client/README.md`
- `client/docs/usage.md`

Root docs:

- `docs/learnings.md`

## Behavior implemented

- Default discovery naming remains `app-{port}`.
- Existing `PORTFLARE_CLIENT_DISCOVER_NAMES` exact per-port overrides still win over all generated naming.
- Added env-backed discovery naming config:
  - `PORTFLARE_CLIENT_DISCOVER_DESCRIPTOR`
  - `PORTFLARE_CLIENT_DISCOVER_NAME_TEMPLATE`
  - `PORTFLARE_CLIENT_DISCOVER_PROTOCOLS`
- Supported templates:
  - empty / `port` / `app-port` -> `app-{port}`
  - `descriptor-port` -> `{descriptor}-{port}`
  - `descriptor-proto-port` -> `{descriptor}-{proto}-{port}`
  - `descriptor-proto` -> `{descriptor}-{proto}`
- Descriptor, protocol labels, and final app names are slugged.
- Missing descriptor falls back to `app-{port}`.
- Missing protocol in proto templates falls back to `{descriptor}-{port}`.
- Generated name collisions append `-{port}` deterministically, e.g. `devbox-http-3000`, `devbox-http-8080`.
- Duplicate exact overrides after slugging are rejected by `parsePortNameMap` when they target different ports.
- Discovery scanner now receives a typed `DiscoveryNamingConfig` and local API rescan uses daemon config.
- Discovery log line includes naming template, descriptor, and protocol metadata for newly discovered apps.

## Tests added/updated

Added coverage for:

- duplicate exact override rejection
- protocol map parsing and validation
- name template parsing and invalid template rejection
- default `app-{port}` naming
- exact override precedence
- `descriptor-port`
- `descriptor-proto-port`
- `descriptor-proto`
- `descriptor-proto` generated collision suffixing
- missing descriptor fallback
- missing proto fallback
- protocol/descriptor slugging through generated names

Existing discovery tests continue to pass.

## Validation

Used Docker because host Go tooling may be unavailable:

```bash
# in an isolated container copy of client/
/usr/local/go/bin/gofmt -w cmd/portflare/main.go cmd/portflare/main_test.go
/usr/local/go/bin/go test ./...
```

Result:

```text
?   	github.com/portflare/client/internal/buildinfo	[no test files]
ok  	github.com/portflare/client/cmd/portflare	0.065s
```

Also ran:

```bash
git diff --check
git -C client diff --check
```

No whitespace errors reported.

## Notes / risks

- Protocol labels are naming metadata only; docs explicitly warn that labels like `mysql` or `redis` do not imply raw TCP/database proxy support.
- Changing descriptor/template/protocol config changes app identity/public URLs and may require server approval for newly generated app names; docs now call this out.
- Implementation intentionally stays in `client/cmd/portflare/main.go` to keep the slice narrow. Future client refactor can move discovery naming into `client/internal/discovery`.
- Review follow-up resolved the exact-vs-generated secondary collision edge case by reserving exact names before assigning generated names and applying deterministic secondary suffixes when needed.
