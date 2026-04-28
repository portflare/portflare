# Portflare deployment learnings

This document captures practical lessons from deploying Portflare in a homelab/Docker environment. It focuses on the operational details that are easy to miss: registration flow, Docker networking, sidecar lifecycle, discovery behavior, Compose validation, secrets, and persistence.

## 1. Mental model

Portflare has two runtime pieces:

1. **Server**
   - Runs centrally.
   - Owns users, API keys, app approvals, dashboard state, and public routing.
   - Receives websocket connections from clients.
   - Proxies public requests back through the connected client.

2. **Client**
   - Runs near the application being exposed.
   - Connects outbound to the server using `PORTFLARE_SERVER_URL` and `PORTFLARE_CLIENT_KEY`.
   - Registers one or more local apps with target URLs such as `http://127.0.0.1:3000`.
   - Handles proxied requests from the server and forwards them to the local app.

The most important operational point is that **the client must be able to reach the target URL from inside the client's own network namespace**. If the client cannot reach `http://127.0.0.1:8100`, Portflare cannot proxy that app, even if the app is reachable from the host browser.

## 2. Server deployment behind Caddy

For the homelab setup, the server runs as a Docker service on port `8080` internally and is reached through Caddy.

Important server environment variables:

```yaml
PORTFLARE_SERVER_LISTEN_ADDR: ":8080"
PORTFLARE_BASE_DOMAIN: r.myw.io
PORTFLARE_STATE_PATH: /var/lib/portflare/state.json
PORTFLARE_ADMIN_USERS: bjorn@myw.io
PORTFLARE_DISABLE_AUTH: ${PORTFLARE_DISABLE_AUTH:-false}
PORTFLARE_TRUST_AUTH_HEADERS: ${PORTFLARE_TRUST_AUTH_HEADERS:-true}
PORTFLARE_REGISTRATION_OPEN: ${PORTFLARE_REGISTRATION_OPEN:-true}
PORTFLARE_ALLOW_USER_APP_APPROVAL: ${PORTFLARE_ALLOW_USER_APP_APPROVAL:-false}
PORTFLARE_AUTO_APPROVE_APPS_FOR_USERS: ${PORTFLARE_AUTO_APPROVE_APPS_FOR_USERS:-false}
PORTFLARE_AUTO_APPROVE_APPS_FOR_ADMINS: ${PORTFLARE_AUTO_APPROVE_APPS_FOR_ADMINS:-false}
```

The server listens on `:8080`, but it does not need to publish that port to the host if Caddy is on the same Docker network. In Compose, use `expose`, not `ports`:

```yaml
expose:
  - "8080"
```

This makes the port visible to containers on the Docker network while avoiding host exposure.

Caddy labels can then route public domains to the service:

```yaml
labels:
  caddy: "admin.r.myw.io *.r.myw.io"
  caddy.reverse_proxy: "{{upstreams 8080}}"
```

### Why `expose` instead of `ports`?

- `expose: ["8080"]` makes port 8080 available to other containers on Docker networks.
- `ports: ["8080:8080"]` publishes port 8080 on the host.
- For a public reverse-proxy deployment, Caddy should be the only public ingress. The Portflare server container should not need direct host exposure.

## 3. User registration flow

Portflare server currently does not have a separate sign-up form. A user is created automatically when they first access their user page while authenticated.

Typical flow:

1. User logs in through the front auth proxy.
2. Auth proxy forwards identity headers to Portflare:

   ```text
   X-Auth-Request-User
   X-Auth-Request-Email
   ```

3. User opens:

   ```text
   https://admin.r.myw.io/me
   ```

   or:

   ```text
   https://<user-label>.r.myw.io
   ```

4. The server derives a user identity from the auth headers.
5. If the user does not exist and registration is open, the server creates the user and generates an API key.

Registration is controlled by state and initialized from:

```yaml
PORTFLARE_REGISTRATION_OPEN: ${PORTFLARE_REGISTRATION_OPEN:-true}
```

Admins can toggle registration in the admin dashboard:

```text
https://admin.r.myw.io/admin
```

### Practical implication

