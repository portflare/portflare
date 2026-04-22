# Portflare

Portflare is a self-hosted way to publish apps running on local machines or inside containers through a central reverse proxy.

It is built for teams that want:

- simple app publishing
- per-user and per-app URLs
- approval flows for exposed apps
- a lightweight client that can run as a sidecar or inside an existing image
- a self-hosted control plane

## Repositories

Portflare is now split into focused repositories:

- [`server/`](./server) — the Portflare server, dashboard, approval flow, and routing control plane
- [`client/`](./client) — the Portflare client daemon and local API
- [`client-embedded-example/`](./client-embedded-example) — an example app image that embeds the client

This root repository should now act primarily as a lightweight landing page, docs hub, and project overview.

## How it works

1. The **server** provides the dashboard, user management hooks, approval flow, and public routing.
2. The **client** connects to the server using a per-user client key.
3. The client registers local apps either manually or by discovery.
4. Approved apps become reachable on public Portflare URLs.

Typical route pattern:

- `{app}-{user-label}.<base-domain>`

Example:

- `web-alicesmith.r.myw.io`

## Quick start

### Run the client

```bash
export REVERSE_SERVER_URL=https://r.myw.io
export REVERSE_CLIENT_KEY=pf_your_key_here
reverse-client daemon
```

### Expose a local app

```bash
reverse-client expose --app web --target http://127.0.0.1:3000
```

### Embedded-image pattern

If you want one container image that runs both your app and Portflare, start from:

- [`client-embedded-example/`](./client-embedded-example)

## Documentation

- [Docs index](./docs/README.md)
- [Architecture overview](./docs/architecture.md)
- [Getting started](./docs/getting-started.md)
- [Repository guide](./docs/repositories.md)

## Status

Portflare is still evolving, but the repositories are now split so the server, client, and embedded example can be built, released, and documented independently.
