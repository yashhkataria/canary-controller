# canary-controller

A Go service that runs canary rollouts on Kubernetes. It shifts traffic in
steps (via replica ratios), checks Prometheus error rate and p99 latency,
and promotes or rolls back automatically. Rollout state lives in MongoDB;
Redis locks give each rollout one owner across controller replicas.
Full design: docs/design.md. Follow it; ask before deviating.

## Stack
Go (stdlib net/http, log/slog), client-go, MongoDB Go driver v2,
go-redis v9, prometheus/client_golang. Local cluster: kind.
Monitoring: kube-prometheus-stack. Load: k6.
Module path: github.com/yashhkataria/canary-controller

## Repo layout
- cmd/controller, cmd/demo-app: main packages only; keep them thin
- internal/api: HTTP handlers
- internal/demoapp: demo app handlers, config and metrics
- internal/rollout: state machine and reconcile loop
- internal/analysis: Prometheus queries and decide()
- internal/k8s: client-go wrappers (scale, set image)
- internal/store: MongoDB repository
- internal/lock: Redis locks
- internal/metrics: the controller's own Prometheus metrics
- deploy/: kind config, Kubernetes manifests, monitoring values, GKE overlay
- loadtest/: k6 scripts; scripts/: experiment runners and tool checks
- docs/: design.md, setup.md, notes.md, results.md, img/
Empty folders hold a .gitkeep; delete it once a folder has real files.

## Conventions
- Propose a plan before writing code; keep each change small.
- context.Context on every call that does I/O; timeouts on all network calls.
- Table-driven tests for pure logic; client-go fake clientset for
  Kubernetes code; testcontainers-go for MongoDB/Redis integration tests.
- No new dependencies without asking.
- Config via env vars, documented in README.
- After each change, explain what you did and why in a few lines.
- Never create cloud resources (GCP) without asking first.

## Commands
make help | make tools | make test | make build-demo
make kind-up | make kind-down | make deploy | make load | make load-stop
make prometheus | make grafana | make alertmanager
Later phases add: make build | make lint

## Status
Phase 0 done: repo layout, go.mod, CLAUDE.md, docs, tool check (make tools).
Phase 1 done: demo app (internal/demoapp, cmd/demo-app), Dockerfile,
make test, make build-demo.
Phase 2 done: kind cluster, kube-prometheus-stack (Alertmanager on, demo
alert rules), demo Deployments + PodMonitor, MongoDB and Redis in
canary-system, k6 Job (make kind-up | deploy | load).
Next: Phase 3, controller API and MongoDB store.
