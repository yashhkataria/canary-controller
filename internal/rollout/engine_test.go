package rollout

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"

	"github.com/yashhkataria/canary-controller/internal/k8s"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestReplicas(t *testing.T) {
	tests := []struct {
		weight         int
		canary, stable int32
	}{
		{0, 0, 10}, {10, 1, 9}, {20, 2, 8}, {30, 3, 7}, {40, 4, 6}, {50, 5, 5},
		{60, 6, 4}, {70, 7, 3}, {80, 8, 2}, {90, 9, 1}, {100, 10, 0},
		{15, 2, 8}, {14, 1, 9}, {-10, 0, 10}, {150, 10, 0},
	}
	for _, tt := range tests {
		c, s := Replicas(tt.weight)
		if c != tt.canary || s != tt.stable {
			t.Errorf("Replicas(%d) = (%d, %d), want (%d, %d)", tt.weight, c, s, tt.canary, tt.stable)
		}
		if c+s != TotalReplicas {
			t.Errorf("Replicas(%d) total = %d, want %d", tt.weight, c+s, TotalReplicas)
		}
	}
}

func TestStartRecordsStableImage(t *testing.T) {
	h := newHarness(t, []int{10, 50, 100})
	r := h.tick()

	if r.Phase != PhaseProgressing || r.StableImage != "demo:v1" || r.Owner != "controller-0" || r.CurrentStep != 0 {
		t.Fatalf("after start: phase=%s stableImage=%q owner=%q step=%d", r.Phase, r.StableImage, r.Owner, r.CurrentStep)
	}
	if r.WaitingSince == nil || !r.WaitingSince.Equal(h.now) {
		t.Errorf("waitingSince = %v, want %v", r.WaitingSince, h.now)
	}
	if got := h.store.eventTypes(); !slices.Equal(got, []string{EventStarted}) {
		t.Errorf("events = %v", got)
	}
	if h.patchCount() != 0 {
		t.Errorf("start must not touch the cluster, made %d patches", h.patchCount())
	}
}

func TestStepGrowsCanaryBeforeShrinkingStable(t *testing.T) {
	h := newHarness(t, []int{10, 50, 100})
	h.tick()

	h.tick()
	if _, img := h.deployment("demo-canary"); img != "demo:v2" {
		t.Fatalf("first action should set the canary image, canary image = %s", img)
	}
	if got := h.rawDeployment("demo-canary").Spec.Template.Labels[k8s.RolloutLabel]; got != h.id.Hex() {
		t.Fatalf("canary pods labelled rollout-id=%q, want %s", got, h.id.Hex())
	}

	h.tick()
	if c, _ := h.deployment("demo-canary"); c != 1 {
		t.Fatalf("canary replicas = %d, want 1", c)
	}
	if s, _ := h.deployment("demo-stable"); s != 10 {
		t.Fatalf("stable shrank to %d before the canary was ready", s)
	}

	before := h.patchCount()
	h.tick()
	if s, _ := h.deployment("demo-stable"); s != 10 || h.patchCount() != before {
		t.Fatalf("engine acted while the canary was not ready: stable=%d patches=%d", s, h.patchCount()-before)
	}

	h.markReady()
	h.tick()
	if s, _ := h.deployment("demo-stable"); s != 9 {
		t.Fatalf("stable replicas = %d, want 9 once the canary is ready", s)
	}
	if h.rollout().Phase != PhaseProgressing {
		t.Fatalf("moved on before stable was ready: %s", h.rollout().Phase)
	}

	h.markReady()
	r := h.tick()
	if r.Phase != PhaseAnalyzing || r.StepStartedAt == nil || r.WaitingSince != nil {
		t.Fatalf("after step ready: phase=%s stepStartedAt=%v waitingSince=%v", r.Phase, r.StepStartedAt, r.WaitingSince)
	}
}

