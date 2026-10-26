package rollout

import (
	"context"
	"errors"
	"fmt"

	"github.com/yashhkataria/canary-controller/internal/k8s"
)

func (e *Engine) Reconcile(ctx context.Context, r Rollout) error {
	switch r.Phase {
	case PhasePending:
		return e.start(ctx, r)
	case PhaseProgressing:
		return e.progress(ctx, r)
	case PhaseAnalyzing:
		return e.analyze(ctx, r)
	case PhasePromoting:
		return e.promote(ctx, r)
	case PhaseRollingBack:
		return e.rollBack(ctx, r)
	default:
		return e.save(ctx, r, func(x *Rollout) { x.Active = false }, "", "")
	}
}

func (e *Engine) start(ctx context.Context, r Rollout) error {
	stable, _, err := e.statuses(ctx, r)
	if errors.Is(err, k8s.ErrNotFound) || errors.Is(err, k8s.ErrForbidden) {
		reason := "cannot start: " + err.Error()
		return e.finish(ctx, r, PhaseRolledBack, reason, EventRolledBack, reason)
	}
	if err != nil {
		return err
	}
	now := e.now()
	return e.save(ctx, r, func(x *Rollout) {
		x.StableImage = stable.Image
		x.Phase = PhaseProgressing
		x.CurrentStep = 0
		x.Owner = e.owner
		x.WaitingSince = &now
		x.StepStartedAt = nil
	}, EventStarted, fmt.Sprintf("replacing %s with %s in steps %v", stable.Image, r.CanaryImage, r.Steps))
}

func (e *Engine) progress(ctx context.Context, r Rollout) error {
	if r.CurrentStep >= len(r.Steps) {
		return e.beginPromotion(ctx, r, "no steps left")
	}
	if e.timedOut(r) {
		return e.beginRollback(ctx, r, fmt.Sprintf("step %d (%d%%) not ready after %s", r.CurrentStep+1, r.Steps[r.CurrentStep], r.Analysis.MaxWait))
	}
	want := e.stepTarget(r)
	done, err := e.converge(ctx, r, want)
	if err != nil || !done {
		return err
	}
	now := e.now()
	return e.save(ctx, r, func(x *Rollout) {
		x.Phase = PhaseAnalyzing
		x.StepStartedAt = &now
		x.WaitingSince = nil
	}, EventStepReady, fmt.Sprintf("step %d of %d ready: %d%% canary (%d canary, %d stable pods)",
		r.CurrentStep+1, len(r.Steps), r.Steps[r.CurrentStep], want.canary, want.stable))
}

func (e *Engine) analyze(ctx context.Context, r Rollout) error {
	if r.CurrentStep >= len(r.Steps) {
		return e.beginPromotion(ctx, r, "no steps left")
	}
	done, err := e.converge(ctx, r, e.stepTarget(r))
	if err != nil || !done {
		return err
	}

	verdict, detail, err := e.analyzer.Analyze(ctx, r, e.now())
	if err != nil {
		return fmt.Errorf("analyze: %w", err)
	}
	switch verdict {
	case VerdictPass:
		if r.CurrentStep == len(r.Steps)-1 {
			return e.beginPromotion(ctx, r, fmt.Sprintf("all %d steps passed (%s)", len(r.Steps), detail))
		}
		now := e.now()
		next := r.CurrentStep + 1
		return e.save(ctx, r, func(x *Rollout) {
			x.CurrentStep = next
			x.Phase = PhaseProgressing
			x.StepStartedAt = nil
			x.WaitingSince = &now
		}, EventStepPassed, fmt.Sprintf("step %d (%d%%) passed: %s; moving to %d%%",
			r.CurrentStep+1, r.Steps[r.CurrentStep], detail, r.Steps[next]))
	case VerdictFail:
		return e.beginRollback(ctx, r, detail)
	default:
		return nil
	}
}

func (e *Engine) promote(ctx context.Context, r Rollout) error {
	if e.timedOut(r) {
		return e.beginRollback(ctx, r, fmt.Sprintf("promotion not ready after %s", r.Analysis.MaxWait))
	}
	done, err := e.converge(ctx, r, desiredState{stableImage: r.CanaryImage, stable: TotalReplicas, canary: 0})
	if err != nil || !done {
		return err
	}
	return e.finish(ctx, r, PhaseSucceeded, "", EventSucceeded,
		fmt.Sprintf("%s now runs %s on %d pods; canary scaled to 0", k8s.StableName(r.App), r.CanaryImage, TotalReplicas))
}

func (e *Engine) rollBack(ctx context.Context, r Rollout) error {
	done, err := e.converge(ctx, r, desiredState{stableImage: r.StableImage, stable: TotalReplicas, canary: 0})
	if err != nil || !done {
		return err
	}
	image := r.StableImage
	if image == "" {
		image = "its current image"
	}
	return e.finish(ctx, r, PhaseRolledBack, r.Reason, EventRolledBack,
		fmt.Sprintf("%s back on %s with %d pods; canary scaled to 0", k8s.StableName(r.App), image, TotalReplicas))
}

func (e *Engine) converge(ctx context.Context, r Rollout, want desiredState) (bool, error) {
	stable, canary, err := e.statuses(ctx, r)
	if err != nil {
		return false, err
	}
	ns, stableName, canaryName := r.Namespace, k8s.StableName(r.App), k8s.CanaryName(r.App)

	switch {
	case want.canaryImage != "" && canary.Image != want.canaryImage:
		return false, e.cluster.SetImage(ctx, ns, canaryName, canary.Container, want.canaryImage)
	case want.stableImage != "" && stable.Image != want.stableImage:
		return false, e.cluster.SetImage(ctx, ns, stableName, stable.Container, want.stableImage)
	case canary.Desired < want.canary:
		return false, e.cluster.SetReplicas(ctx, ns, canaryName, want.canary)
	case stable.Desired < want.stable:
		return false, e.cluster.SetReplicas(ctx, ns, stableName, want.stable)
	case stable.Desired > want.stable:
		if !canary.ReadyAt(want.canary) {
			return false, nil
		}
		return false, e.cluster.SetReplicas(ctx, ns, stableName, want.stable)
	case canary.Desired > want.canary:
		if !stable.ReadyAt(want.stable) {
			return false, nil
		}
		return false, e.cluster.SetReplicas(ctx, ns, canaryName, want.canary)
	}
	return stable.ReadyAt(want.stable) && canary.ReadyAt(want.canary), nil
}

func (e *Engine) statuses(ctx context.Context, r Rollout) (stable, canary k8s.DeploymentStatus, err error) {
	stable, err = e.cluster.Status(ctx, r.Namespace, k8s.StableName(r.App), r.App)
	if err != nil {
		return stable, canary, err
	}
	canary, err = e.cluster.Status(ctx, r.Namespace, k8s.CanaryName(r.App), r.App)
	return stable, canary, err
}

func (e *Engine) stepTarget(r Rollout) desiredState {
	canary, stable := Replicas(r.Steps[r.CurrentStep])
	return desiredState{canaryImage: r.CanaryImage, canary: canary, stable: stable}
}

func (e *Engine) timedOut(r Rollout) bool {
	return r.WaitingSince != nil && e.now().Sub(*r.WaitingSince) > r.Analysis.MaxWait.Std()
}
