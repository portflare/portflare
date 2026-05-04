# Discovery Naming UX/Security/Maintainability Review

> Status: Implemented. UX/security/maintainability review found no blockers.

## Review
- Correct: Backward compatibility is preserved. The scanner still defaults to `app-{port}` through the default template path in `client/cmd/portflare/main.go:1182-1206`, and tests cover this in `client/cmd/portflare/main_test.go:194-201`.
- Correct: Exact per-port names keep precedence over generated descriptor/protocol names. `discoveryCandidateForPort` checks `ExactNameByPort` before calling `generatedDiscoveryName` (`client/cmd/portflare/main.go:1160-1169`), and `TestDiscoveryNamingExactOverrideWins` covers the behavior (`client/cmd/portflare/main_test.go:205-214`).
- Correct: Slugging is applied at the right boundaries for descriptor, protocol labels, exact names, and final candidate normalization (`client/cmd/portflare/main.go:1067-1077`, `client/cmd/portflare/main.go:1160-1178`, `client/cmd/portflare/main.go:1211-1221`, `client/cmd/portflare/main.go:1382-1385`, `client/cmd/portflare/main.go:1418-1421`). This keeps generated app names compatible with existing app-name validation expectations.
- Correct: Failure modes for invalid user config are explicit. Invalid templates, empty protocol labels, invalid ports, and duplicate exact names return parser errors and are surfaced at daemon startup via `mustLoadDiscoveryNamingConfig` (`client/cmd/portflare/main.go:1059-1078`, `client/cmd/portflare/main.go:1287-1301`, `client/cmd/portflare/main.go:1363-1395`, `client/cmd/portflare/main.go:1401-1424`). This is consistent with the existing `mustParse*` discovery env style.
- Correct: Collision behavior is deterministic and now handles generated-vs-exact collisions, not only generated-vs-generated collisions. The implementation seeds `usedNames` with exact overrides and re-suffixes generated candidates until unique (`client/cmd/portflare/main.go:1115-1152`), with coverage in `TestDiscoveryNamingDescriptorProtoCollisionAppendsPort` and `TestDiscoveryNamingGeneratedSuffixAvoidsExactOverride` (`client/cmd/portflare/main_test.go:247-275`).
- Correct: Protocol wording is mostly clear in user docs. `client/docs/usage.md:71-92`, `client/README.md:70-78`, and `docs/learnings.md:341` all state that protocol labels are naming metadata only and do not add raw TCP/database proxy support.
- Correct: Validation passed in a Docker copy of `client/` using `/usr/local/go/bin/go test ./...`:
  - `ok github.com/portflare/client/cmd/portflare 0.083s`
  - `? github.com/portflare/client/internal/buildinfo [no test files]`

- Blocker: None found from the UX/security/maintainability review.

- Note: The `portflare` help text lists `PORTFLARE_CLIENT_DISCOVER_PROTOCOLS` (`client/cmd/portflare/main.go:141-146`) but does not include the important warning that these are labels only and do not enable MySQL/Redis/raw TCP proxying. The longer docs do include the warning, so this is not blocking; consider adding one short help line if support confusion appears likely.
- Note: Discovery naming logic adds meaningful size to the already monolithic `client/cmd/portflare/main.go` (`client/cmd/portflare/main.go:1059-1424`). The implementation is reasonably grouped and tested, so it is acceptable for this slice, but it should move into a future `client/internal/discovery` package during the planned client cleanup.
- Note: The discovered-app log line now includes protocol/template/descriptor metadata only when a new app is first discovered (`client/cmd/portflare/main.go:612`). That keeps logs low-noise, but it does not explicitly explain fallback decisions such as missing protocol causing `{descriptor}-{port}`. Current docs and deterministic naming tests mitigate this; add debug-level naming decision logs later only if users struggle to diagnose generated names.
- Note: `parsePortNameMap` preserves last-write behavior for repeated mappings of the same port because duplicate-name validation runs after the final map is built (`client/cmd/portflare/main.go:1368-1395`). This is user-friendly and avoids false duplicate failures, but it is worth documenting only if repeated port mappings become a supported/intentional UX.
