# Architecture overview

Portflare has three primary pieces.

## 1. Server

The server is the public control plane.

Responsibilities:

- authenticate dashboard users through a front proxy
- issue and validate client keys
- track users and registered apps
- manage approval state
- route public traffic to approved apps

Repository:

- [`../server`](../server)

## 2. Client

The client is the agent that runs near the target app.

Responsibilities:

- connect to the server
- register apps
- watch discovered ports when discovery mode is enabled
- proxy traffic between the server and the local app

Repository:

- [`../client`](../client)

## 3. Embedded example

The embedded example shows how to bundle an app and the client into one image.

Repository:

- [`../client-embedded-example`](../client-embedded-example)

## Dependency guidance

The preferred relationship is:

- `server` is independent
- `client` is independent
- `client-embedded-example` depends on `client`

Avoid making `client` import `server` implementation packages.

If server and client need common protocol or validation helpers later, add a small shared package or repository specifically for those wire-level concerns.

## Routing model

Common public host pattern:

- `{app}-{user-label}.<base-domain>`

Example:

- `web-alicesmith.r.myw.io`
