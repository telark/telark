GO_SERVICES := auth discovery exporter notifier
GO_PACKAGES := data rest kcore x-ware
GO_DIRS      := $(addprefix services/,$(GO_SERVICES)) $(addprefix internal/,$(GO_PACKAGES))
CHART_DIR    := charts/telark
CRDS_DIR     := charts/telark-crds
REGISTRY     ?= oci://ghcr.io/telark/charts
HELM_DOCS    := go run github.com/norwoodj/helm-docs/cmd/helm-docs@v1.14.2
GOLANGCI_CONFIG := $(CURDIR)/.golangci.yml

# The app chart refuses to render without an admin (templates/_guards.tpl); lint and
# validation use a placeholder one.
RENDER_SET   := --set app.auth.bootstrap.admin=test@example.com

.PHONY: help build test lint fmt vet helm-lint helm-template helm-validate deps values-docs changelog publish-charts check

help: ## List targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-14s %s\n", $$1, $$2}'

build: ## Build the Go module
	go build ./...

vet: ## go vet the Go module
	go vet ./...

fmt: ## Format the Go code
	gofmt -w services internal

test: ## Run the Go tests
	go test ./...

lint: helm-lint ## Run golangci-lint per service and package, or for one with SERVICE=<name>
	@if [ -n "$(SERVICE)" ]; then \
		found=0; \
		for d in $(GO_DIRS); do \
			if [ "$${d##*/}" = "$(SERVICE)" ]; then \
				found=1; \
				echo "== lint $$d =="; \
				golangci-lint run --config "$(GOLANGCI_CONFIG)" ./$$d/... || exit 1; \
				break; \
			fi; \
		done; \
		if [ "$$found" -eq 0 ]; then \
			echo "ERROR: unknown SERVICE='$(SERVICE)'"; \
			echo "Valid names: $(GO_SERVICES) $(GO_PACKAGES)"; \
			exit 2; \
		fi; \
	else \
		for d in $(GO_DIRS); do \
			echo "== lint $$d =="; \
			golangci-lint run --config "$(GOLANGCI_CONFIG)" ./$$d/... || exit 1; \
		done; \
	fi

helm-lint: ## Lint both charts
	helm lint $(CRDS_DIR) -f $(CHART_DIR)/values.yaml
	helm lint $(CHART_DIR) $(RENDER_SET)

helm-template: ## Render the app chart (override sizing with MODE=minimal|performance)
	helm template t $(CHART_DIR) $(RENDER_SET) $(if $(MODE),--set app.mode=$(MODE),)

deps: ## Fetch subchart dependencies into charts/*/charts (git-ignored; rebuilt on demand)
	helm repo add bitnami        https://charts.bitnami.com/bitnami
	helm repo add kyverno        https://kyverno.github.io/kyverno/
	helm repo add otwld          https://helm.otwld.com/
	helm repo add metrics-server https://kubernetes-sigs.github.io/metrics-server/
	helm dependency build $(CHART_DIR)

# Kubernetes versions to validate against: the chart's kubeVersion floor through
# the newest supported release. Keep in step with Chart.yaml and the
# helm-kubeconform-validate action, which fails if the floor goes untested.
K8S_VERSIONS ?= 1.30.0 1.31.0 1.32.0 1.33.0 1.34.0

helm-validate: deps ## Schema-validate the rendered manifests for every mode and supported k8s version (kubeconform)
	@mkdir -p /tmp/kubeconform-cache
	@for m in minimal standard performance; do \
	  helm template t $(CHART_DIR) $(RENDER_SET) --set app.mode=$$m --set app.persistence.storageClass=validate \
	    > /tmp/rendered-$$m.yaml || exit 1; \
	  for v in $(K8S_VERSIONS); do \
	    echo "== kubeconform $$m · k8s $$v =="; \
	    kubeconform -strict -summary -ignore-missing-schemas \
	        -kubernetes-version $$v \
	        -cache /tmp/kubeconform-cache \
	        -schema-location default \
	        -schema-location 'https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json' \
	        < /tmp/rendered-$$m.yaml \
	      || exit 1; \
	  done; \
	done

values-docs: ## Regenerate each chart's VALUES.md index from values.yaml (helm-docs)
	$(HELM_DOCS) --chart-search-root charts --output-file VALUES.md

# cliff.toml reads PR links + contributors from the GitHub API, which needs GITHUB_TOKEN;
# without one the preview runs offline and leaves them out.
changelog: ## Preview the next release's CHANGELOG.md section (git-cliff); the release workflow prepends it
	git cliff $(if $(GITHUB_TOKEN),,--offline) --unreleased

# Signed like release-charts.yaml, so a manual publish never leaves an unsigned chart in the repo.
publish-charts: deps ## Package, push and cosign-sign both charts (run `helm registry login ghcr.io` and `cosign login ghcr.io` first)
	rm -rf .cr-release && mkdir -p .cr-release
	helm package $(CRDS_DIR)  --destination .cr-release
	helm package $(CHART_DIR) --destination .cr-release
	@for pkg in .cr-release/*.tgz; do echo "== push $$pkg =="; \
	  helm push "$$pkg" $(REGISTRY) > .cr-release/push.log 2>&1 || { cat .cr-release/push.log; exit 1; }; \
	  cat .cr-release/push.log; \
	  digest=$$(awk '/[Dd]igest:/{print $$NF}' .cr-release/push.log); \
	  name=$$(helm show chart "$$pkg" | awk '/^name:/{print $$2}'); \
	  [ -n "$$digest" ] || { echo "no digest for $$pkg"; exit 1; }; \
	  cosign sign --yes "$(patsubst oci://%,%,$(REGISTRY))/$$name@$$digest" || exit 1; \
	done

check: lint test ## Lint and test everything