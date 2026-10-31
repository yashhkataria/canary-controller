package rollout

import (
	"context"

	"github.com/yashhkataria/canary-controller/internal/logging"
)

func (e *Engine) save(ctx context.Context, r Rollout, mutate func(*Rollout), eventType, message string, before ...Event) error {
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
	events := before
	if eventType != "" {
		events = append(events, Event{Type: eventType, Message: message})
	}
	for _, ev := range events {
		ev.RolloutID = updated.ID
		if ev.At.IsZero() {
			ev.At = e.now()
		}
		if _, err := e.store.AppendEvent(ctx, ev); err != nil {
			log.Warn("write event", "type", ev.Type, "err", err)
		}
	}
	return nil
}

func (e *Engine) beginRollback(ctx context.Context, r Rollout, reason string, before ...Event) error {
	now := e.now()
	return e.save(ctx, r, func(x *Rollout) {
		x.Phase = PhaseRollingBack
		x.Reason = reason
		x.StepStartedAt = nil
		x.WaitingSince = &now
	}, EventRollingBack, reason, before...)
}

func (e *Engine) beginPromotion(ctx context.Context, r Rollout, message string, before ...Event) error {
	now := e.now()
	return e.save(ctx, r, func(x *Rollout) {
		x.Phase = PhasePromoting
		x.StepStartedAt = nil
		x.WaitingSince = &now
	}, EventPromoting, message, before...)
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
