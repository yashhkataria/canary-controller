.DEFAULT_GOAL := help

DOCKER ?= docker
DEMO_DOCKERFILE := cmd/demo-app/Dockerfile

.PHONY: help tools test build-demo

help: ## List available targets
	@grep -E '^[a-zA-Z0-9_-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-12s %s\n", $$1, $$2}'

tools: ## Check that the required local tools are installed
	@bash scripts/check-tools.sh

test: ## Run unit tests with the race detector
	go test -race ./...

build-demo: ## Build demo:v1, demo:v2, demo:v2-bad (20% errors), demo:v2-slow (+800 ms)
	$(DOCKER) build -f $(DEMO_DOCKERFILE) -t demo:v1 --build-arg VERSION=v1 .
	$(DOCKER) build -f $(DEMO_DOCKERFILE) -t demo:v2 --build-arg VERSION=v2 .
	$(DOCKER) build -f $(DEMO_DOCKERFILE) -t demo:v2-bad --build-arg VERSION=v2-bad --build-arg ERROR_RATE=0.2 .
	$(DOCKER) build -f $(DEMO_DOCKERFILE) -t demo:v2-slow --build-arg VERSION=v2-slow --build-arg EXTRA_LATENCY_MS=800 .
