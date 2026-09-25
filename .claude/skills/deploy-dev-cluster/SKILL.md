---
name: deploy-dev-cluster
description: Use when the user asks to deploy, redeploy or roll out local changes to the telark dev cluster, or has started a build-deploy-verify loop. Builds and pushes service images to the tags the chart already pins, upgrades the chart from the local checkout, and restores port-forwards. Not a default step after a fix.
---

# Deploy to the dev cluster

Goal: the dev cluster runs exactly the code in your checkout, every pod is ready, the port-forwards work, and you have exercised the scenario that motivated the change against it.

Deploy only when the user asked for it or started a redeploy loop. Inside that loop, run the build and port-forward scripts yourself, end to end, instead of handing steps back. A deploy never bumps a version: the scripts push to the tag the chart already pins, and the chart pulls with `pullPolicy: Always`, so a restart picks up the new image.

## Before building

- **Branch and tag.** Confirm the branch (`main` unless the user named another) and that the tag you'll push is the one the cluster runs:

  ```sh
  git branch --show-current
  yq e '.services.<svc>.version' charts/telark/values.yaml
  kubectl -n telark get deploy telark-<name> -o jsonpath='{.spec.template.spec.containers[0].image}'
  ```

  `<name>` is `services.<svc>.name` (for example `discovery-service`). If the tags differ, the branch lags main's version bumps and the deploy would silently change nothing. Ask; don't merge or rebase.
- **Cluster.** `kubectl config current-context` and `kubectl get nodes`. If the cluster is unreachable, ask the user whether it's up. Capacity and node changes go through the infra repository, never through `aws` or `eksctl` against the cluster.

## Go services

```sh
scripts/local-build-push.sh discovery exporter   # names or numbers; -s detects changed services, then asks to confirm
```

- Builds each service from its own Dockerfile, pushes `<app.image.registry>/<repository>:<version>` and restarts the deployment. `-l` only builds, `-n` skips the restart. Per-service logs go to a temporary directory the script prints.
- When a local checkout of a shared module (`../internal/<module>`, or `INTERNAL_DIR`) differs from the version a service pins, the script swaps it in inside a temporary build context, so unreleased shared-module changes ship without touching `go.mod`.
- Needs a running Docker daemon logged in to the registry, plus `yq`, `kubectl`, `go`, `rsync` and `git`.

## Analyzer (services/analyzer)

The build script doesn't cover it. Build for the nodes' architecture and push to the pinned tag:

```sh
REG=$(yq e '.app.image.registry' charts/telark/values.yaml)
REPO=$(yq e '.services.analyzer.repository' charts/telark/values.yaml)
TAG=$(yq e '.services.analyzer.version' charts/telark/values.yaml)
ARCH=$(kubectl get nodes -o jsonpath='{.items[0].status.nodeInfo.architecture}')
docker buildx build --platform "linux/$ARCH" -t "$REG/$REPO:$TAG" --push services/analyzer
kubectl -n telark rollout restart deploy/telark-analyzer-service
kubectl -n telark rollout status deploy/telark-analyzer-service --timeout=180s
```

## Chart from the local checkout

Needed when templates, values or CRDs changed. Use the existing release name (`helm list -n telark`):

```sh
helm dependency build charts/telark
helm upgrade --install telark charts/telark -n telark --reset-then-reuse-values
```

- Rebuild the dependencies first: `telark-crds` is vendored from the local directory, so CRD edits reach the cluster only after `helm dependency build`. The CRDs ship inside this release (`crds.enabled`); don't also install `charts/telark-crds` as a separate release, because Helm won't let a second release take over CRDs the first one owns.
- `--reset-then-reuse-values` keeps the release's own overrides and takes the new chart defaults. Plain `--reuse-values` keeps the old chart's defaults, so a changed default never reaches the cluster.
- A fresh install (add `--create-namespace`) needs `app.persistence.storageClass` set to a ReadWriteMany class while the exporter runs two replicas (the chart refuses to render otherwise), or `app.singleNode=true`. For an existing release, `helm get values telark -n telark` shows what is already set.
- Ollama is on by default, sized once for every mode. If its pod stays Pending, check `kubectl describe pod` for `Insufficient cpu` or `Insufficient memory` before treating it as a defect; sizing options are in the chart README's "Analyzer runtime (ollama)" section.

## Port-forwards

A rollout kills the pods behind every port-forward. After each deploy, from the repo root:

```sh
scripts/local-port-forward.sh     # run in the background; restarts existing forwards
pgrep -fl port-forward            # confirm before calling localhost
```

The local port for each service is in `charts/telark/values.dev.yaml`.

## Verify

- `kubectl -n telark get pods`: everything Ready, no new restarts, the restarted deployments on new pods.
- Exercise the exact scenario that motivated the change through the port-forwards (APIs under `/api/v1/`). Use the real opaque ids the UI sends (visible in its network requests), not display names.
- Judge API latency from inside the cluster: `kubectl` from a laptop adds seconds of exec-auth to every call.
- Redis runs in-cluster without auth: `kubectl -n telark exec <release>-redis-master-0 -- redis-cli --no-raw <command>` (find the pod with `kubectl -n telark get pods`).
