.DEFAULT_GOAL := help

DOCKER ?= docker
DEMO_DOCKERFILE := cmd/demo-app/Dockerfile
DEMO_IMAGES := demo:v1 demo:v2 demo:v2-bad demo:v2-slow
CONTROLLER_DOCKERFILE := cmd/controller/Dockerfile
CONTROLLER_IMAGE := controller:dev

CLUSTER ?= canary
KUBE_CONTEXT ?= kind-$(CLUSTER)
KUBECTL := kubectl --context $(KUBE_CONTEXT)

CHART_VERSION := 92.10.1
RATE ?= 50
DURATION ?= 10m

.PHONY: help tools test test-unit build-demo build-controller kind-up kind-down deploy controller-reload load load-stop prometheus grafana alertmanager controller

help: ## List available targets
	@grep -E '^[a-zA-Z0-9_-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-18s %s\n", $$1, $$2}'

tools: ## Check that the required local tools are installed
	@bash scripts/check-tools.sh

test: ## Run all tests with the race detector (integration tests need Docker)
	go test -race ./...

test-unit: ## Run unit tests only, skipping the Docker-based integration tests
	go test -race -short ./...

build-demo: ## Build demo:v1, demo:v2, demo:v2-bad (20% errors), demo:v2-slow (+800 ms)
	$(DOCKER) build -f $(DEMO_DOCKERFILE) -t demo:v1 --build-arg VERSION=v1 .
	$(DOCKER) build -f $(DEMO_DOCKERFILE) -t demo:v2 --build-arg VERSION=v2 .
	$(DOCKER) build -f $(DEMO_DOCKERFILE) -t demo:v2-bad --build-arg VERSION=v2-bad --build-arg ERROR_RATE=0.2 .
	$(DOCKER) build -f $(DEMO_DOCKERFILE) -t demo:v2-slow --build-arg VERSION=v2-slow --build-arg EXTRA_LATENCY_MS=800 .

build-controller: ## Build the controller image (controller:dev)
	$(DOCKER) build -f $(CONTROLLER_DOCKERFILE) -t $(CONTROLLER_IMAGE) .

kind-up: build-demo build-controller ## Create the kind cluster and load the demo and controller images into it
	@kind get clusters | grep -qx $(CLUSTER) || kind create cluster --config deploy/kind/cluster.yaml
	kind load docker-image $(DEMO_IMAGES) $(CONTROLLER_IMAGE) --name $(CLUSTER)

kind-down: ## Delete the kind cluster
	kind delete cluster --name $(CLUSTER)

deploy: ## Install monitoring, MongoDB, Redis, the demo app and the controller
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update
	helm upgrade --install monitoring prometheus-community/kube-prometheus-stack \
		--kube-context $(KUBE_CONTEXT) --namespace monitoring --create-namespace \
		--version $(CHART_VERSION) --values deploy/monitoring/values.yaml --wait --timeout 10m
	$(KUBECTL) apply -k deploy/k8s/infra
	$(KUBECTL) apply -k deploy/k8s/demo
	$(KUBECTL) -n canary-system rollout status statefulset/mongodb --timeout 5m
	$(KUBECTL) -n canary-system rollout status statefulset/redis --timeout 5m
	$(KUBECTL) apply -k deploy/k8s/controller
	$(KUBECTL) -n default rollout status deployment/demo-stable --timeout 5m
	$(KUBECTL) -n canary-system rollout status deployment/controller --timeout 5m

controller-reload: build-controller ## Rebuild the controller, load it into kind and restart it
	kind load docker-image $(CONTROLLER_IMAGE) --name $(CLUSTER)
	$(KUBECTL) -n canary-system rollout restart deployment/controller
	$(KUBECTL) -n canary-system rollout status deployment/controller --timeout 5m

load: ## Start steady k6 load on the demo Service (RATE=50 requests/s, DURATION=10m)
	$(KUBECTL) -n default delete job k6-steady --ignore-not-found --wait
	$(KUBECTL) -n default create configmap k6-script --from-file=loadtest/steady.js --dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUBECTL) -n default create configmap k6-config --from-literal=RATE=$(RATE) --from-literal=DURATION=$(DURATION) --dry-run=client -o yaml | $(KUBECTL) apply -f -
	$(KUBECTL) -n default apply -f deploy/k8s/load/job.yaml

load-stop: ## Stop the k6 load
	$(KUBECTL) -n default delete job k6-steady --ignore-not-found

prometheus: ## Open Prometheus on http://localhost:9090
	$(KUBECTL) -n monitoring port-forward svc/monitoring-kube-prometheus-prometheus 9090:9090

grafana: ## Open Grafana on http://localhost:3000
	$(KUBECTL) -n monitoring port-forward svc/monitoring-grafana 3000:80

alertmanager: ## Open Alertmanager on http://localhost:9093
	$(KUBECTL) -n monitoring port-forward svc/monitoring-kube-prometheus-alertmanager 9093:9093

controller: ## Open the controller API on http://localhost:8081
	$(KUBECTL) -n canary-system port-forward svc/controller 8081:80
