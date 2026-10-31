package analysis

import (
	"fmt"
	"math"

	"github.com/yashhkataria/canary-controller/internal/rollout"
)

func atLeast(value, bar float64) bool {
	return value >= bar-epsilon
}

func pct(v float64) string {
	return fmt.Sprintf("%.1f%%", v*100)
}

func ms(v float64) string {
	return fmt.Sprintf("%.0f ms", v)
}

func summary(stable, canary rollout.TrackMetrics) string {
	return fmt.Sprintf("canary %s errors, p99 %s, %.0f req; stable %s errors, p99 %s, %.0f req",
		pct(canary.ErrorRate), ms(canary.P99Ms), canary.Requests,
		pct(stable.ErrorRate), ms(stable.P99Ms), stable.Requests)
}

func finite(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

func bothTracks(expr, stableSel, canarySel string) string {
	return "(" + fmt.Sprintf(expr, stableSel, window) + ") or (" + fmt.Sprintf(expr, canarySel, window) + ")"
}
