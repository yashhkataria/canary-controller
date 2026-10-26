package rollout

import (
	"context"

	"github.com/yashhkataria/canary-controller/internal/logging"
)

func (e *Engine) save(ctx context.Context, r Rollout, mutate func(*Rollout), eventType, message string) error {
	from := r.Phase
	mutate(&r)
	updated, err := e.store.UpdateRollout(ctx, r)
	if err != nil {
		return err
	}
	log := logging.From(ctx)
	if from != updated.Phase {
		log.Info("phase changed", "from", string(from), "to", string(updated.Phase), "step", updated.CurrentStep, "version", updated.Version)
	}
	if eventType == "" {
		return nil
	}
	if _, err := e.store.AppendEvent(ctx, Event{RolloutID: updated.ID, At: e.now(), Type: eventType, Message: message}); err != nil {
		log.Warn("write event", "type", eventType, "err", err)
	}
	return nil
}

func (e *Engine) beginRollback(ctx context.Context, r Rollout, reason string) error {
	now := e.now()
	return e.save(ctx, r, func(x *Rollout) {
		x.Phase = PhaseRollingBack
		x.Reason = reason
		x.StepStartedAt = nil
		x.WaitingSince = &now
	}, EventRollingBack, reason)
}

func (e *Engine) beginPromotion(ctx context.Context, r Rollout, message string) error {
	now := e.now()
	return e.save(ctx, r, func(x *Rollout) {
		x.Phase = PhasePromoting
		x.StepStartedAt = nil
		x.WaitingSince = &now
	}, EventPromoting, message)
}

func (e *Engine) finish(ctx context.Context, r Rollout, phase Phase, reason, eventType, message string) error {
	now := e.now()
	return e.save(ctx, r, func(x *Rollout) {
		x.Phase = phase
		x.Active = false
		x.Reason = reason
		x.FinishedAt = &now
		x.StepStartedAt = nil
		x.WaitingSince = nil
	}, eventType, message)
}
