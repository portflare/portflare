# Local testing guide

This document explains how to test `Portflare` locally and how to choose an approval mode.

## Recommended local setup

For the easiest local workflow:

- run the server with auth disabled
- use Docker Compose service-to-service networking
- use the sidecar example first

Relevant example files:

- `examples/docker/compose.sidecar.example.yml`
- `examples/docker/compose.embedded.example.yml`
- `examples/caddy/Caddyfile.example`

## Important networking note

In the provided Docker Compose examples, the client connects to the server with:

```yaml
REVERSE_SERVER_URL: "http://reverse-server:8080"
```

This is intentional.
Both services are on the same Compose network, so the Docker service name works directly.

Do **not** use `host.docker.internal` unless you explicitly configure it for your environment.

---

## Local auth-disabled mode

For local testing only, you can disable dashboard auth entirely:

```yaml
REVERSE_DISABLE_AUTH: "true"
REVERSE_LOCAL_DEV_USER: "alice-smith"
REVERSE_LOCAL_DEV_EMAIL: "alice@example.com"
```

When auth is disabled:

- no `X-Auth-Request-User` header is required
- no `X-Auth-Request-Email` header is required
- the configured local dev user is treated as the active dashboard user
- that user has admin access

This makes it easy to test:

- `/admin`
- `/me`
- approval flows
- websocket/live dashboard updates

---

## Approval modes

The server supports three approval controls:

- `REVERSE_ALLOW_USER_APP_APPROVAL`
- `REVERSE_AUTO_APPROVE_APPS_FOR_USERS`
- `REVERSE_AUTO_APPROVE_APPS_FOR_ADMINS`

These settings are also available as toggles in the admin dashboard.

### 1. Strict admin-only approval

Use this to test the full approval workflow.

```yaml
REVERSE_DISABLE_AUTH: "true"
REVERSE_LOCAL_DEV_USER: "alice-smith"
REVERSE_LOCAL_DEV_EMAIL: "alice@example.com"

REVERSE_ALLOW_USER_APP_APPROVAL: "false"
REVERSE_AUTO_APPROVE_APPS_FOR_USERS: "false"
REVERSE_AUTO_APPROVE_APPS_FOR_ADMINS: "false"
```

Behavior:

- users cannot approve their own apps
- admins must approve all apps
- best for testing pending-state behavior and admin approval flow

### 2. User self-approval

Use this to test self-service approval.

```yaml
REVERSE_DISABLE_AUTH: "true"
REVERSE_LOCAL_DEV_USER: "alice-smith"
REVERSE_LOCAL_DEV_EMAIL: "alice@example.com"

REVERSE_ALLOW_USER_APP_APPROVAL: "true"
REVERSE_AUTO_APPROVE_APPS_FOR_USERS: "false"
REVERSE_AUTO_APPROVE_APPS_FOR_ADMINS: "false"
```

Behavior:

- newly registered apps start pending
- users can approve their own apps from `/me`
- admins can still approve them too

### 3. Auto-approve everything for local dev

Use this for the lowest-friction local setup.

```yaml
REVERSE_DISABLE_AUTH: "true"
REVERSE_LOCAL_DEV_USER: "alice-smith"
REVERSE_LOCAL_DEV_EMAIL: "alice@example.com"

REVERSE_ALLOW_USER_APP_APPROVAL: "true"
REVERSE_AUTO_APPROVE_APPS_FOR_USERS: "true"
REVERSE_AUTO_APPROVE_APPS_FOR_ADMINS: "true"
```

Behavior:

- apps are approved immediately on registration
- public routes usually work without manual approval
- best for testing discovery, routing, and live UI updates

---

## Recommended local-dev server block

Here is a good block to paste under `reverse-server.environment` in a compose file:

```yaml
environment:
  REVERSE_SERVER_LISTEN_ADDR: ":8080"
  REVERSE_BASE_DOMAIN: "reverse.example.test"
  REVERSE_STATE_PATH: "/data/state.json"
  REVERSE_ADMIN_USERS: "admin"

  REVERSE_DISABLE_AUTH: "true"
  REVERSE_LOCAL_DEV_USER: "alice-smith"
  REVERSE_LOCAL_DEV_EMAIL: "alice@example.com"

  REVERSE_ALLOW_USER_APP_APPROVAL: "true"
  REVERSE_AUTO_APPROVE_APPS_FOR_USERS: "true"
  REVERSE_AUTO_APPROVE_APPS_FOR_ADMINS: "true"
```

