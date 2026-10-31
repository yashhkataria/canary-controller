package analysis

import (
	"strings"
	"testing"

	"github.com/yashhkataria/canary-controller/internal/rollout"
)

func TestDecide(t *testing.T) {
	healthy := rollout.TrackMetrics{ErrorRate: 0.001, P99Ms: 10, Requests: 2700}
	canary := func(errRate, p99, req float64) rollout.TrackMetrics {
		return rollout.TrackMetrics{ErrorRate: errRate, P99Ms: p99, Requests: req}
	}

	tests := []struct {
		name       string
		settings   func(*rollout.Analysis)
		stable     rollout.TrackMetrics
		canary     rollout.TrackMetrics
		want       rollout.Verdict
		wantReason string
	}{
		{name: "healthy", stable: healthy, canary: canary(0.001, 10, 300), want: rollout.VerdictPass, wantReason: "canary 0.1% errors"},
		{name: "canary errors", stable: healthy, canary: canary(0.21, 10, 300), want: rollout.VerdictFail, wantReason: "error rate 21.0%"},
		{name: "canary slow", stable: healthy, canary: canary(0.001, 995, 300), want: rollout.VerdictFail, wantReason: "p99 995 ms"},
		{name: "canary errors and slow reports errors first", stable: healthy, canary: canary(0.21, 995, 300), want: rollout.VerdictFail, wantReason: "error rate"},
		{name: "stable also failing on errors", stable: canary(0.05, 10, 2700), canary: canary(0.05, 10, 300), want: rollout.VerdictInconclusive, wantReason: "stable is also unhealthy"},
		{name: "stable also failing on latency", stable: canary(0.001, 900, 2700), canary: canary(0.001, 950, 300), want: rollout.VerdictInconclusive, wantReason: "stable is also unhealthy"},
		{name: "canary worse while stable is unhealthy still fails", stable: canary(0.05, 10, 2700), canary: canary(0.30, 10, 300), want: rollout.VerdictFail},
		{name: "too little traffic", stable: healthy, canary: canary(0.5, 2000, 49), want: rollout.VerdictInconclusive, wantReason: "served 49 requests"},
		{name: "exactly minRequests is judged", stable: healthy, canary: canary(0.001, 10, 50), want: rollout.VerdictPass},
		{name: "no traffic at all", stable: healthy, canary: canary(0, 0, 0), want: rollout.VerdictInconclusive},
		{name: "minRequests 0 judges any traffic", settings: func(a *rollout.Analysis) { a.MinRequests = 0 }, stable: healthy, canary: canary(0, 0, 0), want: rollout.VerdictPass},
		{name: "stable has no traffic at the 100% step", stable: rollout.TrackMetrics{}, canary: canary(0.001, 10, 3000), want: rollout.VerdictPass},
		{name: "stable has no traffic and canary errors", stable: rollout.TrackMetrics{}, canary: canary(0.05, 10, 3000), want: rollout.VerdictFail},

		{name: "error rate exactly maxErrorRate fails", stable: canary(0, 10, 2700), canary: canary(0.02, 10, 300), want: rollout.VerdictFail},
		{name: "error rate just below maxErrorRate passes", stable: canary(0, 10, 2700), canary: canary(0.0199, 10, 300), want: rollout.VerdictPass},
		{name: "error rate exactly stable + errorMargin fails", stable: canary(0.015, 10, 2700), canary: canary(0.025, 10, 300), want: rollout.VerdictFail},
		{name: "error rate just below stable + errorMargin passes", stable: canary(0.015, 10, 2700), canary: canary(0.0249, 10, 300), want: rollout.VerdictPass},
		{name: "float rounding: 0.2 + 0.1 still counts 0.3 as at the bar", stable: canary(0.2, 10, 2700), canary: canary(0.3, 10, 300), settings: func(a *rollout.Analysis) { a.ErrorMargin = 0.1; a.MaxErrorRate = 0.25 }, want: rollout.VerdictFail},

		{name: "p99 exactly maxP99Ms fails", stable: canary(0, 300, 2700), canary: canary(0, 500, 300), want: rollout.VerdictFail},
		{name: "p99 just below maxP99Ms passes", stable: canary(0, 300, 2700), canary: canary(0, 499.9, 300), want: rollout.VerdictPass},
		{name: "p99 exactly stable x latencyFactor fails", stable: canary(0, 400, 2700), canary: canary(0, 600, 300), want: rollout.VerdictFail},
		{name: "p99 just below stable x latencyFactor passes", stable: canary(0, 400, 2700), canary: canary(0, 599.9, 300), want: rollout.VerdictPass},

		{name: "stable exactly at maxErrorRate is unhealthy", stable: canary(0.02, 10, 2700), canary: canary(0.02, 10, 300), want: rollout.VerdictInconclusive},
		{name: "stable exactly at maxP99Ms is unhealthy", stable: canary(0, 500, 2700), canary: canary(0, 500, 300), want: rollout.VerdictInconclusive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := rollout.DefaultAnalysis()
			if tt.settings != nil {
				tt.settings(&a)
			}
			got, reason := decide(a, tt.stable, tt.canary)
			if got != tt.want {
				t.Fatalf("verdict = %s (%s), want %s", got, reason, tt.want)
			}
			if tt.wantReason != "" && !strings.Contains(reason, tt.wantReason) {
				t.Errorf("reason = %q, want it to contain %q", reason, tt.wantReason)
			}
		})
	}
}
