package rollout

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestChecksRunEveryInterval(t *testing.T) {
	h := newHarness(t, []int{10, 50, 100})
	h.runUntil(PhaseAnalyzing, 20)
	calls := h.analyzer.callCount()

	steps := []struct {
		offset    time.Duration
		wantCalls int
		wantPhase Phase
	}{
		{offset: 0, wantCalls: 0, wantPhase: PhaseAnalyzing},
		{offset: 9 * time.Second, wantCalls: 0, wantPhase: PhaseAnalyzing},
		{offset: 10 * time.Second, wantCalls: 1, wantPhase: PhaseAnalyzing},
		{offset: 15 * time.Second, wantCalls: 1, wantPhase: PhaseAnalyzing},
		{offset: 20 * time.Second, wantCalls: 2, wantPhase: PhaseProgressing},
	}
	for _, s := range steps {
		r := h.tickAt(s.offset)
		if got := h.analyzer.callCount() - calls; got != s.wantCalls || r.Phase != s.wantPhase {
			t.Fatalf("at +%s: %d checks and phase %s, want %d and %s", s.offset, got, r.Phase, s.wantCalls, s.wantPhase)
		}
	}
	if r := h.rollout(); r.CurrentStep != 1 || r.StepChecks != (StepChecks{}) {
		t.Errorf("after passing: step=%d stepChecks=%+v, want step 1 with counters reset", r.CurrentStep, r.StepChecks)
	}
	if last := h.store.eventTypes(); last[len(last)-1] != EventStepPassed || last[len(last)-2] != EventCheck {
		t.Errorf("last events = %v, want ... Check, StepPassed", last[len(last)-2:])
	}
}

func TestPassesMustBeInARow(t *testing.T) {
	tests := []struct {
		name     string
		verdicts []Verdict
		advances int
	}{
		{name: "two passes", verdicts: []Verdict{VerdictPass, VerdictPass}, advances: 2},
		{name: "inconclusive resets the streak", verdicts: []Verdict{VerdictPass, VerdictInconclusive, VerdictPass, VerdictPass}, advances: 4},
		{name: "a failure resets the streak", verdicts: []Verdict{VerdictPass, VerdictFail, VerdictPass, VerdictPass}, advances: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, []int{10, 50, 100})
			h.setAnalysis(func(a *Analysis) { a.FailureLimit = 2 })
			h.runUntil(PhaseAnalyzing, 20)
			h.analyzer.verdicts = tt.verdicts

			for i := 1; i <= len(tt.verdicts); i++ {
				r := h.tickAt(time.Duration(i) * 10 * time.Second)
				advanced := r.Phase == PhaseProgressing && r.CurrentStep == 1
				if advanced != (i == tt.advances) {
					t.Fatalf("after check %d: phase=%s step=%d passes=%d; want advance only after check %d",
						i, r.Phase, r.CurrentStep, r.StepChecks.Passes, tt.advances)
				}
			}
		})
	}
}

func TestFailuresRollBack(t *testing.T) {
	tests := []struct {
		name         string
		failureLimit int
		verdicts     []Verdict
		rollbackAt   int
	}{
		{name: "first failure with failureLimit 1", failureLimit: 1, verdicts: []Verdict{VerdictFail}, rollbackAt: 1},
		{name: "second failure with failureLimit 2", failureLimit: 2, verdicts: []Verdict{VerdictFail, VerdictPass, VerdictFail}, rollbackAt: 3},
		{name: "failures need not be in a row", failureLimit: 2, verdicts: []Verdict{VerdictFail, VerdictInconclusive, VerdictFail}, rollbackAt: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t, []int{10, 50, 100})
			h.setAnalysis(func(a *Analysis) { a.FailureLimit = tt.failureLimit; a.ChecksPerStep = 3 })
			h.runUntil(PhaseAnalyzing, 20)
			h.analyzer.verdicts = tt.verdicts

			for i := 1; i <= len(tt.verdicts); i++ {
				r := h.tickAt(time.Duration(i) * 10 * time.Second)
				if (r.Phase == PhaseRollingBack) != (i == tt.rollbackAt) {
					t.Fatalf("after check %d: phase=%s failures=%d; want rollback only after check %d",
						i, r.Phase, r.StepChecks.Failures, tt.rollbackAt)
				}
			}
			r := h.rollout()
			if !strings.Contains(r.Reason, "step 1 (10%) failed: scripted fail") {
				t.Errorf("reason = %q", r.Reason)
			}
			if r.LastCheck == nil || r.LastCheck.Verdict != VerdictFail {
				t.Errorf("lastCheck = %+v, want the failing check", r.LastCheck)
			}
			h.runUntil(PhaseRolledBack, 20)
		})
	}
}

