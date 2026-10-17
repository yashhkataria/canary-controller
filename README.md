# canary-controller

A canary deployment controller for Kubernetes, written in Go. It shifts traffic to a new version in stages, checks Prometheus error rate and p99 latency after each stage, and promotes or rolls back automatically.

> **Status:** work in progress. Phase 0 (project setup) is done; see the roadmap below.

## How it works

- Two Deployments, `demo-stable` and `demo-canary`, sit behind one Service. The replica ratio sets the traffic split: 10% → 50% → 100%.
- After each step the controller compares canary and stable in Prometheus and decides: next step, promote, or roll back.
- Rollout state lives in MongoDB. Per-rollout Redis locks let two controller replicas share the work and survive a crash.

Design details: [docs/design.md](docs/design.md)

## Stack

Go · Kubernetes (client-go) · Docker · MongoDB · Redis · Prometheus · Grafana · k6 · kind · GKE · GitHub Actions

## Roadmap

- [x] Phase 0: project setup
- [ ] Phase 1: demo app with error and latency injection
- [ ] Phase 2: local cluster with Prometheus and steady load
- [ ] Phase 3: controller API and MongoDB store
- [ ] Phase 4: rollout engine
- [ ] Phase 5: automated analysis and rollback
- [ ] Phase 6: high availability with Redis locks
- [ ] Phase 7: observability
- [ ] Phase 8: load tests and measured results
- [ ] Phase 9: deployment on GKE
- [ ] Phase 10: CI and the full README

## Running it locally

See [docs/setup.md](docs/setup.md). Quickstart commands will land here as the phases are built.

## License

MIT, see [LICENSE](LICENSE).
