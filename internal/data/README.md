# data

The shared data model of [Telark](https://github.com/telark/telark): the Go types behind every `telark.io/v1alpha1` custom resource, the protection-plan model, the Kyverno policy templates, and the constants and errors the services agree on.

An internal package of the Telark Go module (`github.com/telark/telark/internal/data`): only the Telark services import it, and it changes together with them.

| Package | Contents |
|---|---|
| `metadata/v1alpha1` | Group, version, kind, plural and status projection of the nine CRDs |
| `resources` | `Application`, `User`, `Group`, `AccessRole`, `TelarkConfig` and finalizers |
| `plans` | `ProtectionPlan`, phases, approval and scope |
| `policies` | Policy templates rendered into namespaced Kyverno `Policy` objects, plus their labels |
| `classification` | `Category` (environments, tags, group and role categories) |
| `auth` | `Passkey`, `Session`, session naming |
| `insights` | Insight and recommendation documents written by the analyzer |
| `messages`, `errors`, `logger`, `constants`, `shared` | Event payloads, typed errors, logging and shared constants such as `Condition` |

Used by [`rest`](../rest), [`kcore`](../kcore), [`x-ware`](../x-ware) and the Telark services.
