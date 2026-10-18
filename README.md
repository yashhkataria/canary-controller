# canary-controller

A canary deployment controller for Kubernetes, written in Go. It shifts traffic to a new version in stages, checks Prometheus error rate and p99 latency after each stage, and promotes or rolls back automatically.

> **Status:** work in progress. Phases 0–2 (project setup, demo app, local cluster) are done; see the roadmap below.

## How it works

- Two Deployments, `demo-stable` and `demo-canary`, sit behind one Service. The replica ratio sets the traffic split: 10% → 50% → 100%.
- After each step the controller compares canary and stable in Prometheus and decides: next step, promote, or roll back.
- Rollout state lives in MongoDB. Per-rollout Redis locks let two controller replicas share the work and survive a crash.

Design details: [docs/design.md](docs/design.md)

## Stack

Go · Kubernetes (client-go) · Docker · MongoDB · Redis · Prometheus · Grafana · k6 · kind · GKE · GitHub Actions

## Roadmap

- [x] Phase 0: project setup
- [x] Phase 1: demo app with error and latency injection
- [x] Phase 2: local cluster with Prometheus and steady load
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

```bash
make test        # unit tests
make kind-up     # kind cluster "canary" (1 control plane, 2 workers) with the demo images loaded
make deploy      # kube-prometheus-stack, MongoDB, Redis, demo-stable (10 pods) and demo-canary (0 pods)
make load        # steady k6 load on the demo Service; make load RATE=100 DURATION=30m
make prometheus  # http://localhost:9090 (also: make grafana, make alertmanager)
make kind-down   # delete the cluster
```

Grafana's login is `admin`; read the password with:

```bash
kubectl --context kind-canary -n monitoring get secret monitoring-grafana -o jsonpath='{.data.admin-password}' | base64 -d
```

Alerts for the demo app (error rate, p99 latency, stable capacity, scrape failures) are in `deploy/k8s/demo/alerts.yaml` and show up in Prometheus and Alertmanager. Alertmanager has no notification receiver yet, so alerts are visible in its UI but not sent anywhere.

## Configuration

### Demo app (`cmd/demo-app`)

Listens on `:8080` and serves `GET /` (JSON with version and track), `GET /healthz` and `GET /metrics`. Error and latency injection apply to `GET /` only, so probes stay healthy.

| Env var | Meaning | Default |
| --- | --- | --- |
| `VERSION` | Reported in responses | image build arg, else `dev` |
| `TRACK` | `stable` or `canary`; the `track` metric label | `unknown` (set it in the Deployment) |
| `ERROR_RATE` | Fraction of `GET /` requests answered with 500, 0 to 1 | image build arg, else `0` |
| `EXTRA_LATENCY_MS` | Delay in ms added to every `GET /` request | image build arg, else `0` |

Metrics: `http_requests_total{track, code}` and `http_request_duration_seconds{track}` (histogram), counting `GET /` only.

### Local environment (`make` variables)

| Variable | Meaning | Default |
| --- | --- | --- |
| `RATE` | Requests per second `make load` sends to the demo Service | `50` |
| `DURATION` | How long `make load` runs | `10m` |
| `CLUSTER` | kind cluster name | `canary` |
| `KUBE_CONTEXT` | kubectl/Helm context every target uses | `kind-$(CLUSTER)` |

## License

MIT, see [LICENSE](LICENSE).
