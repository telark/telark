# Publishing the Helm charts

Telark ships two charts, `telark` (the app) and `telark-crds`, as public OCI artifacts in
GitHub Container Registry: `oci://ghcr.io/telark/charts`.

- **Before each release:** run the [upgrade test](#upgrade-test).
- **Automated:** run *Release · Publish Charts* with `chart` (`both`, `telark`, `telark-crds`) and `bump` (`patch`, `minor`, `major`). Don't edit the versions in `Chart.yaml` first: the workflow bumps each selected chart from its own current version, writes it to `Chart.yaml`, packages, pushes and **cosign-signs** it (telark-crds first), then commits the change. A version already in the registry fails the run before anything is pushed. See [`.github/workflows/release-charts.yaml`](../.github/workflows/release-charts.yaml).
  - `appVersion` is the Telark release: telark's follows its new `version`, and telark-crds takes the same value when released with telark (`chart=both`). A CRD-only release keeps telark-crds' `appVersion`.
  - On `chart=both`, telark's `telark-crds` dependency and `Chart.lock` move to the new CRD version in the same commit, so the app chart bundles those CRDs. A telark-only release keeps the pin.
  - Only telark is tagged (`v<version>`) and gets a GitHub Release. `release_type` only picks the release kind (`auto`: a pre-release when the version has a suffix such as `-rc.1`) and never changes the version. The release notes and `CHANGELOG.md` both come from [`cliff.toml`](../cliff.toml): the run prepends the new version's section to `CHANGELOG.md` and never rewrites older ones. Each entry links its commit and the pull request GitHub associates with it (the one that merged it into `main`, or the open one of the branch the run releases), so open the release pull request before running the workflow. When the UI version changed since the previous release, the section ends with a Dashboard UI part copied from the `telark/dashboard-ui` `CHANGELOG.md`, which `build-ui.yaml` prepends with each UI tag (format in [`.github/cliff-ui.toml`](../.github/cliff-ui.toml)) and publishes as that tag's GitHub Release.
  - Pushing a `vX.Y.Z` tag yourself publishes both charts at their current `Chart.yaml` versions, without a bump.
- **Manual:** the commands here, for testing a publish from your machine.

## Prerequisites

- `helm` ≥ 3.8 (OCI is GA), `cosign`, and `oras` for ArtifactHub.
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
  --set app.auth.bootstrap.admin=test@example.com
```

## 4. Deploy from the registry

The chart ships the services, subcharts and CRDs (telark-crds is a subchart). Size it with
`--set app.mode=<mode>`; the exporter's volumes come from the default StorageClass
([Exporter storage](INSTALL.md#exporter-storage)).

```sh
helm upgrade --install telark oci://ghcr.io/telark/charts/telark --version <version> \
  -n telark --create-namespace \
  --set app.auth.bootstrap.admin=test@example.com \
  --wait --timeout 15m
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

## 6. Register on ArtifactHub

Each OCI chart is its own Artifact Hub repository, with its own repository ID.

1. Sign in to Artifact Hub, open **Control Panel → Repositories → Add**, pick **Helm charts** and use the URL `oci://ghcr.io/telark/charts/telark`.
2. Copy the new repository's ID (on its card in the control panel) into `repositoryID` in [`charts/artifacthub-repo.yml`](../charts/artifacthub-repo.yml).
3. Push the file to the chart's `artifacthub.io` tag, where Artifact Hub looks for it (ownership claim and the verified-publisher badge):

```sh
oras push ghcr.io/telark/charts/telark:artifacthub.io \
  --config /dev/null:application/vnd.cncf.artifacthub.config.v1+yaml \
  charts/artifacthub-repo.yml:application/vnd.cncf.artifacthub.repository-metadata.layer.v1.yaml
```

`telark` already bundles the CRDs, so listing `telark-crds` is optional: register `oci://ghcr.io/telark/charts/telark-crds` the same way and push a copy of the file with that repository's ID to `ghcr.io/telark/charts/telark-crds:artifacthub.io`.

ArtifactHub auto-detects the cosign signatures and shows the charts as **Signed**.

## Upgrade test

It proves that what the previous release stored still reads and saves after the upgrade, again after rolling back one release, and again after upgrading once more. Run it from the release branch, after the release's images are built and before the charts are published, on a test cluster where you are cluster-admin (kind works). The release's [upgrade notes](INSTALL.md#upgrade-notes) say which schema changes and migration steps to expect.

1. Install the previous release with sample data:

   ```sh
   helm install telark oci://ghcr.io/telark/charts/telark --version <previous version> -n telark --create-namespace \
     --set app.auth.bootstrap.admin=test@example.com --wait --timeout 15m
   kubectl create namespace upgrade-demo
   kubectl create deployment web -n upgrade-demo --image=nginx   # discovery turns it into an application
   ```

   Sign in as the bootstrap admin ([First admin](INSTALL.md#2-first-admin)) and create one of each in the dashboard: a user, a group, a role, a category, a protection plan on `upgrade-demo` that you start, and a settings change. Then list what is stored:

   ```sh
   kubectl get telark,telark-auth -n telark -o custom-columns=KIND:.kind,NAME:.metadata.name --no-headers | sort > before.txt
   ```

2. Upgrade to the branch. The app chart pulls `telark-crds` from the registry at its pinned version, so the second command replaces that copy with the branch's CRDs:

   ```sh
   helm dependency build charts/telark
   helm package charts/telark-crds --destination charts/telark/charts
   helm upgrade telark charts/telark -n telark --set app.auth.bootstrap.admin=test@example.com --wait --timeout 15m
   ```

3. Check that every object reads and saves:

   ```sh
   kubectl logs -n telark deploy/telark-exporter-service | grep 'migration step'
   kubectl get telark,telark-auth -n telark -o custom-columns=KIND:.kind,NAME:.metadata.name --no-headers | sort | diff before.txt -
   kubectl get telark,telark-auth -n telark -o json |
     kubectl replace --dry-run=server --as=system:serviceaccount:telark:telark-exporter-service-sa -f - > /dev/null &&
     echo every object saves
   ```

   - The log names each step of the upgrade notes, then the step the stored data is at.
   - The diff prints nothing, apart from sessions that your sign-ins added or that expired.
   - The dry run writes every stored object back as the exporter would, against the new schema and the CRD write guard. A `Conflict` only means a service changed that object in between: run it again.
   - In the dashboard, every page lists its objects, and editing and saving one of each kind works.

4. Roll back one release, run any rollback step the [upgrade notes](INSTALL.md#upgrade-notes) give, then repeat step 3:

   ```sh
   helm rollback telark -n telark
   kubectl rollout status deploy/telark-exporter-service -n telark --timeout=5m
   ```

   A previous release from before 0.2.0 logs no migration line. If the release ran a migration step and the previous one is 0.2.0 or later, the previous exporter logs `lowered the record to step <n>`, with `<n>` its own last step, and starts.

5. If the release ran a migration step, upgrade again with step 2's `helm upgrade` command and repeat step 3. The rollback lowered the record, so the log names the release's steps again.

## Corresponding source images

The analyzer and ui images contain GPL, LGPL and MPL Alpine packages. Every build of either image ends by pushing its source image to the image's own package, tagged `<tag>-source` (`ghcr.io/telark/<image>:<tag>-source`), with [`.github/scripts/build-source-image.sh`](../.github/scripts/build-source-image.sh): one `abuild srcpkg` bundle per such package (APKBUILD, patches and upstream archives, from the aports commit the package was built from) and an `index.txt`. The Go images are `FROM scratch` and get none. Images released before this step (0.1.0) have no `-source` tag: for a source request about one, run the script without `--push`.

- Keep a `-source` tag as long as its image tag is published: delete the two together, never the `-source` tag alone.
- If the step fails, the image and its version bump are already pushed. Rerun it by hand with the same tag, after `docker login ghcr.io`:

```sh
.github/scripts/build-source-image.sh ghcr.io/telark/analyzer:<tag> ghcr.io/telark/analyzer:<tag>-source --push
```

## Notices of the Alpine packages and Rust crates

The analyzer's and the dashboard's `THIRD-PARTY-NOTICES.md` reproduce the copyright notices of the permissive Alpine packages in their base image ("Alpine package notices") and, for the analyzer, of the Rust crates compiled into pydantic-core ("pydantic-core's Rust crates"). Alpine's `-doc` packages don't carry these files, so they come from upstream. Regenerate a section when its input changes:

- **Base image bump:** list the packages with `docker run --rm --entrypoint sh <image> -c 'apk list --installed'`. For each permissive package, take its license and copyright files from the source archive at the installed version: the `source=` of `main/<origin>/APKBUILD` in aports, mirrored at `https://distfiles.alpinelinux.org/distfiles/v<release>/`. Update the table, the versions and the texts.
- **pydantic-core bump:** the `required` components of `pydantic_core-<version>.dist-info/sboms/pydantic-core.cyclonedx.json` are the crates. Take each crate's license files from `https://static.crates.io/crates/<name>/<name>-<version>.crate`, using MIT where a crate offers it, and keep one copy of each distinct text.

## Repository settings the workflows rely on

The build and release workflows push version-bump commits to the branch they run from and hold registry credentials (`GITHUB_TOKEN` with `packages: write` pushes the images to `ghcr.io/telark`), the `ACCESS_TOKEN` PAT and `id-token: write` (cosign). The repository defines no GitHub environments, so nothing in the workflows asks for an approval; these settings are what keep them safe:

- **Protect `main`**: require the CI checks and a CODEOWNERS review on pull requests, block force pushes and deletion. The bump commits are pushed with `GITHUB_TOKEN`, so either allow `github-actions[bot]` to bypass the pull-request rule or move the bumps to a bot branch merged by pull request.
- **Restrict who can run workflows**: `workflow_dispatch` runs with the repository's secrets from any branch a writer names, so keep write access to maintainers. To require an approval per run, create an environment (Settings → Environments, for example `release`) with required reviewers and add `environment: release` to the build and release jobs.
- **Tags are immutable**: protect `v*` tags (Settings → Rules → tag ruleset) here and in `telark/dashboard-ui`. A UI build refuses to re-tag an existing `vX.Y.Z` (`.github/scripts/tag-service-repo.sh`), so every build bumps the version (`patch`, `minor` or `major`).
- **Scope the `ACCESS_TOKEN` PAT** to the repository it pushes to (`telark/dashboard-ui`) with contents write only; the CI pull-request jobs don't receive it.