func TestFullRolloutSucceeds(t *testing.T) {
	h := newHarness(t, []int{10, 50, 100})
	r := h.runUntil(PhaseSucceeded, 80)

	if r.Active || r.FinishedAt == nil || r.Reason != "" {
		t.Errorf("finished rollout: active=%v finishedAt=%v reason=%q", r.Active, r.FinishedAt, r.Reason)
	}
	if n, img := h.deployment("demo-stable"); n != 10 || img != "demo:v2" {
		t.Errorf("stable = %d pods on %s, want 10 on demo:v2", n, img)
	}
	if n, _ := h.deployment("demo-canary"); n != 0 {
		t.Errorf("canary = %d pods, want 0", n)
	}
	want := []string{EventStarted, EventStepReady, EventStepPassed, EventStepReady, EventStepPassed, EventStepReady, EventPromoting, EventSucceeded}
	if got := h.store.phaseEventTypes(); !slices.Equal(got, want) {
		t.Errorf("phase events\n got  %v\n want %v", got, want)
	}
	if n := len(h.store.eventsOf(EventCheck)); n != 6 {
		t.Errorf("check events = %d, want 6 (2 per step x 3 steps)", n)
	}
}

func TestCanaryRelabelledForEachRollout(t *testing.T) {
	h := newHarness(t, []int{10})
	d := h.rawDeployment("demo-canary").DeepCopy()
	d.Spec.Template.Spec.Containers[0].Image = "demo:v2"
	d.Spec.Template.Labels = map[string]string{k8s.RolloutLabel: "an-older-rollout"}
	if err := h.cs.Tracker().Update(deploymentsGVR, d, "default"); err != nil {
		t.Fatalf("seed canary: %v", err)
	}
	h.tick()
	before := h.patchCount()
	h.tick()
	if h.patchCount() != before+1 {
		t.Fatalf("same image but another rollout's label: want one relabel patch, got %d", h.patchCount()-before)
	}
	if got := h.rawDeployment("demo-canary").Spec.Template.Labels[k8s.RolloutLabel]; got != h.id.Hex() {
		t.Errorf("rollout-id = %q, want %s", got, h.id.Hex())
	}
}

func TestShortStepsPromoteFromPartialWeight(t *testing.T) {
	h := newHarness(t, []int{10, 50})
	h.runUntil(PhaseSucceeded, 80)
	if n, img := h.deployment("demo-stable"); n != 10 || img != "demo:v2" {
		t.Errorf("stable = %d pods on %s, want 10 on demo:v2", n, img)
	}
}

func TestReconcileTwiceIsHarmless(t *testing.T) {
	tests := []struct {
		name  string
		setup func(h *harness)
	}{
		{name: "waiting for canary pods", setup: func(h *harness) { h.tick(); h.tick(); h.tick() }},
		{name: "step ready, analysis waiting", setup: func(h *harness) { h.runUntil(PhaseAnalyzing, 20) }},
		{name: "rolling back, waiting for stable pods", setup: func(h *harness) {
			h.runUntil(PhaseAnalyzing, 20)
			h.setPhase(PhaseRollingBack)
			h.tick()
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, []int{10, 50, 100})
			tt.setup(h)
			before, version, events := h.patchCount(), h.rollout().Version, len(h.store.eventTypes())

			h.tick()
			h.tick()

			if h.patchCount() != before {
				t.Errorf("repeated reconcile made %d extra cluster changes", h.patchCount()-before)
			}
			if h.rollout().Version != version || len(h.store.eventTypes()) != events {
				t.Errorf("repeated reconcile wrote to the store: version %d -> %d, events %d -> %d",
					version, h.rollout().Version, events, len(h.store.eventTypes()))
			}
		})
	}
}

func TestAbortMidStepRestoresCapacityFirst(t *testing.T) {
	h := newHarness(t, []int{10, 50, 100})
	h.runUntilStep(1, PhaseAnalyzing, 40)
	if c, _ := h.deployment("demo-canary"); c != 5 {
		t.Fatalf("setup: canary = %d, want 5", c)
	}

	h.setPhase(PhaseRollingBack)
	h.tick()
	if s, _ := h.deployment("demo-stable"); s != 10 {
		t.Fatalf("stable = %d, want 10 first", s)
	}
	if c, _ := h.deployment("demo-canary"); c != 5 {
		t.Fatalf("canary scaled down to %d before stable was ready", c)
	}

	r := h.runUntil(PhaseRolledBack, 20)
	if r.Active {
		t.Error("rolled-back rollout still active")
	}
	if n, img := h.deployment("demo-stable"); n != 10 || img != "demo:v1" {
		t.Errorf("stable = %d on %s, want 10 on demo:v1", n, img)
	}
	if c, _ := h.deployment("demo-canary"); c != 0 {
		t.Errorf("canary = %d, want 0", c)
	}
}

