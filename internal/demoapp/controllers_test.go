package demoapp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestErrorInjection(t *testing.T) {
	tests := []struct {
		rate    float64
		want500 int
	}{
		{rate: 0, want500: 0},
		{rate: 0.2, want500: 20},
		{rate: 0.5, want500: 50},
		{rate: 1, want500: 100},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			app := New(Config{Version: "v2", Track: "canary", ErrorRate: tt.rate}, steppedRand())
			h := app.Handler()

			got500 := 0
			for range 100 {
				switch code := get(t, h, "/").Code; code {
				case http.StatusInternalServerError:
					got500++
				case http.StatusOK:
				default:
					t.Fatalf("unexpected status %d", code)
				}
			}
			if got500 != tt.want500 {
				t.Errorf("rate %v: got %d 500s out of 100, want %d", tt.rate, got500, tt.want500)
			}
		})
	}
}

func TestInjectionSkipsProbesAndMetrics(t *testing.T) {
	app := New(Config{Track: "canary", ErrorRate: 1, ExtraLatency: time.Second}, steppedRand())
	h := app.Handler()

	start := time.Now()
	for _, path := range []string{"/healthz", "/metrics"} {
		if code := get(t, h, path).Code; code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, code)
		}
	}
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Errorf("probes took %v; latency should not be injected", d)
	}
}

func TestRootResponse(t *testing.T) {
	app := New(Config{Version: "v1", Track: "stable"}, steppedRand())
	rec := get(t, app.Handler(), "/")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	want := map[string]string{"version": "v1", "track": "stable"}
	if len(body) != len(want) || body["version"] != want["version"] || body["track"] != want["track"] {
		t.Errorf("body = %v, want %v", body, want)
	}
}

func TestUnknownPathIs404(t *testing.T) {
	app := New(Config{Track: "stable"}, steppedRand())
	if code := get(t, app.Handler(), "/nope").Code; code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", code)
	}
}

func TestExtraLatency(t *testing.T) {
	const delay = 50 * time.Millisecond
	app := New(Config{Track: "canary", ExtraLatency: delay}, steppedRand())

	start := time.Now()
	get(t, app.Handler(), "/")
	if d := time.Since(start); d < delay {
		t.Errorf("request took %v, want at least %v", d, delay)
	}

	m := family(t, app, "http_request_duration_seconds").GetMetric()[0]
	if got := m.GetHistogram().GetSampleSum(); got < delay.Seconds() {
		t.Errorf("recorded duration %vs, want at least %vs", got, delay.Seconds())
	}
}

func TestCancelledRequestIsNotRecorded(t *testing.T) {
	app := New(Config{Track: "canary", ExtraLatency: time.Minute}, steppedRand())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	app.Handler().ServeHTTP(httptest.NewRecorder(), req)

	if n := testutil.CollectAndCount(app.metrics.requests); n != 0 {
		t.Errorf("got %d request series after a cancelled request, want 0", n)
	}
}
