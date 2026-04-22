# Architecture overview

Portflare has four primary pieces.

## 1. Server

The server is the public control plane.

Responsibilities:

- authenticate dashboard users through a front proxy
- issue and validate client keys
- track users and registered apps
- manage approval state
- route public traffic to approved apps

Repository:

- [`github.com/portflare/server`](https://github.com/portflare/server)

## 2. Client

The client is the agent that runs near the target app.

Responsibilities:

- connect to the server
- register apps
- watch discovered ports when discovery mode is enabled
- proxy traffic between the server and the local app

Repository:

- [`github.com/portflare/client`](https://github.com/portflare/client)

## 3. Protocol

The protocol repository holds shared wire-level contracts.

Responsibilities:

- shared websocket message types
- shared JSON payload structs
- lightweight cross-repo validation helpers
- protocol constants that should stay consistent between server and client

Repository:

- [`github.com/portflare/protocol`](https://github.com/portflare/protocol)

## 4. Embedded example

The embedded example shows how to bundle an app and the client into one image.

Repository:

- [`github.com/portflare/client-embedded-example`](https://github.com/portflare/client-embedded-example)

## Dependency guidance

The preferred relationship is:

- `server` is independent
- `client` is independent
- `protocol` is shared by `server` and `client`
- `client-embedded-example` depends on `client`

Avoid making `client` import `server` implementation packages.

## Routing model

Common public host pattern:

- `{app}-{user-label}.<base-domain>`

Example:

- `web-alicesmith.r.myw.io`
