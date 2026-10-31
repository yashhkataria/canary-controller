package analysis

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"github.com/yashhkataria/canary-controller/internal/rollout"
)

func New(address string) (*Prometheus, error) {
	return newWithTimeout(address, queryTimeout)
}

func newWithTimeout(address string, timeout time.Duration) (*Prometheus, error) {
	c, err := api.NewClient(api.Config{Address: address, Client: &http.Client{Timeout: timeout}})
	if err != nil {
		return nil, fmt.Errorf("prometheus client: %w", err)
	}
	return &Prometheus{api: v1.NewAPI(c), timeout: timeout}, nil
}

func (p *Prometheus) Check(ctx context.Context, r rollout.Rollout, now time.Time) (rollout.LastCheck, error) {
	stableSel := fmt.Sprintf(stableSelector, r.Namespace, r.App)
	canarySel := fmt.Sprintf(canarySelector, r.Namespace, r.App, r.ID.Hex())

	errorRates, err := p.byTrack(ctx, "error rate", bothTracks(errorRateExpr, stableSel, canarySel), now)
	if err != nil {
		return rollout.LastCheck{}, err
	}
	p99s, err := p.byTrack(ctx, "p99", bothTracks(p99Expr, stableSel, canarySel), now)
	if err != nil {
		return rollout.LastCheck{}, err
	}
	requests, err := p.byTrack(ctx, "request count", bothTracks(requestsExpr, stableSel, canarySel), now)
	if err != nil {
		return rollout.LastCheck{}, err
	}

	track := func(name string) rollout.TrackMetrics {
		return rollout.TrackMetrics{
			ErrorRate: finite(errorRates[name]),
			P99Ms:     finite(p99s[name]) * 1000,
			Requests:  finite(requests[name]),
		}
	}
	stable, canary := track(trackStable), track(trackCanary)
	verdict, reason := decide(r.Analysis, stable, canary)
	return rollout.LastCheck{At: now, Stable: stable, Canary: canary, Verdict: verdict, Reason: reason}, nil
}

func (p *Prometheus) byTrack(ctx context.Context, what, query string, at time.Time) (map[string]float64, error) {
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	val, _, _, err := p.api.Query(ctx, query, at, v1.WithTimeout(p.timeout))
	if err != nil {
		return nil, fmt.Errorf("prometheus %s query: %w", what, err)
	}
	vec, ok := val.(model.Vector)
	if !ok {
		return nil, fmt.Errorf("prometheus %s query returned %s, want a vector", what, val.Type())
	}
	out := map[string]float64{}
	for _, s := range vec {
		out[string(s.Metric[labelTrack])] = float64(s.Value)
	}
	return out, nil
}
