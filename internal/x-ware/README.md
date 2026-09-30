# x-ware

Middleware and infrastructure clients shared by the [Telark](https://github.com/telark/telark) services: the authorization layer every API goes through, CORS, and Redis and NATS clients.

An internal package of the Telark Go module (`github.com/telark/telark/internal/x-ware`): only the Telark services import it, and it changes together with them.

| Package | Contents |
|---|---|
| `authz` | Request authentication (session or service token) and route requirements: levels per scope, deny rules, unmapped routes denied. See the [security model](https://github.com/telark/telark/blob/main/docs/security/README.md) |
| `cors` | CORS from `CORS_ALLOWED_ORIGINS`; no CORS headers when it is empty |
| `redis` | Client with retry on start, cache, streams, locks and leader election |
| `nats` | JetStream client with retry on start |
| `async`, `shared`, `constants` | Worker pool, helpers, constants |

Clients block with backoff until Redis or NATS is reachable, so services start cleanly while their dependencies roll out.

Depends on [`data`](../data) and [`rest`](../rest).
