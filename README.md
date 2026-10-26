# canary-controller

A canary deployment controller for Kubernetes, written in Go. It shifts traffic to a new version in stages, checks Prometheus error rate and p99 latency after each stage, and promotes or rolls back automatically.

> **Status:** work in progress. Phases 0–4 (project setup, demo app, local cluster, controller API and store, rollout engine) are done; see the roadmap below.

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
- [x] Phase 3: controller API and MongoDB store
- [x] Phase 4: rollout engine
- [ ] Phase 5: automated analysis and rollback
- [ ] Phase 6: high availability with Redis locks
- [ ] Phase 7: observability
- [ ] Phase 8: load tests and measured results
- [ ] Phase 9: deployment on GKE
- [ ] Phase 10: CI and the full README

## Running it locally

See [docs/setup.md](docs/setup.md). Quickstart commands will land here as the phases are built.

```bash
make test        # all tests; the MongoDB integration tests need Docker (make test-unit skips them)
make kind-up     # kind cluster "canary" (1 control plane, 2 workers) with the demo and controller images loaded
make deploy      # kube-prometheus-stack, MongoDB, Redis, demo-stable (10 pods), demo-canary (0 pods), controller
make load        # steady k6 load on the demo Service; make load RATE=100 DURATION=30m
make prometheus  # http://localhost:9090 (also: make grafana, make alertmanager)
make controller  # controller API on http://localhost:8081
make kind-down   # delete the cluster
```

Grafana's login is `admin`; read the password with:

```bash
kubectl --context kind-canary -n monitoring get secret monitoring-grafana -o jsonpath='{.data.admin-password}' | base64 -d
```

Alerts for the demo app (error rate, p99 latency, stable capacity, scrape failures) are in `deploy/k8s/demo/alerts.yaml` and show up in Prometheus and Alertmanager. Alertmanager has no notification receiver yet, so alerts are visible in its UI but not sent anywhere.

### Using the controller API

With `make controller` running in another terminal:

```bash
curl -s -X POST localhost:8081/v1/create-rollout -H 'Content-Type: application/json' \
  -d '{"namespace":"default","app":"demo","image":"demo:v2-bad","steps":[10,50,100]}'
```

```bash
curl -s 'localhost:8081/v1/rollouts?app=demo'
```

```bash
curl -s localhost:8081/v1/rollouts/<id>/events
```

The engine picks the rollout up within 2 s and moves pods step by step (1/9, 5/5, 10/0), then promotes: demo-stable runs the new image on 10 pods and demo-canary goes back to 0. Until Phase 5 each step passes after a fixed wait of `interval × checksPerStep` (60 s by default; send `"analysis":{"interval":"5s","checksPerStep":2}` to try it quickly).

```bash
curl -s -X POST localhost:8081/v1/rollouts/<id>/abort
```

```bash
curl -s -X POST localhost:8081/v1/rollouts/<id>/promote
```

Abort rolls back (stable to 10 first, then canary to 0); promote skips the remaining steps. Both return 202, or 409 when the rollout's phase doesn't allow it. Watch it happen with `kubectl get deploy demo-stable demo-canary -w`.

## Configuration

### Controller (`cmd/controller`)

| Env var | Meaning | Default |
| --- | --- | --- |
| `PORT` | HTTP port | `8080` |
| `MONGO_URI` | MongoDB connection string | `mongodb://mongodb.canary-system:27017` |
| `MONGO_DATABASE` | Database for `rollouts` and `events` | `canary` |
| `PROMETHEUS_URL` | Prometheus base URL (used from Phase 5) | `http://monitoring-kube-prometheus-prometheus.monitoring:9090` |
| `REDIS_ADDR` | Redis `host:port` (used from Phase 6) | `redis.canary-system:6379` |
| `ENGINE_INTERVAL` | How often the engine scans active rollouts (500ms to 1m) | `2s` |
| `KUBECONFIG` | Kubeconfig to use when not running in a cluster | in-cluster config, then `~/.kube/config` |
| `POD_NAME` | This replica's name, stored as a rollout's `owner` | the Deployment sets it; falls back to `HOSTNAME` |
| `LOG_LEVEL` | `debug`, `info`, `warn` or `error` | `info` |

Invalid values stop the controller at startup. Probe requests (`/healthz`, `/readyz`, `/metrics`) are logged at `debug`.

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
