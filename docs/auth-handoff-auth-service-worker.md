# Auth Service Worker Implementation

Implemented the next focused TDD slice for the auth architecture: new `auth/` Go service skeleton plus trusted backend HMAC verification utilities.

## Important context

The requested read files were not present:

- `/tmp/pi-subagents-uid-1000/chain-runs/8ae1a14f/context.md`
- `/tmp/pi-subagents-uid-1000/chain-runs/8ae1a14f/plan.md`

I used the persisted planning docs instead:

- `docs/auth-registration-phase-1.md`
- `docs/auth-dashboard-phase-2.md`
- `docs/auth-handoff-phase-plan.md`
- `docs/auth-handoff-scout-review.md`

## Changed files

New auth module files:

- `auth/go.mod`
- `auth/README.md`
- `auth/Makefile`
- `auth/cmd/portflare-auth/main.go`
- `auth/cmd/portflare-auth/main_test.go`
- `auth/internal/config/config.go`
- `auth/internal/config/config_test.go`
- `auth/internal/handoff/signing.go`
- `auth/internal/handoff/signing_test.go`

No server/client/protocol files were changed in this slice.

## TDD tests added first

Tests were written before implementation and initially failed because the packages/functions did not exist.

Added tests for:

### Auth service health route

- `GET /healthz` returns JSON with `ok: true` and `service: portflare-auth`
- non-GET `/healthz` returns `405 Method Not Allowed`

### Config loading and validation

- default env config values
- configured env values
- public URL normalization
- duration parsing
- all-or-nothing Google config validation
- weak shared-secret rejection when a secret is set
- invalid duration rejection

### HMAC backend verification

- valid signed request is accepted
- missing signature headers are rejected
- tampered body is rejected
- stale timestamp is rejected
- replayed nonce is rejected
- wrong shared secret is rejected
- nonce cache expires old entries

## Implemented behavior

### `auth/cmd/portflare-auth`

- loads config from env
- logs startup metadata without secrets
- starts an HTTP server
- exposes `GET /healthz`
- supports graceful shutdown on SIGINT/SIGTERM

### `auth/internal/config`

Env vars implemented from the plan:

- `PORTFLARE_AUTH_LISTEN_ADDR` default `:8081`
- `PORTFLARE_AUTH_PUBLIC_URL`
- `PORTFLARE_AUTH_SHARED_SECRET`
- `PORTFLARE_AUTH_CODE_TTL` default `5m`
- `PORTFLARE_AUTH_TX_TTL` default `5m`
- `PORTFLARE_AUTH_BACKEND_SKEW` default `60s`
- `PORTFLARE_AUTH_GOOGLE_CLIENT_ID`
- `PORTFLARE_AUTH_GOOGLE_CLIENT_SECRET`
- `PORTFLARE_AUTH_GOOGLE_REDIRECT_URL`

Validation rules:

- listen address cannot be empty
- public URL, when set, must be absolute `http` or `https`
- shared secret, when set, must be at least 32 characters
- Google config is all-or-nothing, but Google itself is not implemented yet
- Google redirect URL, when set, must be absolute `http` or `https`
- TTL/skew durations must be positive

### `auth/internal/handoff`

Implemented HMAC utilities for future internal auth-service verify endpoints.

Headers:

- `X-Portflare-Timestamp`
- `X-Portflare-Nonce`
- `X-Portflare-Signature`

Signature payload:

```text
METHOD + "\n" + PATH + "\n" + TIMESTAMP + "\n" + NONCE + "\n" + SHA256(body)
```

Verifier behavior:

- requires strong shared secret
- requires timestamp, nonce, and signature headers
- rejects timestamps outside configured skew
- recomputes signature over method/path/timestamp/nonce/body hash
- uses constant-time signature comparison
- restores request body after verification so future handlers can read it
- records accepted nonces in an in-memory replay cache
- rejects replayed nonces until their TTL expires

`SignRequest` is included for tests and later server-side client implementation.

## Validation

Local Go tooling is unavailable, so validation used Docker `golang:1.23` with the tar-copy workflow.

Command effectively run from `auth/`:

```bash
go test ./...
go build -o /tmp/portflare-auth ./cmd/portflare-auth
```

Result:

```text
ok   github.com/portflare/auth/cmd/portflare-auth      0.003s
ok   github.com/portflare/auth/internal/config          0.002s
ok   github.com/portflare/auth/internal/handoff         0.004s
```

Build also succeeded.

## Open questions / notes

- Google OAuth is intentionally not implemented in this slice.
- CLI/browser handoff routes are intentionally not implemented in this slice.
- No Dockerfile was added because the delegated task required the module, README, optional Makefile, health route, config, and signing utilities; Dockerfile can be added in a later packaging slice.
- The HMAC timestamp currently uses Unix seconds. Server-side signing must match this when implemented.
- Nonce replay cache is in-memory, which is fine for the first single-instance auth service but will need shared storage for multi-instance deployments.
- The requested `/tmp/.../context.md` and `/tmp/.../plan.md` were unavailable; persisted `docs/` artifacts were used as source-of-truth context.
