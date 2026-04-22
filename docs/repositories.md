# Repository guide

## `server`

Repository for the Portflare server.

Owns:

- dashboard
- approval flow
- public routing
- admin settings
- server image publishing

Repository:

- [`github.com/portflare/server`](https://github.com/portflare/server)

## `client`

Repository for the Portflare client.

Owns:

- client daemon
- local app registration API
- discovery mode
- client image publishing

Repository:

- [`github.com/portflare/client`](https://github.com/portflare/client)

## `protocol`

Repository for shared wire-level contracts.

Owns:

- shared protocol structs
- shared message type constants
- lightweight validation helpers used by both sides

Repository:

- [`github.com/portflare/protocol`](https://github.com/portflare/protocol)

## `client-embedded-example`

Repository for the example embedded app image.

Owns:

- example Dockerfile
- example embedded entrypoint
- sample app packaging pattern
- embedded example image publishing

Repository:

- [`github.com/portflare/client-embedded-example`](https://github.com/portflare/client-embedded-example)

## Why split them

The split makes it easier to:

- release server and client independently
- keep shared wire contracts small and intentional
- publish cleaner Docker images
- separate implementation concerns
- keep example code out of the main client repository
- keep this root repo focused on high-level docs and project overview
