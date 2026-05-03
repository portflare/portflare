## Review

- Correct: The auth service skeleton is appropriately scoped for the planned Phase 1 Commit 2 slice. It adds a standalone `github.com/portflare/auth` module with a `portflare-auth` binary, health route, config parsing, Makefile, and README without introducing provider or CLI/browser handoff routes prematurely (`auth/cmd/portflare-auth/main.go:17-65`, `auth/README.md:1-64`).
- Correct: The health endpoint is small and test-covered. `GET /healthz` returns JSON with `ok`, `service`, and `google_enabled`; non-GET requests return `405` (`auth/cmd/portflare-auth/main.go:49-58`, `auth/cmd/portflare-auth/main_test.go:11-41`). This fits the server health-detection plan.
- Correct: Config parsing covers the env vars called out for this slice: listen address, public URL, shared secret, code/transaction TTL, backend skew, and Google client settings (`auth/internal/config/config.go:13-50`). Validation checks absolute HTTP(S) URLs, positive durations, partial Google config, and weak secrets (`auth/internal/config/config.go:53-87`).
- Correct: The HMAC verifier implements the documented backend trust primitive: method/path/timestamp/nonce/body-hash payload, signed with HMAC-SHA256; timestamp skew rejection; nonce replay rejection; constant-time signature comparison; and request-body restoration for downstream handlers (`auth/internal/handoff/signing.go:75-130`). Tests cover valid signatures, missing headers, tampered body, stale timestamp, replay, wrong secret, and nonce expiry (`auth/internal/handoff/signing_test.go:11-98`).
- Correct: The nonce cache is mutex-protected and expiry-prunes on writes (`auth/internal/handoff/signing.go:33-73`), which is sufficient for the documented first single-instance auth service. Multi-instance replay protection remains a later shared-store concern.
- Fixed: Hardened Google-enabled config so it cannot be enabled without backend verification credentials. The original config allowed all Google OAuth variables while `PORTFLARE_AUTH_SHARED_SECRET` was empty; that would conflict with the architecture requirement that the server only accepts identity assertions over an authenticated backend channel. `auth/internal/config/config.go:81-83` now rejects Google-enabled config without a shared secret, and `auth/internal/config/config_test.go:78-92` covers it.
- Fixed: Improved test isolation for defaults by clearing Google env vars in `TestLoadDefaults` (`auth/internal/config/config_test.go:9-18`). Without this, a developer shell with Google env vars set could make the default test observe Google-enabled config or partial-Google failures.
- Note: `SignRequest` signs `req.URL.EscapedPath()` only, not the query string (`auth/internal/handoff/signing.go:140-145`). This is consistent enough for the planned internal `POST /internal/*/verify` endpoints, which should not need query parameters. If future signed backend endpoints use meaningful query strings, the signer/verifier should include `RawQuery` in the canonical target before those endpoints are added.
- Note: `PORTFLARE_AUTH_PUBLIC_URL` is optional even when Google config is present (`auth/internal/config/config.go:57-61`). This is acceptable for this skeleton because no public redirect routes exist yet. The Google provider/route slice should decide whether public URL is required or whether `PORTFLARE_AUTH_GOOGLE_REDIRECT_URL` is sufficient.
- Note: `GET /healthz` returns `google_enabled`; this is useful operationally and does not expose secrets. It does reveal provider availability, which is usually acceptable for a health/status endpoint but should remain limited to coarse provider flags.
- Note: No Dockerfile was added. The planner listed it in the skeleton commit, but the worker scope explicitly deferred packaging; this is reasonable if the next packaging/docs slice adds it before release.

## Validation

Ran through Docker `golang:1.23` tar-copy workflow from `auth/`:

```bash
go test ./...
go build -o /tmp/portflare-auth ./cmd/portflare-auth
```

Result:

```text
ok   github.com/portflare/auth/cmd/portflare-auth      0.003s
ok   github.com/portflare/auth/internal/config          0.002s
ok   github.com/portflare/auth/internal/handoff         0.003s
```

Build succeeded.

## Recommended parent follow-up

- Review the small config hardening fix before committing the auth slice.
- Commit this as something like `feat(auth): add service skeleton and backend signing`.
- In the next auth-service slice, keep TDD focused on provider/store boundaries before wiring Google routes.
- Before adding signed server clients, decide whether the canonical HMAC target should remain path-only for all internal endpoints or be expanded to path + query for future-proofing.
- Add `auth/Dockerfile` in a packaging slice before release/deployment docs claim container support.
