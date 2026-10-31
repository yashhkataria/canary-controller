package rollout

import (
	"context"
	"fmt"
)

func (e *Engine) analyze(ctx context.Context, r Rollout) error {
	if r.CurrentStep >= len(r.Steps) {
		return e.beginPromotion(ctx, r, "no steps left")
	}
	done, err := e.converge(ctx, r, e.stepTarget(r))
	if err != nil || !done {
		return err
	}

	now := e.now()
	if !e.checkDue(r) {
		return nil
	}
	check, err := e.analyzer.Check(ctx, r, now)
	if err != nil {
		check = LastCheck{At: now, Verdict: VerdictInconclusive, Reason: "analysis failed: " + err.Error()}
	}
	check.At = now

	sc := r.StepChecks
	sc.LastAt = &now
	switch check.Verdict {
	case VerdictPass:
		sc.Passes++
		sc.InconclusiveSince = nil
	case VerdictFail:
		sc.Failures++
		sc.Passes = 0
		sc.InconclusiveSince = nil
	default:
		check.Verdict = VerdictInconclusive
		sc.Passes = 0
		if sc.InconclusiveSince == nil {
			sc.InconclusiveSince = &now
		}
	}
	r.LastCheck = &check
	r.StepChecks = sc
	event := checkEvent(r, check, sc)
	step, weight := r.CurrentStep+1, r.Steps[r.CurrentStep]

	switch {
	case sc.Failures >= r.Analysis.FailureLimit:
		return e.beginRollback(ctx, r, fmt.Sprintf("step %d (%d%%) failed: %s", step, weight, check.Reason), event)
	case check.Verdict == VerdictInconclusive && now.Sub(*sc.InconclusiveSince) > r.Analysis.MaxWait.Std():
		return e.beginRollback(ctx, r, fmt.Sprintf("step %d (%d%%) inconclusive for more than %s: %s",
			step, weight, r.Analysis.MaxWait, check.Reason), event)
	case sc.Passes >= r.Analysis.ChecksPerStep && r.CurrentStep == len(r.Steps)-1:
		return e.beginPromotion(ctx, r, fmt.Sprintf("all %d steps passed their checks", len(r.Steps)), event)
	case sc.Passes >= r.Analysis.ChecksPerStep:
		next := r.CurrentStep + 1
		return e.save(ctx, r, func(x *Rollout) {
			x.CurrentStep = next
			x.Phase = PhaseProgressing
			x.StepStartedAt = nil
			x.WaitingSince = &now
			x.StepChecks = StepChecks{}
		}, EventStepPassed, fmt.Sprintf("step %d (%d%%) passed %d checks in a row; moving to %d%%",
			step, weight, sc.Passes, r.Steps[next]), event)
	default:
		return e.save(ctx, r, func(*Rollout) {}, "", "", event)
	}
}

func (e *Engine) checkDue(r Rollout) bool {
	last := r.StepChecks.LastAt
	if last == nil {
		last = r.StepStartedAt
	}
	return last == nil || e.now().Sub(*last) >= r.Analysis.Interval.Std()
}

func checkEvent(r Rollout, c LastCheck, sc StepChecks) Event {
	var progress string
	switch c.Verdict {
	case VerdictPass:
		progress = fmt.Sprintf("pass %d/%d", sc.Passes, r.Analysis.ChecksPerStep)
	case VerdictFail:
		progress = fmt.Sprintf("fail %d/%d", sc.Failures, r.Analysis.FailureLimit)
	default:
		progress = fmt.Sprintf("inconclusive for %s", c.At.Sub(*sc.InconclusiveSince))
	}
	return Event{
		At:      c.At,
		Type:    EventCheck,
		Message: fmt.Sprintf("step %d (%d%%) %s: %s", r.CurrentStep+1, r.Steps[r.CurrentStep], progress, c.Reason),
		Metrics: map[string]float64{
			"canary_error_rate": c.Canary.ErrorRate,
			"canary_p99_ms":     c.Canary.P99Ms,
			"canary_requests":   c.Canary.Requests,
			"stable_error_rate": c.Stable.ErrorRate,
			"stable_p99_ms":     c.Stable.P99Ms,
			"stable_requests":   c.Stable.Requests,
		},
	}
}
