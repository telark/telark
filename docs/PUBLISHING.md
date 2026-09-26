# Publishing the Helm charts

telark ships two charts — `telark` (app) and `telark-crds` — as **OCI artifacts** in
GitHub Container Registry: `oci://ghcr.io/telark/charts`. GHCR packages are **private by
default**, so the steps below test the full private publish/pull/deploy path before you
ever make anything public.

- **Automated:** tag `vX.Y.Z` (or run *Release · Publish Charts*) → CI packages, pushes, and **cosign-signs** both charts. See [`.github/workflows/release-charts.yaml`](../.github/workflows/release-charts.yaml).
- **Manual:** the commands here, for testing a publish from your machine.

## Prerequisites

- `helm` ≥ 3.8 (OCI is GA), `cosign`, and — for ArtifactHub — `oras`.
- A GitHub PAT (classic) with **`write:packages`** and **`read:packages`**, exported as `CR_PAT`.

## 1. Log in to the registry

```sh
echo "$CR_PAT" | helm registry login ghcr.io -u <your-gh-user> --password-stdin
echo "$CR_PAT" | cosign login ghcr.io -u <your-gh-user> --password-stdin
```

`make publish-charts` signs each chart right after pushing it, like the release workflow, so cosign needs its own login.

## 2. Package + push

```sh
make publish-charts            # build deps, package both charts, push to $REGISTRY, cosign-sign each
```

<details><summary>What that runs</summary>

```sh
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo add kyverno https://kyverno.github.io/kyverno/
helm repo add otwld https://helm.otwld.com/
helm repo add metrics-server https://kubernetes-sigs.github.io/metrics-server/
helm dependency build charts/telark
mkdir -p .cr-release
helm package charts/telark-crds --destination .cr-release
helm package charts/telark      --destination .cr-release
for pkg in .cr-release/*.tgz; do
  helm push "$pkg" oci://ghcr.io/telark/charts          # prints Digest: sha256:…
  cosign sign --yes ghcr.io/telark/charts/<chart>@<digest>   # keyless, opens a browser login
done
```
</details>

The signature is keyless (your OIDC identity, not the CI workflow's); to sign with a key instead, see step 5.

## 3. Verify the publish

```sh
helm show chart oci://ghcr.io/telark/charts/telark --version <version>
helm pull       oci://ghcr.io/telark/charts/telark --version <version>
helm template t oci://ghcr.io/telark/charts/telark --version <version> \
  --set 'app.auth.bootstrap.admins={jane.doe@example.com}'
```

## 4. Deploy from the registry

Mirrors the deploy workflow. The app release **must** be named `telark-release` so the
subchart DNS (`{{ .Release.Name }}-redis-master`, `-nats`, `-ollama`) resolves.

```sh
NS=telark

# App: services, subcharts, and CRDs — all ship in the chart (telark-crds is a
# subchart). NATS config inlined; size with --set app.mode=<mode>. standard runs
# two exporter replicas that share both exporter volumes, so name a ReadWriteMany
# class (or --set app.singleNode=true on a one-node test cluster).
helm upgrade --install telark-release oci://ghcr.io/telark/charts/telark --version <version> \
  -n "$NS" --create-namespace \
  --set app.persistence.storageClass=<rwx-class> \
  --set 'app.auth.bootstrap.admins={jane.doe@example.com}' \
  --wait --timeout 15m

helm test telark-release -n "$NS"    # readiness probe against auth
```

## 5. Sign with cosign

CI signs **keyless** via GitHub OIDC automatically. To sign a manual push locally:

```sh
# key-based (fully offline)
cosign generate-key-pair
DIGEST=sha256:...                       # from the `helm push` output in step 2
cosign sign   --key cosign.key ghcr.io/telark/charts/telark@$DIGEST
cosign verify --key cosign.pub ghcr.io/telark/charts/telark@$DIGEST
```

Verify a CI (keyless) signature:

```sh
cosign verify ghcr.io/telark/charts/telark@$DIGEST \
  --certificate-identity-regexp '^https://github.com/telark/telark/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## 6. Go public (when ready)

In each package's settings on GHCR (`ghcr.io/telark/charts/telark`, `.../telark-crds`),
change visibility to **Public**. Only then can ArtifactHub index them.

## 7. Register on ArtifactHub

1. Add an OCI repository pointing at `oci://ghcr.io/telark/charts/telark` (and `.../telark-crds`).
2. Copy the issued `repositoryID` into [`charts/artifacthub-repo.yml`](../charts/artifacthub-repo.yml).
3. Push that metadata so ArtifactHub verifies ownership:

```sh
oras push ghcr.io/telark/charts/artifacthub-repo.yml:latest \
  charts/artifacthub-repo.yml:application/vnd.cncf.artifacthub.repository-metadata.layer.v1.yaml
```

ArtifactHub auto-detects the cosign signatures and shows the charts as **Signed**.

## Values docs

Each chart carries an auto-generated `VALUES.md` (exhaustive key/type/default index).
Regenerate after any `values.yaml` change — CI fails if it drifts:

```sh
make values-docs
```

## Repository settings the workflows rely on

The build and release workflows push version-bump commits to the branch they run from and hold registry credentials, the `ACCESS_TOKEN` PAT and `id-token: write` (cosign). The repository defines no GitHub environments, so nothing in the workflows asks for an approval; these settings are what keep them safe:

- **Protect `main`**: require the CI checks and a CODEOWNERS review on pull requests, block force pushes and deletion. The bump commits are pushed with `GITHUB_TOKEN`, so either allow `github-actions[bot]` to bypass the pull-request rule or move the bumps to a bot branch merged by pull request.
- **Restrict who can run workflows**: `workflow_dispatch` runs with the repository's secrets from any branch a writer names, so keep write access to maintainers. To require an approval per run, create an environment (Settings → Environments, for example `release`) with required reviewers and add `environment: release` to the build and release jobs.
- **Tags are immutable**: protect `v*` tags (Settings → Rules → tag ruleset) here and in `telark/dashboard-ui`. A UI build refuses to re-tag an existing `vX.Y.Z` (`.github/scripts/tag-service-repo.sh`), so a rebuild of a released UI version needs a version bump (`patch`) rather than `re-build-current`.
- **Scope the `ACCESS_TOKEN` PAT** to the repositories it pushes to (`telark/dashboard-ui` and the shared modules) with contents write only; the CI pull-request jobs no longer receive it.
