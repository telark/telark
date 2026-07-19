export GOPRIVATE := github.com/telark/*

GO_SERVICES := auth discovery exporter notifier
CHART_DIR    := charts/telark
CRDS_DIR     := charts/telark-crds
REGISTRY     ?= oci://ghcr.io/telark/charts
HELM_DOCS    := go run github.com/norwoodj/helm-docs/cmd/helm-docs@v1.14.2

.PHONY: help build test lint fmt vet helm-lint helm-template helm-validate deps values-docs changelog publish-charts sync check

help: ## List targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  %-14s %s\n", $$1, $$2}'

build: ## Build the Go workspace
	go build ./...

vet: ## go vet the workspace
	go vet ./...

fmt: ## Format the Go services
	gofmt -w services

test: ## Run Go tests per module
	@for s in $(GO_SERVICES); do echo "== test $$s =="; (cd services/$$s && go test ./...) || exit 1; done

lint: helm-lint ## golangci-lint per service (shared root .golangci.yml) + helm lint
	@for s in $(GO_SERVICES); do echo "== lint $$s =="; (cd services/$$s && golangci-lint run) || exit 1; done

helm-lint: ## Lint both charts
	helm lint $(CRDS_DIR) -f $(CHART_DIR)/values.yaml
	helm lint $(CHART_DIR)

helm-template: ## Render the app chart (override sizing with MODE=minimal|performance)
	helm template t $(CHART_DIR) $(if $(MODE),--set app.mode=$(MODE),)

deps: ## Fetch subchart dependencies into charts/*/charts (git-ignored; rebuilt on demand)
	helm repo add bitnami        https://charts.bitnami.com/bitnami
	helm repo add kyverno        https://kyverno.github.io/kyverno/
	helm repo add otwld          https://helm.otwld.com/
	helm repo add metrics-server https://kubernetes-sigs.github.io/metrics-server/
	helm dependency build $(CHART_DIR)

helm-validate: deps ## Schema-validate the rendered manifests for every mode (kubeconform)
	@for m in minimal standard performance; do \
	  echo "== kubeconform $$m =="; \
	  helm template t $(CHART_DIR) --set app.mode=$$m --set app.persistence.storageClass=validate \
	    | kubeconform -strict -summary -ignore-missing-schemas \
	        -schema-location default \
	        -schema-location 'https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json' \
	    || exit 1; \
	done

sync: ## Sync the Go workspace
	go work sync

values-docs: ## Regenerate each chart's VALUES.md index from values.yaml (helm-docs)
	$(HELM_DOCS) --chart-search-root charts --output-file VALUES.md

changelog: ## Regenerate CHANGELOG.md from conventional commits (git-cliff)
	git cliff -o CHANGELOG.md

publish-charts: deps ## Package + push both charts to the OCI registry (run `helm registry login ghcr.io` first)
	rm -rf .cr-release && mkdir -p .cr-release
	helm package $(CRDS_DIR)  --destination .cr-release
	helm package $(CHART_DIR) --destination .cr-release
	@for pkg in .cr-release/*.tgz; do echo "== push $$pkg =="; helm push "$$pkg" $(REGISTRY); done

check: lint test ## Lint and test everything