A new user does not need a manual invitation record in the state file as long as registration is open and the auth proxy provides a valid identity. If registration is closed, first-time users will fail with a registration-closed error.

## 4. API keys and secrets

The client needs a user API key:

```env
PORTFLARE_CLIENT_KEY=pf_your_key_here
```

For Compose deployments, put this in `.env` and ensure `.env` is ignored by Git.

If a real `PORTFLARE_CLIENT_KEY` appears in chat logs, shell history, CI logs, screenshots, or a public issue, treat it as compromised and rotate it from the Portflare user page.

Avoid hardcoding client keys into committed Compose files. It is acceptable to hardcode non-secret deployment values, such as:

```yaml
PORTFLARE_SERVER_URL: https://r.myw.io
PORTFLARE_BASE_DOMAIN: r.myw.io
```

but API keys should remain local secrets.

## 5. Docker Compose sidecar pattern

A common deployment pattern is:

- one main app container
- one `ghcr.io/portflare/client:latest` sidecar container
- both containers share a network namespace

Example:

```yaml
services:
  pi:
    image: mywio/pi-coding-agent:latest
    environment:
      PORTFLARE_EMBEDDED_CLIENT: "false"
    volumes:
      - ./data:/home/pi/.pi

  portflare:
    image: ghcr.io/portflare/client:latest
    depends_on:
      - pi
    network_mode: "service:pi"
    restart: unless-stopped
    environment:
      PORTFLARE_SERVER_URL: https://r.myw.io
      PORTFLARE_CLIENT_KEY: ${PORTFLARE_CLIENT_KEY}
      PORTFLARE_CLIENT_LISTEN_ADDR: 127.0.0.1:9901
      PORTFLARE_CLIENT_STATE_PATH: /state/state.json
      PORTFLARE_CLIENT_DISCOVER: "true"
      PORTFLARE_CLIENT_DISCOVER_ALLOW: 3000,8080,9000-9100
      PORTFLARE_CLIENT_DISCOVER_DENY: 22,2375,2376
      PORTFLARE_CLIENT_DISCOVER_NAMES: 3000=web,8080=admin
    volumes:
      - ./data/portflare-client:/state
```

`network_mode: "service:pi"` means the Portflare sidecar shares the `pi` container's network namespace. In that mode:

- `127.0.0.1` in the sidecar is the same as `127.0.0.1` in the app container.
- Discovery can see ports listening in that shared namespace.
- The sidecar local API at `127.0.0.1:9901` is reachable from the app container.

### Compose only starts sidecars when Compose is used

Running the image directly with `docker run mywio/pi-coding-agent:latest` starts only that one container. It does not start the Compose-defined Portflare sidecar.

Use Compose when you want Compose services:

```bash
docker compose up -d --build
```

Then inspect the sidecar:

```bash
docker compose logs -f portflare
```

## 6. Plain `docker run` options

If not using Compose, there are three main options.

### Option A: embedded client inside the app container

Use this when the image already includes the Portflare client. Pass the client environment variables directly to the app container and do not disable the embedded client:

```bash
docker run --rm -it \
  --name my-app \
  -e PORTFLARE_SERVER_URL=https://r.myw.io \
  -e PORTFLARE_CLIENT_KEY \
  -e PORTFLARE_CLIENT_DISCOVER=true \
  -e PORTFLARE_CLIENT_DISCOVER_ALLOW=3000,8080,9000-9100 \
  -e PORTFLARE_CLIENT_DISCOVER_NAMES=3000=web,8080=admin \
  my-app-image
```

This gives the client the same localhost as the app because both run in one container.

### Option B: manual sidecar sharing one container's network namespace

Start the app first:

```bash
docker run -d --name my-app my-app-image
```

Then start Portflare sharing that exact container's network namespace:

