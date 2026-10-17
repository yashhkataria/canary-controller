package demoapp

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricLabels(t *testing.T) {
	tests := []struct {
		track    string
		rate     float64
		requests int
		want200  float64
		want500  float64
	}{
		{track: "stable", rate: 0, requests: 10, want200: 10, want500: 0},
		{track: "canary", rate: 0.2, requests: 10, want200: 8, want500: 2},
		{track: "canary", rate: 1, requests: 5, want200: 0, want500: 5},
	}
	for _, tt := range tests {
		t.Run(tt.track, func(t *testing.T) {
			app := New(Config{Version: "v2", Track: tt.track, ErrorRate: tt.rate}, steppedRand())
			h := app.Handler()
			for range tt.requests {
				get(t, h, "/")
			}
			for _, path := range []string{"/healthz", "/metrics", "/nope"} {
				get(t, h, path)
			}

			c := app.metrics.requests
			if got := testutil.ToFloat64(c.WithLabelValues(tt.track, "200")); got != tt.want200 {
				t.Errorf(`http_requests_total{code="200"} = %v, want %v`, got, tt.want200)
			}
			if got := testutil.ToFloat64(c.WithLabelValues(tt.track, "500")); got != tt.want500 {
				t.Errorf(`http_requests_total{code="500"} = %v, want %v`, got, tt.want500)
			}

			counter := family(t, app, "http_requests_total")
			assertLabelNames(t, counter, "code", "track")
			var total float64
			for _, m := range counter.GetMetric() {
				total += m.GetCounter().GetValue()
				if v := labelValue(m, "track"); v != tt.track {
					t.Errorf("series with track=%q, want only %q", v, tt.track)
				}
			}
			if total != float64(tt.requests) {
				t.Errorf("http_requests_total summed = %v, want %d (probes or 404s were counted)", total, tt.requests)
			}

			hist := family(t, app, "http_request_duration_seconds")
			assertLabelNames(t, hist, "track")
			if n := len(hist.GetMetric()); n != 1 {
				t.Fatalf("histogram has %d series, want 1", n)
			}
			m := hist.GetMetric()[0]
			if v := labelValue(m, "track"); v != tt.track {
				t.Errorf("histogram track = %q, want %q", v, tt.track)
			}
			if got := m.GetHistogram().GetSampleCount(); got != uint64(tt.requests) {
				t.Errorf("histogram sample count = %d, want %d", got, tt.requests)
			}
		})
	}
}
