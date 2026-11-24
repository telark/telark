# The telark CLI

`telark` works with a Telark installation through the Kubernetes API server, as the user in your kubeconfig, so the cluster's RBAC and audit log cover every action. It needs no Telark login and opens no new way into the cluster.

## Installing the CLI

Every Telark release from 0.3.0 on ships the CLI for Linux, macOS and Windows on amd64 and arm64. Use the CLI of the release you run: `telark install` and `telark upgrade` install the chart of their own version.

- **Homebrew** (macOS and Linux): `brew install telark/tap/telark`, and `brew upgrade telark` later. The tap follows releases without a version suffix: for a pre-release, use the install script with `TELARK_VERSION`.
- **Install script** (Linux and macOS): `curl -sSfL https://telark.io/install.sh | sh` installs the latest release to `/usr/local/bin`, with `sudo` when that needs it. `TELARK_VERSION=0.3.0` picks a release and `TELARK_INSTALL_DIR` another directory. It checks the archive against `checksums.txt` and, when [cosign](https://docs.sigstore.dev/cosign/system_config/installation/) is installed, `checksums.txt` against its signature.
- **The release archives**: `telark_<version>_<os>_<arch>.tar.gz`, and `.zip` for Windows, on the [GitHub Release](https://github.com/telark/telark/releases). Put `telark` (`telark.exe`) on your `PATH`. The binaries aren't notarized or Authenticode-signed yet: macOS blocks a file a browser downloaded until you run `xattr -d com.apple.quarantine telark`, and Windows SmartScreen asks before the first run. Homebrew and the install script need neither.
- **From a checkout**: `make cli` writes `bin/telark` ([Development](DEVELOPMENT.md)).

### Verifying downloads

Each release has a `checksums.txt` of its archives, signed keyless with cosign from the release workflow's run on the tag, the identity the chart's signature has. Check the signature, then the archive:

```sh
VERSION=0.3.0
cosign verify-blob checksums.txt --bundle checksums.txt.sigstore.json \
  --certificate-identity "https://github.com/telark/telark/.github/workflows/release-charts.yaml@refs/tags/v$VERSION" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
sha256sum --ignore-missing -c checksums.txt   # shasum -a 256 --ignore-missing -c on macOS
```

Each archive also has a build provenance attestation, `gh attestation verify telark_${VERSION}_linux_amd64.tar.gz -R telark/telark`, and an SPDX SBOM beside it (`<archive>.sbom.json`). cosign 2.6 or later reads the bundle.

## Common flags and output

- `--kubeconfig` and `--context` select the cluster, in kubectl's order: the flag, then `$KUBECONFIG`, then `~/.kube/config`.
- `-n, --namespace` is the namespace Telark runs in, `telark` by default.
- `-o table|json|yaml` on `version`, `check` and `auth status`.
- Exit code 0 on success, 1 on a failed check or any error, which prints one `Error:` line on stderr.
- Color only on a terminal, never with `NO_COLOR` set.

## telark version

Prints the CLI's version, commit, build date and Go version, then the server's release (the chart's `app.kubernetes.io/version` label on the Telark Deployments) and each service's image tag. `--client` skips the server. A warning on stderr says when the CLI and the server differ in major or minor version.

## telark check

Prints one line per check: ✔ passed, ‼ a warning, × failed. Any × makes the command exit 1. A ‼ still exits 0.

On a running Telark:

| Check | Passes when |
|---|---|
| CRDs | Telark's ten CRDs exist and are established. ‼ when they come from several telark-crds versions |
| Workloads | Every Deployment and StatefulSet of the release, subcharts included, is available. The release is the `app.kubernetes.io/instance` label of the Telark services |
| Pod restarts | No pod of the release restarted in the last hour (‼) |
| Versions | The CLI and the server are from the same major and minor release (‼) |
| Kyverno | Kyverno's webhooks are registered and its admission controller (`app.kubernetes.io/name=kyverno-admission-controller`, in any namespace) is available |
| CRD guard | `telark-crd-guard` and its binding exist and deny. ‼ when the guard only audits or is off |
| Bootstrap | `TelarkConfig default` exists and its bootstrap account is pinned. ‼ with the enroll command when it isn't |
| Recovery request | No recovery request is open. ‼ with the time it can complete: when it opened plus `app.auth.request.waitHours` |
| Protection plans | No plan is `drifted` or `degraded` (‼ with their names) |
| Update | No newer Telark release exists (‼, see below) |

It reads Deployments, StatefulSets, pods, Telark's custom resources, CRDs, webhook configurations and admission policies, so it needs read access to them in Telark's namespace and across the cluster.

### Before an install: telark check --pre

| Check | Passes when |
|---|---|
| Kubernetes | The cluster runs 1.30 or newer |
| Permissions | You may create every kind of object the chart installs, asked with one SelfSubjectAccessReview per kind, and the namespace when it doesn't exist yet |
| StorageClass | A default class exists for the volume claims. Under `telark install`, the class `app.persistence.storageClass` names exists, and `-` passes |
| Kyverno | No other Kyverno runs (its webhooks aren't registered). To use the one you run, install with `kyverno.enabled=false`, which `telark install` then requires |
| Earlier install | Telark doesn't run in any namespace. ‼ when an earlier install left Telark's CRDs or volume claims behind, since a reinstall keeps that data |
| Node capacity | The Ready nodes a pod without tolerations can use have room for the mode's requests (‼). With the chart's defaults, Ollama included: `minimal` 1.35 CPU and 2.9 GiB, `standard` 1.65 CPU and 3.3 GiB, `performance` 2.55 CPU and 5.6 GiB |

`--mode minimal|standard|performance` picks the mode to size for, `standard` by default. The checks assume the chart's default values, and `telark install` runs them with its own.

### Flags

- `--wait <duration>` repeats the checks every two seconds until they all pass or the time runs out, and prints the last run.
- `--skip-update-check`, or `TELARK_SKIP_UPDATE_CHECK=1`, turns the update check off.

### The update check

`telark check` sends one GET to `https://api.github.com/repos/telark/telark/releases/latest` from your machine, never from the cluster, with a two-second timeout and a `telark/<version>` User-Agent. When GitHub can't be reached, as on an air-gapped site, the line says the check was skipped and the command still passes.

## telark install

```sh
telark install --bootstrap-admin <email> [--mode minimal|standard|performance] [-f <values>]... [--set <key>=<value>]... \
  [--version <version>] [--chart-directory <dir>] [--wait] [--dry-run]
```

Installs Telark as the Helm release `telark` in the `-n` namespace, which it creates when missing. It drives the Helm 4 SDK, so `helm list`, `helm get values` and `helm upgrade` keep working on the release.

1. It merges `-f` and `--set` the way Helm does, then sets `app.auth.bootstrap.admin` from `--bootstrap-admin` and `app.mode` from `--mode` on top. The chart requires the bootstrap admin, so the command stops without one.
2. It runs the checks of `telark check --pre` with the merged values: the mode, `app.persistence.storageClass`, and `kyverno.enabled=false`, which then requires the Kyverno you run. Any × stops it.
3. It pulls `oci://ghcr.io/telark/charts/telark` at `--version`, the CLI's own version by default, and checks its [signature](#the-chart-signature). `--chart-directory` installs a chart checkout instead, for development or an air-gapped site: its signature isn't checked, and its subcharts must be built first (`make deps`, or `helm dependency build`).
4. It installs with server-side apply and a five-minute timeout. `--wait` also waits until every object is ready.

`--dry-run` changes nothing. The chart renders against the cluster, its lookups and guards included, and the command prints the merged values and how many objects of each kind the install would create. The checks still run first, so a dry run also stops on a cluster where Telark runs.

After the install it prints the next steps: `telark check`, then `telark auth enroll --email <the bootstrap admin>`.

You need the rights to create everything the chart installs, CRDs and cluster roles included. The permission check of `--pre` asks for each kind before anything is created.

## telark upgrade

```sh
telark upgrade [--version <version>] [-f <values>]... [--set <key>=<value>]... [--reset-values] \
  [--chart-directory <dir>] [--wait] [--dry-run]
```

Upgrades the release `telark` in the `-n` namespace to `--version`, the CLI's own version by default. It keeps the release's values and takes the new chart's defaults, like `helm upgrade --reset-then-reuse-values`, then applies `-f` and `--set` on top. `--reset-values` starts from the chart's defaults instead. Unlike a plain `helm upgrade`, you don't repeat the flags you installed with.

Before anything changes, it runs these checks and stops on any ×:

| Check | Passes when |
|---|---|
| Signature | The chart's [signature](#the-chart-signature) verifies. ‼ without cosign, and for `--chart-directory` |
| Step | The target is the installed release or the next published one. × for a downgrade (go back with `helm rollback`) and when a published release would be skipped, since a rollback works one release back and the exporter refuses data two releases newer. ‼ when the registry can't list its releases |
| Server dry run | The chart renders against the cluster, its lookups and guards included, its objects fit the cluster's API schemas, and the server accepts each object as a dry-run server-side apply under Helm's field manager. × names the refused objects: a field another field manager owns (after a `kubectl set`, `kubectl edit` or `kubectl label`), a field Kubernetes can't change, or an admission policy |
| Stored objects | Every Telark custom resource in the namespace fits the target's CRDs once their defaults apply: the schema, its CEL rules and its list types. ‼ when the new schemas would drop fields from stored objects, and when the chart installs no CRDs (`crds.enabled=false`) |
| Recovery request | No recovery request is open (‼) |
| Protection plans | No plan enforces now (‼). Kyverno restarts during the upgrade, and its failure policy decides what happens to changes meanwhile |

`--dry-run` stops after the checks. Otherwise the command upgrades with a five-minute timeout, then runs `telark check` until every check passes or five minutes go by. `--wait` also waits for every object to be ready before that.

It applies the way the release was installed, as `helm upgrade` does on Helm 4: server-side when Helm 4 installed it, client-side for a release Helm 3 installed. Client-side apply ignores field ownership, so for such a release the dry run skips the ownership test and still catches the other refusals. `telark upgrade` applies as Helm's own field manager, `helm`, so `helm upgrade` and `telark upgrade` can take turns on one release.

When the dry run refuses an object because another field manager owns a field, make your values match what the cluster runs, for example `--set services.<service>.version=<the running tag>` after a `kubectl set image`. Don't reach for `helm upgrade --force-conflicts`: it takes every such field from its owner, which can put an older CRD schema back and let the API server prune newer fields.

The upgrade reads Helm's release Secrets in the namespace and needs the same rights as `helm upgrade`.

### The chart signature

The release workflow signs the telark chart keyless with cosign, from the workflow run on the chart's own tag. `telark install` and `telark upgrade` check the chart by the digest they pulled, so what they install is what was signed: they run `cosign verify ghcr.io/telark/charts/telark@<digest>` with that run's identity, `https://github.com/telark/telark/.github/workflows/release-charts.yaml@refs/tags/v<version>`, and GitHub's OIDC issuer. Install [cosign](https://docs.sigstore.dev/cosign/system_config/installation/) 2.6 or later to have this check: without it the line is a ‼ and the command goes on. The signature is a Sigstore bundle stored beside the chart, which cosign 2.5 and older can't find, so they report "no signatures found". A signature that doesn't verify is a ×. Charts published before 0.3.0 were signed from a branch, so they fail this check: install those with `helm`, and check them as [Publishing](PUBLISHING.md#5-sign-with-cosign) shows.

## telark auth enroll

```sh
telark auth enroll --email <the bootstrap admin>
```

Enrolls the bootstrap admin, or recovers it once it is pinned, through an [`AuthRequest` object](INSTALL.md#authrequest-objects), and replaces the in-pod command of releases before 0.3.0. It needs no Telark login, only the `telark-auth` role in the release namespace (`app.auth.request.subjects`, or a cluster admin).

1. It reads your Kubernetes username from the API server (a SelfSubjectReview, as `kubectl auth whoami` does), since the admission policy refuses any other name in `spec.requestedBy`.
2. It makes a one-time X25519 key pair, kept in memory only, and creates the object with `generateName: auth-request-`, the email, your username and the public key.
3. It watches the object and prints each phase auth reports, with its reason:
   - `WaitingForApproval`: a [recovery request](INSTALL.md#recovery-requests) is open and its wait runs. The command prints `approvalUrl`, the request's page, and `completesAt`, then exits 0. The object rests there: run the command again after that time, or after a replacement's approval, and it continues the same request.
   - `LinkReady`: the command opens `status.sealedLink` with its private key and prints the enrollment link and `linkExpiresAt`. Open the link in a browser that isn't signed in to Telark and register a passkey: the command waits for it. Only this run of the command can open the link, so keep it running until the link is printed.
   - `Completed`: the passkey is registered (✔, exit 0).
   - `Denied`, `Canceled`, `Expired` or `Failed`, or the object is deleted: the reason goes to stderr and the command exits 1. Run it again for a new request or link.

When `app.auth.passkey.origin` is empty, the links start with `<dashboard-url>`: replace it with the address you open the dashboard at, `http://localhost:3000` with the port-forward. A Telark older than 0.3.0 serves no `AuthRequest` objects, so the command stops and prints the `kubectl exec` command that release uses instead.

## telark auth status

```sh
telark auth status [-o table|json|yaml]
```

Prints whether the bootstrap account is pinned and since when, and the newest request in `TelarkConfig` `status.bootstrap`: its id, what it does, its state and reason, when it opened and the Kubernetes user who opened it. It reads `TelarkConfig default` and changes nothing.
