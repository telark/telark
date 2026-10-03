# auth-service subcommands

Run as `./main <subcommand> [flags]` inside the auth container (the image entry point is `./main`). `main.go` delegates to `cmd.Dispatch`; without a recognized subcommand, the auth HTTP server starts normally.

## `break-glass`

Create, promote or recover the bootstrap admin, the account whose email is `BOOTSTRAP_ADMIN`. Any other email, or an unset `BOOTSTRAP_ADMIN`, is refused before anything is read or written, so the command never creates or promotes another account (other users get enrollment links from Members). With `--enroll` it creates the bootstrap account when missing: that is how the first administrator enrolls on a passkey-only install, and how the bootstrap admin recovers access. It is the only path that grants Admin from an email nobody has verified, which is why it is an operator-run subcommand and not an API. It also removes any non-passkey (Google) identity from the account it promotes, so the bootstrap admin signs in with a passkey only.

**Usage:**
```bash
./main break-glass --email <email>
./main break-glass --email <email> --enroll
```

**Flags:**
| Flag | Required | What it does |
|------|----------|--------------|
| `--email` | yes | The bootstrap admin's email (`BOOTSTRAP_ADMIN`); any other email is refused |
| `--enroll` | no | Create the bootstrap admin when missing (Admin role, `bootstrap: true`) and print a one-time passkey enrollment token (10 minutes, needs Redis). Open `/register?enroll=<token>` in the dashboard to register the passkey. |

**Exit codes:**
- `0`: user promoted (or created), or already had Admin
- `1`: email is not `BOOTSTRAP_ADMIN` (or none is set), user not found (without `--enroll`), create or patch failed, Redis unavailable, or `--email` missing
- `2`: unknown or malformed flag

## `backfill-finalizers`

Manual operator tool: add the cleanup finalizer to every User, Group and AccessRole CR that lacks it. Idempotent, so safe to re-run.

Normally not needed: the auth-service controller's cleanup sweep (every `CLEANUP_SWEEPER_INTERVAL_SECONDS`) already adds the finalizer to any CR it sees without one. Use this subcommand for a single explicit pass (for example during a migration window) instead of waiting for the controller to converge.

**Usage** (run inside an auth-service pod, or via a one-off Job/`kubectl run` against the auth image):
```bash
./main backfill-finalizers
```

No flags. Behavior controlled by env vars:

| Env | Default | What it does |
|-----|---------|--------------|
| `BACKFILL_BATCH_SIZE` | `10` | Number of patches per batch before pausing |
| `BACKFILL_BATCH_PAUSE_MS` | `100` | Pause between batches, in milliseconds (rate-limits the exporter) |

**Exit codes:**
- `0`: backfill completed (logs scanned/patched/skipped counters)
- `1`: listing failed for one of the resource types

## `remove-finalizers`

Uninstall step: remove the cleanup finalizer from every User, Group and AccessRole CR that carries it, so deleting the CRDs or the namespace after `helm uninstall` does not hang. Best effort: a failed list or removal is logged and the run goes on, so it never blocks the uninstall.

Run by the chart's `pre-delete` hook (`charts/telark/templates/auth/uninstall-finalizers.yaml`) on `helm uninstall`, not by hand: a Job on the auth image, with the service token, once auth is scaled to zero and its pods are gone. A running auth leader would put the finalizers back on its next sweep (every `CLEANUP_SWEEPER_INTERVAL_SECONDS`, 60 by default), which is also how a reinstall that kept the data gets them back. After `helm uninstall --no-hooks` the exporter it calls is gone, so clear the finalizers with `kubectl` instead ([Full teardown](../../../docs/INSTALL.md#full-teardown)).

**Usage** (what the hook's Job runs):
```bash
./main remove-finalizers
```

No flags.

**Exit codes:**
- `0`: always; the log line `removed=<n> failed=<n>` gives the outcome

## Adding a subcommand

1. Create `cmd/<name>.go` (package `cmd`) with a `RunFoo(args []string) int` function.
2. Register it in `cmd/dispatch.go`:
   ```go
   const SubcommandFoo = "foo"
   var registry = map[string]runner{
     ...
     SubcommandFoo: RunFoo,
   }
   ```
3. No changes to `main.go` needed.
