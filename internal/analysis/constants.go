package analysis

import "time"

const (
	window       = "1m"
	queryTimeout = 5 * time.Second
	epsilon      = 1e-9
)

const (
	trackStable = "stable"
	trackCanary = "canary"
	labelTrack  = "track"
)

const (
	errorRateExpr = `(sum by (track) (rate(http_requests_total{%[1]s,code=~"5.."}[%[2]s]))` +
		` or (0 * sum by (track) (rate(http_requests_total{%[1]s}[%[2]s]))))` +
		` / sum by (track) (rate(http_requests_total{%[1]s}[%[2]s]))`
	p99Expr        = `histogram_quantile(0.99, sum by (track, le) (rate(http_request_duration_seconds_bucket{%[1]s}[%[2]s])))`
	requestsExpr   = `sum by (track) (increase(http_requests_total{%[1]s}[%[2]s]))`
	stableSelector = `namespace=%q,app=%q,track="stable"`
	canarySelector = `namespace=%q,app=%q,track="canary",rollout_id=%q`
)
