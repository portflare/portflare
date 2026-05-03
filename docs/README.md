# Portflare docs

This repository is now the top-level docs and overview hub for Portflare.

## Start here

- [Getting started](./getting-started.md)
- [Architecture overview](./architecture.md)
- [Repository guide](./repositories.md)
- [Deployment learnings](./learnings.md)
- [Phase 1: Auth-backed CLI registration](./auth-registration-phase-1.md)
- [Phase 2: Server auth-service handoff](./auth-dashboard-phase-2.md)
- [Auth handoff scout review](./auth-handoff-scout-review.md)
- [Auth handoff implementation plan](./auth-handoff-phase-plan.md)
- [Protocol auth worker notes](./auth-handoff-protocol-worker.md)
- [Protocol auth review notes](./auth-handoff-protocol-review.md)
- [Auth service worker notes](./auth-handoff-auth-service-worker.md)
- [Auth service review notes](./auth-handoff-auth-service-review.md)

## Product overview

Portflare is split into separate repositories:

- [`github.com/portflare/server`](https://github.com/portflare/server) — server, dashboard, approval flow, routing control plane
- [`github.com/portflare/client`](https://github.com/portflare/client) — client daemon, discovery, local registration API
- [`github.com/portflare/protocol`](https://github.com/portflare/protocol) — shared protocol types and validation helpers
- [`github.com/portflare/client-embedded-example`](https://github.com/portflare/client-embedded-example) — example embedded image

## Repo-specific docs

For implementation and operational detail, see each repo directly:

- [`server/docs/`](https://github.com/portflare/server/tree/main/docs)
- [`client/docs/`](https://github.com/portflare/client/tree/main/docs)
- [`protocol/README.md`](https://github.com/portflare/protocol)
- [`client-embedded-example/docs/`](https://github.com/portflare/client-embedded-example/tree/main/docs)
