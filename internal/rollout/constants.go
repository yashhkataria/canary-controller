package rollout

import "time"

const (
	PhasePending     Phase = "Pending"
	PhaseProgressing Phase = "Progressing"
	PhaseAnalyzing   Phase = "Analyzing"
	PhasePromoting   Phase = "Promoting"
	PhaseSucceeded   Phase = "Succeeded"
	PhaseRollingBack Phase = "RollingBack"
	PhaseRolledBack  Phase = "RolledBack"
)

const (
	EventCreated     = "Created"
	EventStarted     = "Started"
	EventStepReady   = "StepReady"
	EventStepPassed  = "StepPassed"
	EventPromoting   = "Promoting"
	EventSucceeded   = "Succeeded"
	EventRollingBack = "RollingBack"
	EventRolledBack  = "RolledBack"
)

const (
	VerdictPass         Verdict = "pass"
	VerdictFail         Verdict = "fail"
	VerdictInconclusive Verdict = "inconclusive"
)

const (
	DefaultEngineInterval = 2 * time.Second
	reconcileTimeout      = 10 * time.Second
)

const TotalReplicas = 10

const (
	DefaultInterval      = 20 * time.Second
	DefaultChecksPerStep = 3
	DefaultFailureLimit  = 1
	DefaultMaxErrorRate  = 0.02
	DefaultErrorMargin   = 0.01
	DefaultMaxP99Ms      = 500
	DefaultLatencyFactor = 1.5
	DefaultMinRequests   = 50
	DefaultMaxWait       = 5 * time.Minute
)
