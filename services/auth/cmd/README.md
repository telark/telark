# auth-service subcommands

Run as `auth <subcommand> [flags]`. The binary entry point in `main.go` delegates to `cmd.Dispatch`. If a subcommand is not recognized, the auth HTTP server starts normally.

---

## `break-glass`

Promote a user to the built-in Admin role, or with `--enroll` bootstrap an account that does not exist yet. Used to recover access when no admin exists, to enrol the first administrator on a passkey-only install, or when the bootstrap-admin entry didn't take effect. This is the only path that grants Admin from an email nobody has verified, which is why it is a subcommand run by the operator and not an API.

**Usage:**
```bash
auth break-glass --email <email>
auth break-glass --email <email> --enroll
```

**Flags:**
| Flag | Required | What it does |
|------|----------|--------------|
| `--email` | yes | Email of the user to promote |
| `--enroll` | no | Create the user when missing (Admin role, `bootstrap: true` when the email is in `BOOTSTRAP_ADMINS`) and print a one-time passkey enrolment token (10 minutes, needs Redis). Open `/register?enroll=<token>` in the dashboard to register the passkey. |

**Exit codes:**
- `0` — user promoted (or created), or already had Admin
- `1` — user not found (without `--enroll`), create or patch failed, Redis unavailable, or missing/invalid flag

---

## `backfill-finalizers`

Manual operator tool. Add the cleanup finalizer to every existing User / Group / Role CR that doesn't already have it. Idempotent — safe to re-run.

Normally not needed: the auth-service controller's per-tick "ensure-finalizer" pass already adds the finalizer to any CR observed without it. Use this subcommand when you want a single explicit one-shot pass (e.g., during a migration window) instead of waiting for the controller to converge.

**Usage** (run inside an auth-service pod, or via a one-off Job/`kubectl run` against the auth image):
```bash
auth backfill-finalizers
```

No flags. Behavior controlled by env vars:

| Env | Default | What it does |
|-----|---------|--------------|
| `BACKFILL_BATCH_SIZE` | `10` | Number of patches per batch before pausing |
| `BACKFILL_BATCH_PAUSE_MS` | `100` | Pause between batches, in milliseconds (rate-limits the exporter) |

**Exit codes:**
- `0` — backfill completed (logs scanned/patched/skipped counters)
- `1` — listing failed for one of the resource types

---

## Adding a new subcommand

1. Create `cmd/<name>/run.go` with a `Run(args []string) int` exported function.
2. Register it in `cmd/dispatch.go`:
   ```go
   const SubcommandFoo = "foo"
   var registry = map[string]runner{
     ...
     SubcommandFoo: foo.Run,
   }
   ```
3. No changes to `main.go` needed.
