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
	EventCreated = "Created"
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
