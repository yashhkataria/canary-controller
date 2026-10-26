package rollout

import (
	"context"
	"fmt"
	"time"
)

func (FixedWait) Analyze(_ context.Context, r Rollout, now time.Time) (Verdict, string, error) {
	if r.StepStartedAt == nil {
		return VerdictInconclusive, "step start time not recorded yet", nil
	}
	wait := r.Analysis.Interval.Std() * time.Duration(r.Analysis.ChecksPerStep)
	waited := now.Sub(*r.StepStartedAt)
	if waited < wait {
		return VerdictInconclusive, fmt.Sprintf("waited %s of %s", waited.Round(time.Second), wait), nil
	}
	return VerdictPass, fmt.Sprintf("fixed wait of %s elapsed", wait), nil
}