func TestInconclusiveTooLongRollsBack(t *testing.T) {
	h := newHarness(t, []int{10, 50, 100})
	h.setAnalysis(func(a *Analysis) { a.MaxWait = Duration(time.Minute) })
	h.runUntil(PhaseAnalyzing, 20)
	h.analyzer.fallback = VerdictInconclusive

	r := h.tickAt(10 * time.Second)
	if r.StepChecks.InconclusiveSince == nil || r.Phase != PhaseAnalyzing {
		t.Fatalf("first inconclusive: phase=%s since=%v", r.Phase, r.StepChecks.InconclusiveSince)
	}
	since := *r.StepChecks.InconclusiveSince

	for offset := 20 * time.Second; offset <= 70*time.Second; offset += 10 * time.Second {
		r = h.tickAt(offset)
		if r.Phase != PhaseAnalyzing {
			t.Fatalf("rolled back at +%s, only %s after the first inconclusive", offset, h.now.Sub(since))
		}
	}
	r = h.tickAt(80 * time.Second)
	if r.Phase != PhaseRollingBack || !strings.Contains(r.Reason, "inconclusive for more than 1m0s: scripted inconclusive") {
		t.Fatalf("phase=%s reason=%q; want RollingBack for being inconclusive too long", r.Phase, r.Reason)
	}
}

func TestPassClearsInconclusiveTimer(t *testing.T) {
	h := newHarness(t, []int{10, 50, 100})
	h.setAnalysis(func(a *Analysis) { a.MaxWait = Duration(time.Minute); a.ChecksPerStep = 100 })
	h.runUntil(PhaseAnalyzing, 20)
	h.analyzer.verdicts = []Verdict{VerdictInconclusive, VerdictInconclusive, VerdictInconclusive, VerdictPass}
	h.analyzer.fallback = VerdictInconclusive

	for offset := 10 * time.Second; offset <= 110*time.Second; offset += 10 * time.Second {
		if r := h.tickAt(offset); r.Phase != PhaseAnalyzing {
			t.Fatalf("at +%s phase=%s reason=%q; the pass at +40s should have restarted the timer", offset, r.Phase, r.Reason)
		}
	}
	if r := h.tickAt(120 * time.Second); r.Phase != PhaseRollingBack {
		t.Fatalf("at +2m phase=%s; 70 s of inconclusive since +50s should exceed the 1m limit", r.Phase)
	}
}

func TestAnalyzerErrorCountsAsInconclusive(t *testing.T) {
	h := newHarness(t, []int{10, 50, 100})
	h.runUntil(PhaseAnalyzing, 20)
	h.analyzer.err = errors.New("prometheus down")

	r := h.tickAt(10 * time.Second)
	if r.Phase != PhaseAnalyzing || r.LastCheck == nil || r.LastCheck.Verdict != VerdictInconclusive ||
		r.LastCheck.Reason != "analysis failed: prometheus down" || r.StepChecks.InconclusiveSince == nil {
		t.Errorf("phase=%s lastCheck=%+v stepChecks=%+v", r.Phase, r.LastCheck, r.StepChecks)
	}
}

func TestEachCheckIsStored(t *testing.T) {
	h := newHarness(t, []int{10, 50, 100})
	h.runUntil(PhaseAnalyzing, 20)
	before := h.rollout().Version

	r := h.tickAt(10 * time.Second)
	if r.Version != before+1 {
		t.Errorf("version %d -> %d, want one write per check", before, r.Version)
	}
	if r.LastCheck == nil || r.LastCheck.Verdict != VerdictPass || !r.LastCheck.At.Equal(h.now) || r.StepChecks.Passes != 1 {
		t.Fatalf("lastCheck=%+v stepChecks=%+v", r.LastCheck, r.StepChecks)
	}
	checks := h.store.eventsOf(EventCheck)
	if len(checks) != 1 {
		t.Fatalf("check events = %d, want 1", len(checks))
	}
	e := checks[0]
	if !strings.Contains(e.Message, "step 1 (10%) pass 1/2: scripted pass") || e.Metrics["canary_requests"] != 100 || e.Metrics["stable_p99_ms"] != 5 {
		t.Errorf("check event = %q metrics=%v", e.Message, e.Metrics)
	}
}

func TestNoCheckWhilePodsDrift(t *testing.T) {
	h := newHarness(t, []int{10, 50, 100})
	h.runUntil(PhaseAnalyzing, 20)
	h.setPhase(PhaseAnalyzing)
	calls := h.analyzer.callCount()

	d := h.rawDeployment("demo-canary").DeepCopy()
	three := int32(3)
	d.Spec.Replicas = &three
	if err := h.cs.Tracker().Update(deploymentsGVR, d, "default"); err != nil {
		t.Fatalf("simulate drift: %v", err)
	}
	h.tickAt(10 * time.Second)
	if h.analyzer.callCount() != calls {
		t.Error("checked while the step's pods were not at their target")
	}
}
