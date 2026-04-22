# Repository guide

## `server`

Repository for the Portflare server.

Owns:

- dashboard
- approval flow
- public routing
- admin settings
- server image publishing

Paths:

- [`../server`](../server)

## `client`

Repository for the Portflare client.

Owns:

- client daemon
- local app registration API
- discovery mode
- client image publishing

Paths:

- [`../client`](../client)

## `client-embedded-example`

Repository for the example embedded app image.

Owns:

- example Dockerfile
- example embedded entrypoint
- sample app packaging pattern
- embedded example image publishing

Paths:

- [`../client-embedded-example`](../client-embedded-example)

## Why split them

The split makes it easier to:

- release server and client independently
- publish cleaner Docker images
- separate implementation concerns
- keep example code out of the main client repository
- keep this root repo focused on high-level docs and project overview