---

## Starting the sidecar example

From the project root:

```bash
docker compose -f examples/docker/compose.sidecar.example.yml down
docker compose -f examples/docker/compose.sidecar.example.yml up --build
```

Make sure you have replaced:

```yaml
REVERSE_CLIENT_KEY: "replace-me"
```

with a real client key from the user dashboard.

---

## Basic local test flow

### 1. Open the user dashboard

```bash
curl http://127.0.0.1:8080/me
```

You should see:

- the local dev identity
- the generated connection key
- public user label
- current applications

### 2. Open the admin dashboard

```bash
curl http://127.0.0.1:8080/admin
```

You should see:

- registration status
- approval settings
- users
- applications

### 3. Start app + reverse client

With the sidecar example running, the sample app listens on port `3000` and the client discovers it.

### 4. Check the public route

Use the `Host` header locally:

```bash
curl -H 'Host: web-alicesmith.reverse.example.test' \
  http://127.0.0.1:8080/
```

If auto-approve is enabled, this should work immediately.

If strict approval is enabled, approve first and then retry.

---

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
curl http://127.0.0.1:9901/apps/web
curl -X DELETE http://127.0.0.1:9901/apps/web
curl -X POST http://127.0.0.1:9901/discovery/rescan
```

Note: in the sidecar example, the client API is inside the client container namespace. You may prefer to inspect logs or exec into the container for these checks.

---

## Discovery settings

Supported discovery environment variables:

```yaml
REVERSE_CLIENT_DISCOVER: "true"
REVERSE_CLIENT_DISCOVER_ALLOW: "3000,8080,9000-9100"
REVERSE_CLIENT_DISCOVER_DENY: "22,2375,2376"
REVERSE_CLIENT_DISCOVER_NAMES: "3000=web,8080=admin"
REVERSE_CLIENT_DISCOVER_INTERVAL: "5s"
REVERSE_CLIENT_DISCOVER_GRACE: "10m"
```

Behavior:

- listening ports are discovered from `/proc/net/tcp*`
- allowlist controls which ports are considered
- denylist excludes ports that should never be exposed
- custom names can be assigned by port
- if a discovered port disappears, it is not removed immediately
- it is marked offline only after the grace period expires

---

## Live UI updates

The admin and user dashboards use WebSockets for live updates.

Current behavior:

- the page opens a websocket to `/ws/ui`
- server state changes emit refresh notifications
- the UI fetches fresh JSON state and updates the page in place

This means:

- no manual refresh should be needed for most changes
- approvals, key rotation, connection changes, and settings toggles should appear live

---

## Troubleshooting

### `missing X-Auth-Request-User header`

This means auth is still enabled.

Make sure these are actually enabled in the server environment:

```yaml
REVERSE_DISABLE_AUTH: "true"
REVERSE_LOCAL_DEV_USER: "alice-smith"
REVERSE_LOCAL_DEV_EMAIL: "alice@example.com"
```

Then recreate the server container.

### `lookup host.docker.internal ... no such host`

Use:

```yaml
REVERSE_SERVER_URL: "http://reverse-server:8080"
```

inside Docker Compose.

### Sample app logs do not appear again

Compose may have reused the container.
Force restart everything:

```bash
docker compose -f examples/docker/compose.sidecar.example.yml down
docker compose -f examples/docker/compose.sidecar.example.yml up --build
```

---

## Suggested testing matrix

### Strict admin-only

Expected:

- app appears pending
- admin approves it
- public route works after approval

### Self-approve

Expected:

- app appears pending
- user approves it from `/me`
- public route works after approval

### Auto-approve

Expected:

- app works immediately after discovery or registration

---

## Current status

`Portflare` is ready for local/dev workflow testing, including:

- server UI
- user/admin dashboards
- local auth-disabled mode
- sidecar Docker setup
- embedded Docker setup
- discovery flow
- approval flow
- live websocket UI updates
- public app proxy routing

It is still a v1/prototype and not yet fully production-hardened.