```bash
docker run -d \
  --name my-app-portflare \
  --network container:my-app \
  -e PORTFLARE_SERVER_URL=https://r.myw.io \
  -e PORTFLARE_CLIENT_KEY \
  -e PORTFLARE_CLIENT_LISTEN_ADDR=127.0.0.1:9901 \
  -e PORTFLARE_CLIENT_STATE_PATH=/state/state.json \
  -e PORTFLARE_CLIENT_DISCOVER=true \
  -e PORTFLARE_CLIENT_DISCOVER_ALLOW=3000,8080,9000-9100 \
  -e PORTFLARE_CLIENT_DISCOVER_NAMES=3000=web,8080=admin \
  -v "$HOME/.config/portflare-client:/state" \
  ghcr.io/portflare/client:latest
```

This is the closest `docker run` equivalent to a Compose sidecar.

### Option C: one shared Portflare client for many containers

This is possible, but it is not the same as a sidecar.

Create a shared network:

```bash
docker network create portflare-shared
```

Run app containers on it:

```bash
docker run -d --network portflare-shared --name project-a project-a-image
docker run -d --network portflare-shared --name project-b project-b-image
```

Run one Portflare client on the same network:

```bash
docker run -d \
  --name portflare \
  --network portflare-shared \
  -e PORTFLARE_SERVER_URL=https://r.myw.io \
  -e PORTFLARE_CLIENT_KEY \
  -e PORTFLARE_CLIENT_LISTEN_ADDR=0.0.0.0:9901 \
  -e PORTFLARE_CLIENT_STATE_PATH=/state/state.json \
  -v "$HOME/.config/portflare-client:/state" \
  ghcr.io/portflare/client:latest
```

Then register explicit targets:

```bash
portflare expose --app project-a-web --target http://project-a:3000
portflare expose --app project-b-web --target http://project-b:3000
```

This pattern is useful when you want one central client, but automatic localhost discovery is less useful because the shared client does not share each app container's `127.0.0.1`.

## 7. `localhost` vs `0.0.0.0`

This is the most common Docker networking trap.

### If app and client share a network namespace

Examples:

- embedded client in the same container
- sidecar with `network_mode: "service:app"`
- sidecar with `--network container:app`

Then the app can listen on:

```text
127.0.0.1:8100
```

and the client can reach:

```text
http://127.0.0.1:8100
```

### If app and client are separate containers on a Docker network

Then `127.0.0.1` in the client is the client container, not the app container. The app must listen on:

```text
0.0.0.0:8100
```

and the target should use the app container name:

```text
http://app-container-name:8100
```

Listening on `0.0.0.0` inside a container does not publish the port to the host by itself. Host publication only happens with Docker `ports:` or `-p`.

## 8. Discovery behavior

The client's discovery mode scans listening TCP ports from the client's own network namespace. It discovers ports that appear local to the client and registers targets like:

```text
http://127.0.0.1:<port>
```

Discovery configuration:

```env
PORTFLARE_CLIENT_DISCOVER=true
PORTFLARE_CLIENT_DISCOVER_ALLOW=3000,8080,9000-9100
PORTFLARE_CLIENT_DISCOVER_DENY=22,2375,2376
PORTFLARE_CLIENT_DISCOVER_NAMES=3000=web,8080=admin
PORTFLARE_CLIENT_DISCOVER_INTERVAL=5s
PORTFLARE_CLIENT_DISCOVER_GRACE=10m
```

Discovery is best suited for:

- embedded client mode
- one sidecar per app container
- network namespace sharing

Discovery is not a Docker-wide service scanner. It does not automatically inspect unrelated containers. If a dashboard in another container listens on `localhost:8100`, a separate Portflare client container will not see it unless they share the same network namespace.

## 9. Sidecar lifecycle

Plain Docker and Docker Compose do not automatically stop a sidecar when the main app exits.

Important details:

- `depends_on` controls startup order only.
- `depends_on` does not mean the sidecar exits when the app exits.
- `restart: unless-stopped` can keep the sidecar running even after the app is gone.
- `--network container:<app>` ties networking to one app container, but it is still not a full lifecycle supervisor.

Options if lifecycle coupling matters:

1. Run the app and client in one embedded container.
2. Stop the whole Compose project with `docker compose down`.
3. Add a supervisor script that exits the sidecar when the app is no longer healthy.
4. Use a higher-level orchestrator that has pod semantics.

## 10. Local client API

The client exposes a local API using:

