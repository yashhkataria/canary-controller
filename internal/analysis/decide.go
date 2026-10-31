package analysis

import (
	"fmt"

	"github.com/yashhkataria/canary-controller/internal/rollout"
)

func decide(a rollout.Analysis, stable, canary rollout.TrackMetrics) (rollout.Verdict, string) {
	if canary.Requests < float64(a.MinRequests) {
		return rollout.VerdictInconclusive, fmt.Sprintf("canary served %.0f requests in the last %s, need %d",
			canary.Requests, window, a.MinRequests)
	}

	errorBar := stable.ErrorRate + a.ErrorMargin
	if atLeast(canary.ErrorRate, a.MaxErrorRate) && atLeast(canary.ErrorRate, errorBar) {
		return rollout.VerdictFail, fmt.Sprintf("canary error rate %s >= %s and >= stable %s + %s",
			pct(canary.ErrorRate), pct(a.MaxErrorRate), pct(stable.ErrorRate), pct(a.ErrorMargin))
	}

	latencyBar := stable.P99Ms * a.LatencyFactor
	if atLeast(canary.P99Ms, a.MaxP99Ms) && atLeast(canary.P99Ms, latencyBar) {
		return rollout.VerdictFail, fmt.Sprintf("canary p99 %s >= %s and >= stable %s x %g",
			ms(canary.P99Ms), ms(a.MaxP99Ms), ms(stable.P99Ms), a.LatencyFactor)
	}

	if atLeast(stable.ErrorRate, a.MaxErrorRate) || atLeast(stable.P99Ms, a.MaxP99Ms) {
		return rollout.VerdictInconclusive, fmt.Sprintf("stable is also unhealthy (%s errors, p99 %s); not blaming the canary",
			pct(stable.ErrorRate), ms(stable.P99Ms))
	}

	return rollout.VerdictPass, summary(stable, canary)
}
