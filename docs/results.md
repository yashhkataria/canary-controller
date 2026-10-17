# Results

Filled in during Phase 8 (local) and Phase 9 (GKE). Every number is the median of 5 runs unless stated. Raw runs go in `docs/results.csv`.

## Test setup

- **Cluster:** kind, 1 control-plane + 2 workers, on <your machine>
- **Load:** k6 at ___ RPS, new connection per request
- **Analysis settings:** interval 20 s, checksPerStep 3, failureLimit 1, maxErrorRate 0.02, maxP99Ms 500 (update if you tune them)
- **Prometheus scrape interval:** 5 s

## Results

| Metric | How it's measured | Local (kind) | GKE |
| --- | --- | --- | --- |
| Detection time, bad release (`demo:v2-bad`) | Canary ready at step 1 → rollback decision, from events | | |
| Detection time, slow release (`demo:v2-slow`) | Same, for the slow image | | |
| Failed requests from a bad canary | Canary 5xx before rollback ÷ all requests in that window | | |
| Failover gap | Old owner's last event → new owner's first event | | |
| Failed requests in a healthy rollout | k6 failed ÷ total during a `demo:v2` rollout | | |
| Healthy rollout duration | Created → Succeeded | | |

## Tuning experiment

Change one setting, rerun, and record the effect.

| Setting changed | Before | After | Detection time before → after | False rollbacks before → after |
| --- | --- | --- | --- | --- |
| | | | | |

## Notes

Anything unusual about the runs (machine load, a flaky run you excluded and why).