```env
PORTFLARE_CLIENT_LISTEN_ADDR=127.0.0.1:9901
```

When a sidecar shares the app network namespace, the app container can talk to the sidecar API at:

```text
http://127.0.0.1:9901
```

This is useful for commands such as:

```bash
portflare expose --app web --target http://127.0.0.1:3000
portflare list
```

If using one shared client for many containers on a Docker network, bind the API to `0.0.0.0:9901` only if other containers need to call it:

```env
PORTFLARE_CLIENT_LISTEN_ADDR=0.0.0.0:9901
```

Be careful with this. The local API should remain private to trusted containers/networks.

## 11. Persistence and backups

Server state is stored at:

```text
/var/lib/portflare/state.json
```

with a Docker volume such as:

```yaml
volumes:
  - portflare-state:/var/lib/portflare
```

Back up the server volume:

```bash
docker run --rm \
  -v portflare-state:/volume:ro \
  -v "$PWD":/backup \
  alpine \
  tar czf /backup/portflare-state-backup.tar.gz -C /volume .
```

Restore it:

```bash
docker compose down

docker run --rm \
  -v portflare-state:/volume \
  -v "$PWD":/backup \
  alpine \
  sh -c "cd /volume && tar xzf /backup/portflare-state-backup.tar.gz"

docker compose up -d
```

Client state should also be persisted if you want registrations/approval status mirrored locally across restarts:

```yaml
volumes:
  - ./data/portflare-client:/state
environment:
  PORTFLARE_CLIENT_STATE_PATH: /state/state.json
```

## 12. Compose validation

`docker compose config` is the best quick validation step for a Compose file:

```bash
docker compose config
```

If the host only has Docker CLI but not the Compose plugin, this command fails before validating anything. That does not mean the Compose file is invalid; it means the validation tool is missing.

Install Docker Compose v2 or validate on a host where `docker compose` works.

## 13. Expected public URLs

With base domain:

```text
r.myw.io
```

and user label:

```text
bjorn
```

apps commonly appear as:

```text
https://web-bjorn.r.myw.io
https://admin-bjorn.r.myw.io
```

The admin dashboard is:

```text
https://admin.r.myw.io
```

The user page is:

```text
https://bjorn.r.myw.io
```

## 14. Troubleshooting checklist

### Client connects but app is not reachable

Check from inside the client container or shared namespace:

```bash
curl -v http://127.0.0.1:8100
```

If using separate containers on a bridge network, check:

```bash
curl -v http://app-container-name:8100
```

If `localhost` fails but container-name works, register the container-name target explicitly.

### Discovery did not find a port

Confirm:

- `PORTFLARE_CLIENT_DISCOVER=true`
- the port is included in `PORTFLARE_CLIENT_DISCOVER_ALLOW`
- the port is not included in `PORTFLARE_CLIENT_DISCOVER_DENY`
- the app is listening in the same network namespace as the client
- the app is actually listening, not just printing a URL

### App prints `http://localhost:8100` but Portflare does not detect it

The app may be listening only on loopback inside a different container. Start it on `0.0.0.0:8100` if Portflare is separate on a Docker network, or run Portflare with `--network container:<app>` if you want localhost discovery.

### Sidecar did not start

If you used `docker run` for the main app, Compose sidecars will not start. Use `docker compose up` or manually start the sidecar with `docker run`.

### Sidecar keeps running after app exits

This is normal Docker behavior. Stop it explicitly, stop the Compose project, use embedded mode, or add lifecycle supervision.

## 15. Recommended patterns

### Best for simple single-container apps

Use embedded mode. It gives the simplest networking and lifecycle behavior.

### Best for Compose apps

Use one Portflare sidecar per app service with:

```yaml
network_mode: "service:app"
```

This preserves localhost discovery and keeps the Compose file explicit.

### Best for many apps with one shared Portflare client

Use a shared Docker network and explicit app targets. Do not rely on discovery for unrelated containers.

### Best for server exposure

Run the Portflare server behind Caddy on a shared Docker network. Use `expose`, not host `ports`, unless there is a specific reason to publish the server directly.
