# Protocol Auth Worker Implementation

Implemented Phase 1 Commit 1 protocol-only TDD slice.

## Changed files

- `protocol/types/auth.go`
- `protocol/types/auth_test.go`
- `protocol/validation/auth.go`
- `protocol/validation/auth_test.go`

## Tests added first

Added tests before implementation for:

- auth registration DTO JSON shape
- registration exchange DTO JSON shape
- registration response DTO JSON shape
- auth identity claims/internal verify DTO JSON shape
- auth/direct mode constants
- Google provider constant
- S256 code challenge method constant
- PKCE verifier validation
- S256 challenge generation/validation
- loopback CLI redirect URI validation
- safe relative browser return path validation
- basic auth identity claim validation

## Implemented API

### `protocol/types`

Added:

- `RegistrationMode`
  - `RegistrationModeAuth = "auth"`
  - `RegistrationModeDirect = "direct"`
- `AuthProvider`
  - `AuthProviderGoogle = "google"`
- `CodeChallengeMethodS256 = "S256"`
- `RegisterStartRequest`
- `RegisterStartResponse`
- `RegisterExchangeRequest`
- `RegistrationResponse`
- `AuthIdentityClaims`
- `InternalVerifyRequest`
- `InternalVerifyResponse`

### `protocol/validation`

Added:

- `IsValidPKCECodeVerifier(string) bool`
- `S256CodeChallenge(string) (string, error)`
- `IsValidS256CodeChallenge(string) bool`
- `IsLoopbackCLIRedirectURI(string) bool`
- `IsSafeBrowserReturnPath(string) bool`
- `AuthIdentityClaimsInput`
- `IsValidAuthIdentityClaims(AuthIdentityClaimsInput) bool`

## Validation

Local `go` is unavailable. Initial direct Docker bind-mount validation could not see the protocol files in the container, so I used the same tar-copy Docker workflow used elsewhere in this workspace.

Command run effectively:

```bash
cd protocol
go test ./...
```

via Docker `golang:1.23`.

Result:

```text
ok  github.com/portflare/protocol/types       0.002s
ok  github.com/portflare/protocol/validation  0.002s
```

## Deviations / open questions

- Requested `/tmp/pi-subagents-uid-1000/chain-runs/ea21a4d8/context.md` and `plan.md` were not present, so I used the available phase docs and `/tmp/pi-subagents-uid-1000/chain-runs/9b02ce01/auth-handoff/phase-plan.md`.
- I kept browser-specific DTOs out for now to keep the API small; Phase 2 currently only needed shared browser return-path validation from this slice. If the server/auth implementation wants explicit browser start response DTOs, they can be added when route contracts are finalized.
- `InternalVerifyResponse` embeds `AuthIdentityClaims`, producing the flat JSON shape shown in the docs.
- `IsLoopbackCLIRedirectURI` intentionally requires `http`, an explicit port, and `/callback` path.
