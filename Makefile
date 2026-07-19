export GOPRIVATE := github.com/telark/*

GO_SERVICES := auth discovery exporter notifier
CHART_DIR    := charts/telark
CRDS_DIR     := charts/telark-crds
NATS_CONF    := --set-file nats.configuration=$(CHART_DIR)/config/nats.conf

.PHONY: help build test lint fmt vet helm-lint helm-template sync check

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

lint: helm-lint ## golangci-lint per service (each service's own config) + helm lint
	@for s in $(GO_SERVICES); do echo "== lint $$s =="; (cd services/$$s && golangci-lint run) || exit 1; done

helm-lint: ## Lint both charts
	helm lint $(CRDS_DIR) -f $(CHART_DIR)/values.yaml
	helm lint $(CHART_DIR)

helm-template: ## Render the app chart (standard mode)
	helm template t $(CHART_DIR) -f $(CHART_DIR)/values.mode.standard.yaml $(NATS_CONF)

sync: ## Sync the Go workspace
	go work sync

check: lint test ## Lint and test everything
