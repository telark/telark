# rest

HTTP toolkit for service-to-service REST in the [telark](https://telark.io) platform. Typed clients, a server harness, routing, and response envelopes — so services call each other the same way everywhere.

## Packages

| Package | What it provides |
|---|---|
| `clients` | Typed clients for calling other telark services |
| `endpoints` | Endpoint path definitions |
| `base` | Service base URLs |
| `router` | Route registration |
| `handlers` | Handler helpers |
| `response` | Typed response envelopes (e.g. `GenericResponse`) |
| `mappers` | DTO / model mapping |
| `server` | Server lifecycle (signal handling, listen loop) |
| `connectivity` | Health and reachability checks |
| `utils` | Shared helpers |
| `constants` | Shared constants |

## Install

```sh
export GOPRIVATE=github.com/telark/*   # private until public release
go get github.com/telark/rest
```

Depends on [`data`](https://github.com/telark/data); consumed by the telark services.
