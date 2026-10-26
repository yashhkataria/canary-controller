package rollout

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/yashhkataria/canary-controller/internal/logging"
)

func NewEngine(s Store, c Cluster, a Analyzer, o EngineOptions) *Engine {
	if o.Interval <= 0 {
		o.Interval = DefaultEngineInterval
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	if o.Now == nil {
		o.Now = func() time.Time { return time.Now().UTC().Truncate(time.Millisecond) }
	}
	return &Engine{store: s, cluster: c, analyzer: a, owner: o.Owner, interval: o.Interval, now: o.Now, logger: o.Logger}
}

func (e *Engine) Run(ctx context.Context) {
	e.logger.Info("rollout engine started", "interval", e.interval.String(), "owner", e.owner)
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()
	for {
		e.Tick(ctx)
		select {
		case <-ctx.Done():
			e.logger.Info("rollout engine stopped")
			return
		case <-ticker.C:
		}
	}
}

func (e *Engine) Tick(ctx context.Context) {
	work := context.WithoutCancel(ctx)

	listCtx, cancel := context.WithTimeout(work, reconcileTimeout)
	active, err := e.store.ListActive(listCtx)
	cancel()
	if err != nil {
		e.logger.Warn("list active rollouts", "err", err)
		return
	}

	for _, r := range active {
		if ctx.Err() != nil {
			return
		}
		log := e.logger.With("rollout_id", r.ID.Hex(), "app", r.Namespace+"/"+r.App, "phase", string(r.Phase))
		rctx, cancel := context.WithTimeout(logging.With(work, log), reconcileTimeout)
		err := e.Reconcile(rctx, r)
		cancel()
		switch {
		case err == nil:
		case errors.Is(err, ErrVersionConflict):
			log.Info("rollout changed while reconciling; retrying next tick")
		default:
			log.Warn("reconcile failed; retrying next tick", "err", err)
		}
	}
}
