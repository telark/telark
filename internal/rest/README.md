# rest

The HTTP layer shared by the [Telark](https://github.com/telark/telark) services: endpoint definitions for the `/api/v1` API, typed clients for service-to-service calls, the router and the response envelope.

An internal package of the Telark Go module (`github.com/telark/telark/internal/rest`): only the Telark services import it, and it changes together with them.

| Package | Contents |
|---|---|
| `endpoints` | Every route path and request/response type, one package per resource (`applications`, `protectionplans` in `plans`, `users`, `groups`, `accessroles`, `categories`, `config`, `reports`, `snapshots`, `notifications`, `cluster`, `insights`, `auth`, `cleanup`, `status`) |
| `clients` | Typed clients for the same resources, with the service token attached, path parameters escaped and redirects refused |
| `router` | Route registration and the method-plus-path key the authorization layer uses |
| `response`, `handlers`, `mappers`, `utils` | Response envelope, handler helpers, payload mapping, request parsing with a 1 MiB body cap |
| `server`, `connectivity`, `base`, `constants` | Server lifecycle, peer readiness, service base URLs, constants |

Depends on [`data`](../data).
