# x-ware

Infrastructure adapters and middleware for the [telark](https://telark.io) platform. Redis and NATS clients with resilient init, plus HTTP middleware — the plumbing services share instead of re-implementing.

## Packages

| Package | What it provides |
|---|---|
| `redis` | Redis client with retry init, caching, streams, and key invalidation |
| `nats` | NATS client with retry init and a core wrapper |
| `async` | Async / background helpers |
| `authz` | Authorization middleware |
| `cors` | CORS middleware |
| `shared` | Shared helpers |
| `constants` | Shared constants |

The Redis and NATS clients block-with-backoff until the backend is reachable (`NewClientWithRetry`), so services start cleanly during dependency rollout.

## Install

```sh
export GOPRIVATE=github.com/telark/*   # private until public release
go get github.com/telark/x-ware
```

Consumed by the telark services.