func TestAbortDuringPromotionRestoresStableImage(t *testing.T) {
	h := newHarness(t, []int{100})
	h.runUntil(PhasePromoting, 40)
	h.tick()
	if _, img := h.deployment("demo-stable"); img != "demo:v2" {
		t.Fatalf("setup: stable image = %s, want demo:v2 after promotion started", img)
	}

	h.setPhase(PhaseRollingBack)
	h.runUntil(PhaseRolledBack, 30)
	if n, img := h.deployment("demo-stable"); n != 10 || img != "demo:v1" {
		t.Errorf("stable = %d on %s, want 10 on demo:v1", n, img)
	}
}

func TestStaleRolloutIsRejected(t *testing.T) {
	h := newHarness(t, []int{10, 50, 100})
	stale := h.rollout()
	h.setPhase(PhaseRollingBack)
	version := h.rollout().Version

	err := h.engine.Reconcile(context.Background(), stale)
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("err = %v, want ErrVersionConflict", err)
	}
	if r := h.rollout(); r.Phase != PhaseRollingBack || r.Version != version {
		t.Errorf("stale write landed: phase=%s version=%d", r.Phase, r.Version)
	}
	if len(h.store.eventTypes()) != 0 {
		t.Errorf("stale reconcile wrote events: %v", h.store.eventTypes())
	}
}

func TestMissingDeploymentEndsRollout(t *testing.T) {
	h := newHarness(t, []int{10}, []runtime.Object{testDeployment("demo-stable", 10, "demo:v1")}...)
	r := h.tick()
	if r.Phase != PhaseRolledBack || r.Active || !strings.Contains(r.Reason, "not found") {
		t.Errorf("phase=%s active=%v reason=%q; want RolledBack, inactive, a not-found reason", r.Phase, r.Active, r.Reason)
	}
	if h.patchCount() != 0 {
		t.Errorf("made %d patches for a rollout that could not start", h.patchCount())
	}
}

func TestPodsNeverReadyRollsBack(t *testing.T) {
	h := newHarness(t, []int{10, 50, 100})
	h.tick()
	h.tick()
	h.tick()

	h.now = h.now.Add(5*time.Minute + time.Second)
	r := h.tick()
	if r.Phase != PhaseRollingBack || !strings.Contains(r.Reason, "not ready after") {
		t.Fatalf("phase=%s reason=%q; want RollingBack with a not-ready reason", r.Phase, r.Reason)
	}
	h.runUntil(PhaseRolledBack, 20)
}

func TestDriftDuringAnalysisIsCorrected(t *testing.T) {
	h := newHarness(t, []int{10, 50, 100})
	h.runUntil(PhaseAnalyzing, 20)

	d := h.rawDeployment("demo-canary").DeepCopy()
	three := int32(3)
	d.Spec.Replicas = &three
	if err := h.cs.Tracker().Update(appsv1.SchemeGroupVersion.WithResource("deployments"), d, "default"); err != nil {
		t.Fatalf("simulate drift: %v", err)
	}
	h.markReady()

	r := h.tick()
	if c, _ := h.deployment("demo-canary"); c != 1 {
		t.Errorf("canary = %d after drift, want it put back to 1", c)
	}
	if r.Phase != PhaseAnalyzing {
		t.Errorf("drift changed the phase to %s", r.Phase)
	}
}

func TestTerminalButActiveIsDeactivated(t *testing.T) {
	h := newHarness(t, []int{10})
	h.setPhase(PhaseSucceeded)
	if r := h.tick(); r.Active {
		t.Error("terminal rollout left active")
	}
}
