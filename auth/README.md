# Portflare Auth

`portflare-auth` is the dedicated authentication service for Portflare.

It owns authentication provider logic and returns verified identity assertions to the configured Portflare server over an authenticated backend channel. The Portflare server remains authoritative for Portflare users, apps, and client API keys.

## Current scope

This initial skeleton includes:

- `GET /healthz`
- `GET /readyz` with application/version/build/Go debug metadata
- environment-driven config parsing
- trusted backend HMAC verification utilities for future internal verify endpoints

Google OAuth and CLI/browser handoff routes are intentionally not implemented in this slice.

## Build and test

```bash
make test
make build
```

## Run

```bash
export PORTFLARE_AUTH_LISTEN_ADDR=:8081
export PORTFLARE_AUTH_PUBLIC_URL=https://auth.example.com
export PORTFLARE_AUTH_SHARED_SECRET='replace-with-at-least-32-random-characters'
portflare-auth
```

## Environment

| Variable | Default | Description |
| --- | --- | --- |
| `PORTFLARE_AUTH_LISTEN_ADDR` | `:8081` | HTTP listen address. |
| `PORTFLARE_AUTH_PUBLIC_URL` | empty | Public auth service URL. Required by future redirect flows. |
| `PORTFLARE_AUTH_SHARED_SECRET` | empty | Shared secret for server-to-auth HMAC verification. Must be at least 32 characters when set. |
| `PORTFLARE_AUTH_CODE_TTL` | `5m` | Future one-time authorization code TTL. |
| `PORTFLARE_AUTH_TX_TTL` | `5m` | Future auth transaction TTL. |
| `PORTFLARE_AUTH_BACKEND_SKEW` | `60s` | Allowed timestamp skew for signed backend requests. |
| `PORTFLARE_AUTH_GOOGLE_CLIENT_ID` | empty | Future Google OAuth client ID. |
| `PORTFLARE_AUTH_GOOGLE_CLIENT_SECRET` | empty | Future Google OAuth client secret. |
| `PORTFLARE_AUTH_GOOGLE_REDIRECT_URL` | empty | Future Google OAuth callback URL. |

Google config is all-or-nothing: if any Google variable is set, all three Google variables must be set.

## Backend HMAC verification

Internal server-to-auth requests are signed with:

```text
HMAC-SHA256(secret, METHOD + "\n" + PATH + "\n" + TIMESTAMP + "\n" + NONCE + "\n" + SHA256(body))
```

Headers:

```text
X-Portflare-Timestamp: <unix seconds>
X-Portflare-Nonce: <unique random nonce>
X-Portflare-Signature: <hex hmac>
```

The verifier rejects missing headers, weak secrets, invalid signatures, stale timestamps, and replayed nonces.
