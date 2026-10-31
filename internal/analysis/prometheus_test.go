package analysis

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/yashhkataria/canary-controller/internal/rollout"
)

type fakeProm struct {
	mu      sync.Mutex
	queries []string
	answers map[string]string
	status  int
	delay   time.Duration
}

func (f *fakeProm) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	q := r.Form.Get("query")
	f.mu.Lock()
	f.queries = append(f.queries, q)
	f.mu.Unlock()
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	if f.status != 0 {
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte(`{"status":"error","errorType":"internal","error":"boom"}`))
		return
	}

	kind := "error_rate"
	switch {
	case strings.Contains(q, "histogram_quantile("):
		kind = "p99"
	case strings.Contains(q, "increase("):
		kind = "requests"
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[%s]}}`, f.answers[kind])
}

func sample(track, value string) string {
	return fmt.Sprintf(`{"metric":{"track":%q},"value":[1761800000,%q]}`, track, value)
}

func newFake(t *testing.T, f *fakeProm, timeout time.Duration) *Prometheus {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	p, err := newWithTimeout(srv.URL, timeout)
	if err != nil {
		t.Fatalf("newWithTimeout: %v", err)
	}
	return p
}

func closeTo(a, b rollout.TrackMetrics) bool {
	near := func(x, y float64) bool { return math.Abs(x-y) < 1e-9 }
	return near(a.ErrorRate, b.ErrorRate) && near(a.P99Ms, b.P99Ms) && near(a.Requests, b.Requests)
}

var testID = bson.NewObjectID()

func testRollout() rollout.Rollout {
	return rollout.Rollout{ID: testID, Namespace: "default", App: "demo", Analysis: rollout.DefaultAnalysis()}
}

func TestCheckReadsBothTracks(t *testing.T) {
	f := &fakeProm{answers: map[string]string{
		"error_rate": sample("stable", "0") + "," + sample("canary", "0.21"),
		"p99":        sample("stable", "0.005") + "," + sample("canary", "0.0049"),
		"requests":   sample("stable", "2700.5") + "," + sample("canary", "280"),
	}}
	got, err := newFake(t, f, time.Second).Check(context.Background(), testRollout(), time.Now())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	wantStable := rollout.TrackMetrics{ErrorRate: 0, P99Ms: 5, Requests: 2700.5}
	wantCanary := rollout.TrackMetrics{ErrorRate: 0.21, P99Ms: 4.9, Requests: 280}
	if !closeTo(got.Stable, wantStable) || !closeTo(got.Canary, wantCanary) {
		t.Errorf("stable %+v canary %+v\nwant  %+v       %+v", got.Stable, got.Canary, wantStable, wantCanary)
	}
	if got.Verdict != rollout.VerdictFail || !strings.Contains(got.Reason, "21.0%") {
		t.Errorf("verdict = %s (%s), want fail on errors", got.Verdict, got.Reason)
	}
}

func TestQueriesAreScopedToTheApp(t *testing.T) {
	f := &fakeProm{answers: map[string]string{}}
	if _, err := newFake(t, f, time.Second).Check(context.Background(), testRollout(), time.Now()); err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(f.queries) != 3 {
		t.Fatalf("ran %d queries, want 3", len(f.queries))
	}
	stableSel := `namespace="default",app="demo",track="stable"`
	canarySel := `namespace="default",app="demo",track="canary",rollout_id="` + testID.Hex() + `"`
	for _, q := range f.queries {
		for _, want := range []string{stableSel, canarySel, `[1m]`, "by (track", ") or ("} {
			if !strings.Contains(q, want) {
				t.Errorf("query %q is missing %s", q, want)
			}
		}
		if strings.Count(q, "rollout_id=") != strings.Count(q, `track="canary"`) {
			t.Errorf("every canary selector must carry rollout_id, and no stable one: %s", q)
		}
	}
	if strings.Count(f.queries[0], `code=~"5.."`) != 2 || strings.Count(f.queries[0], " or (0 * ") != 2 {
		t.Errorf("error rate query must count 5xx and fill missing tracks with 0: %s", f.queries[0])
	}
}

func TestMissingDataBecomesZero(t *testing.T) {
	tests := []struct {
		name    string
		answers map[string]string
		want    rollout.TrackMetrics
	}{
		{name: "track absent from every result", answers: map[string]string{}, want: rollout.TrackMetrics{}},
		{name: "NaN from 0/0 and histogram_quantile", answers: map[string]string{
			"error_rate": sample("canary", "NaN"), "p99": sample("canary", "NaN"), "requests": sample("canary", "0"),
		}, want: rollout.TrackMetrics{}},
		{name: "infinities", answers: map[string]string{
			"error_rate": sample("canary", "+Inf"), "p99": sample("canary", "-Inf"), "requests": sample("canary", "12"),
		}, want: rollout.TrackMetrics{Requests: 12}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newFake(t, &fakeProm{answers: tt.answers}, time.Second).Check(context.Background(), testRollout(), time.Now())
			if err != nil {
				t.Fatalf("Check: %v", err)
			}
			if got.Canary != tt.want {
				t.Errorf("canary = %+v, want %+v", got.Canary, tt.want)
			}
			if got.Verdict != rollout.VerdictInconclusive {
				t.Errorf("verdict = %s, want inconclusive with too little data", got.Verdict)
			}
		})
	}
}

func TestPrometheusFailuresAreErrors(t *testing.T) {
	tests := []struct {
		name string
		fake *fakeProm
	}{
		{name: "server error", fake: &fakeProm{status: http.StatusInternalServerError}},
		{name: "too slow", fake: &fakeProm{delay: 300 * time.Millisecond}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newFake(t, tt.fake, 100*time.Millisecond)
			if _, err := p.Check(context.Background(), testRollout(), time.Now()); err == nil {
				t.Error("want an error")
			}
		})
	}
}

func TestUnreachablePrometheus(t *testing.T) {
	p, err := newWithTimeout("http://127.0.0.1:1", 200*time.Millisecond)
	if err != nil {
		t.Fatalf("newWithTimeout: %v", err)
	}
	if _, err := p.Check(context.Background(), testRollout(), time.Now()); err == nil {
		t.Error("want an error when Prometheus is unreachable")
	}
}
