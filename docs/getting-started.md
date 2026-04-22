# Getting started

## What Portflare does

Portflare lets you expose apps running locally or in containers through a central server.

You usually deploy:

- a **server** in a public location
- one or more **clients** near the apps you want to expose

## Components

### Server

The server handles:

- dashboard and admin pages
- user identity integration
- app approval flow
- public routing

See:

- [`github.com/portflare/server`](https://github.com/portflare/server)

### Client

The client handles:

- connection to the server
- app registration
- optional local port discovery
- local API for managing registered apps

See:

- [`github.com/portflare/client`](https://github.com/portflare/client)

### Protocol

Shared wire-level contracts and lightweight validation helpers live in:

- [`github.com/portflare/protocol`](https://github.com/portflare/protocol)

### Embedded example

If you want to package your app and the client in one image, use:

- [`github.com/portflare/client-embedded-example`](https://github.com/portflare/client-embedded-example)

## Typical flow

1. Run the server.
2. Obtain a client key.
3. Start the client with `REVERSE_SERVER_URL` and `REVERSE_CLIENT_KEY`.
4. Register an app manually or use discovery mode.
5. Approve the app if approval is required.
6. Access it on its public URL.

## Minimal client example

```bash
export REVERSE_SERVER_URL=https://r.myw.io
export REVERSE_CLIENT_KEY=pf_your_key_here
reverse-client daemon
reverse-client expose --app web --target http://127.0.0.1:3000
```

## Where to go next

- [Architecture overview](./architecture.md)
- [Repository guide](./repositories.md)
