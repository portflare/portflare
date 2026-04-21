# Portflare

[![CI](https://github.com/portflare/portflare/actions/workflows/ci.yml/badge.svg)](https://github.com/portflare/portflare/actions/workflows/ci.yml)
[![Docker](https://github.com/portflare/portflare/actions/workflows/docker.yml/badge.svg)](https://github.com/portflare/portflare/actions/workflows/docker.yml)
[![Release](https://github.com/portflare/portflare/actions/workflows/release.yml/badge.svg)](https://github.com/portflare/portflare/actions/workflows/release.yml)
[![GHCR Server](https://img.shields.io/badge/ghcr-server-blue?logo=docker)](https://ghcr.io/portflare/portflare-server)
[![GHCR Client](https://img.shields.io/badge/ghcr-client-blue?logo=docker)](https://ghcr.io/portflare/portflare-client)

Portflare is a Go monorepo for exposing applications running inside containers or on local machines through a central reverse proxy service.

> Note: this project is experimental. For many real-world use cases, you should probably use an existing mature tool instead.

## Layout

- `cmd/reverse-server`: central server, dashboard, approval flow, per-user keys
- `cmd/reverse-client`: lightweight client intended to be embedded in existing containers or used as a sidecar
- `examples/caddy/Caddyfile.example`: example host routing configuration
- `examples/docker/`: example Dockerfiles and Compose snippets for sidecar and embedded-client patterns
- `.github/workflows/ci.yml`: Go test + Docker build validation
- `.github/workflows/docker.yml`: optional GHCR image publishing workflow
- `.github/workflows/release.yml`: tagged binary/archive release workflow

## Alternatives

Before using this project, you should strongly consider established tools in this space:

- [ngrok](https://ngrok.com/)
- [Cloudflare Tunnel](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/)
- [inlets](https://inlets.dev/)
- [frp](https://github.com/fatedier/frp)
- [boringproxy](https://github.com/boringproxy/boringproxy)
- [sish](https://github.com/antoniomika/sish)
- [rathole](https://github.com/rapiz1/rathole)
- [Tailscale Funnel](https://tailscale.com/kb/1223/funnel)

If you want a well-known, production-proven tunnel or reverse-access solution, those are likely better choices today.

This project is most useful if you specifically want to explore or build around:

- a self-hosted multi-user control plane
- auth-header based user identity
- approval workflows for app publication
- per-user and per-app routing patterns like `{app}-{user-label}.<base-domain>`
- a lightweight client that can be embedded into existing container images

## Design goals

- easy to self-host
- easy to open source
- minimal client integration cost
- dynamic per-user and per-app routing
- auth-header based dashboards
- local port mode and remote subdomain mode

## Routing model

Examples only; domains are configurable.

Confirmed public app format:

- `{app}-{user-label}.<base-domain>`

Examples:

- `admin.reverse.example.test` -> admin dashboard
- `{user-label}.reverse.example.test` -> authenticated user dashboard
- `{app}-{user-label}.reverse.example.test` -> open application route
- local fallback: `/r/{user}/{app}`

`user-label` is a normalized public slug derived from the auth user name using only lowercase letters and digits. Example:

- auth user: `alice-smith`
- public user label: `alicesmith`
- public app URL: `web-alicesmith.reverse.example.test`

If two users normalize to the same public label, registration is rejected and the user must choose a different slug.

Users can update their public user label from the `/me` dashboard. The server validates uniqueness and rejects collisions.

Public user label rules:

- lowercase letters and digits only after normalization
- minimum length: 3
- maximum length: 32
- reserved labels are rejected, including: `admin`, `api`, `www`, `static`, `assets`, `me`

When a user changes their public user label, older user-label hosts are kept as aliases and redirected to the latest canonical host.

## App approval controls

The server now supports three approval settings:

- `REVERSE_ALLOW_USER_APP_APPROVAL` -> users can approve their own apps
- `REVERSE_AUTO_APPROVE_APPS_FOR_USERS` -> newly registered user apps are auto-approved
- `REVERSE_AUTO_APPROVE_APPS_FOR_ADMINS` -> newly registered admin apps are auto-approved

These settings are also exposed as admin toggles in the admin dashboard and persisted in server state.

## Authentication

Server dashboard auth is expected to be handled by a front proxy that injects:

- `X-Auth-Request-User`
- `X-Auth-Request-Email`

Clients authenticate using a per-user API key issued by the server.

Issued client keys are prefixed with `pf_`.

The server and client both validate this prefix, so `REVERSE_CLIENT_KEY` must begin with `pf_`.

### Local testing without auth

For local testing only, you can disable dashboard auth entirely:

- `REVERSE_DISABLE_AUTH=true`

Optional local identity overrides:

- `REVERSE_LOCAL_DEV_USER=localdev`
- `REVERSE_LOCAL_DEV_EMAIL=localdev@example.test`

When auth is disabled, the server treats every dashboard request as coming from the configured local development user, and that user has admin access.

## Current code status

The repository contains an initial HTTP-first implementation of:

- server dashboard and approval flow
- per-user API key onboarding
- websocket control connection from client to server
- client-side local registration API
- dynamic HTTP forwarding for approved apps
- server-side dynamic public-port listeners

Current client discovery support:

- optional autodiscovery of listening local HTTP ports from `/proc/net/tcp*`
- allowlist filtering with explicit ports and ranges
- denylist filtering for ports that should never be exposed
- configurable per-port discovery names
- grace period before marking a discovered app offline
- persisted discovered state so restarts do not thrash registrations
- discovered apps are named `app-<port>` by default when no custom name is provided

Key client discovery environment variables:

- `REVERSE_CLIENT_DISCOVER=true`
- `REVERSE_CLIENT_DISCOVER_ALLOW=3000,8080,9000-9100`
- `REVERSE_CLIENT_DISCOVER_DENY=22,2375,2376`
- `REVERSE_CLIENT_DISCOVER_NAMES=3000=web,8080=admin`
- `REVERSE_CLIENT_DISCOVER_INTERVAL=5s`
- `REVERSE_CLIENT_DISCOVER_GRACE=10m`

Planned next steps:

- richer discovery metadata and app naming
- raw TCP mode with explicit opt-in
- shared internal packages
- production hardening and tests

## GitHub Actions

Included workflows:

- `ci.yml`
  - runs `go test ./...`
  - builds the example Docker images
- `docker.yml`
  - builds and publishes images to GHCR on `main`, tags, or manual dispatch
- `release.yml`
  - builds tagged binaries for Linux, macOS, and Windows
  - packages archives with docs and examples
  - creates a GitHub release and uploads artifacts

Default published image names are configured for the intended repository:

- `ghcr.io/portflare/portflare-server`
- `ghcr.io/portflare/portflare-client`
- `ghcr.io/portflare/portflare-embedded-example`

Expected repository:

- `github.com/portflare/portflare`

## Docker integration patterns

### Sidecar pattern

Use this when you do not want to modify the application image.
The reverse client shares the app container network namespace.

Files:

- `examples/docker/client.Dockerfile`
- `examples/docker/compose.sidecar.example.yml`

Key point:

- `network_mode: service:<app-service>` lets the client reach `127.0.0.1:<port>` inside the app container network namespace.

### Embedded client pattern

Use this when you want a single image containing both the app and the reverse client.

Files:

- `examples/docker/app-with-embedded-client.Dockerfile`
- `examples/docker/embedded-entrypoint.sh`
- `examples/docker/compose.embedded.example.yml`

Key point:

- the container entrypoint starts `reverse-client daemon` in the background and then starts the main app process.

## Example client usage

Run the client daemon:

```bash
reverse-client daemon
```

Register an app from inside the container:

```bash
reverse-client expose --app web --target http://127.0.0.1:3000
```

Optionally request a local public port on the server host:

```bash
reverse-client expose --app web --target http://127.0.0.1:3000 --public-port 13000
```

Enable discovery mode inside a container:

```bash
export REVERSE_CLIENT_DISCOVER=true
export REVERSE_CLIENT_DISCOVER_ALLOW=3000,8080,9000-9100
export REVERSE_CLIENT_DISCOVER_DENY=22,2375,2376
export REVERSE_CLIENT_DISCOVER_NAMES=3000=web,8080=admin
reverse-client daemon
```

This will auto-register apps such as `web` for port `3000`, `admin` for port `8080`, and `app-<port>` for matching ports without an explicit name. If a port disappears, the client waits for the configured grace period before marking that app offline.

## Local client API

The client exposes a local API on `REVERSE_CLIENT_LISTEN_ADDR`.

Endpoints:

- `GET /apps` -> all apps
- `GET /apps?source=manual` -> only manual apps
- `GET /apps?source=discovery` -> only discovered apps
- `GET /apps/<app-name>` -> single app details
- `POST /apps` -> create or update a manual app
- `DELETE /apps/<app-name>` -> delete an app from local state
- `POST /discovery/rescan` -> trigger an immediate discovery scan
- `GET /healthz` -> health check

Examples:

```bash
curl http://127.0.0.1:9901/apps
curl http://127.0.0.1:9901/apps?source=discovery
curl -X DELETE http://127.0.0.1:9901/apps/web
curl -X POST http://127.0.0.1:9901/discovery/rescan
```

Example: sidecar usage with discovery enabled:

```bash
docker compose -f examples/docker/compose.sidecar.example.yml up --build
```

In the provided Docker Compose examples, the client connects to the server with:

```bash
REVERSE_SERVER_URL=http://reverse-server:8080
```

That is intentional: both services are on the same Compose network, so the Docker service name works directly. On Linux, `host.docker.internal` is often unavailable unless you add it explicitly.

For easiest local testing, uncomment the `REVERSE_DISABLE_AUTH`, `REVERSE_LOCAL_DEV_USER`, and `REVERSE_LOCAL_DEV_EMAIL` lines in the example compose file.

The example compose files also include commented approval controls:

- `REVERSE_ALLOW_USER_APP_APPROVAL`
- `REVERSE_AUTO_APPROVE_APPS_FOR_USERS`
- `REVERSE_AUTO_APPROVE_APPS_FOR_ADMINS`

Example: embedded client image:

```bash
docker compose -f examples/docker/compose.embedded.example.yml up --build
```
