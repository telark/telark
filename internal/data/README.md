# data

Shared domain types and models for the [telark](https://telark.io) platform. The single source of truth for the data shapes exchanged between services — CRDs, protection plans, insights, auth, and errors.

## Packages

| Package | What it holds |
|---|---|
| `resources` | Application / Group / Role / User custom-resource types |
| `plans` | Protection-plan types and lifecycle states |
| `classification` | Classification / category types |
| `insights` | AI insight types — grounded signals, risks, suggestions |
| `auth` | Passkey, session, and OIDC models |
| `messages` | Event and message payloads |
| `metadata` | Shared metadata types |
| `policies` | Policy-model types |
| `errors` | `errors.Error` — a string-typed error for constant error values |
| `logger` | Logging helpers |
| `constants` | Shared constants |

## Install

```sh
export GOPRIVATE=github.com/telark/*   # private until public release
go get github.com/telark/data
```

Consumed by the telark services and the other shared packages (`rest`, `x-ware`, `kcore`).
