package rollout

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type Phase string

type Duration time.Duration

type Analysis struct {
	Interval      Duration `json:"interval" bson:"interval"`
	ChecksPerStep int      `json:"checksPerStep" bson:"checksPerStep"`
	FailureLimit  int      `json:"failureLimit" bson:"failureLimit"`
	MaxErrorRate  float64  `json:"maxErrorRate" bson:"maxErrorRate"`
	ErrorMargin   float64  `json:"errorMargin" bson:"errorMargin"`
	MaxP99Ms      float64  `json:"maxP99Ms" bson:"maxP99Ms"`
	LatencyFactor float64  `json:"latencyFactor" bson:"latencyFactor"`
	MinRequests   int      `json:"minRequests" bson:"minRequests"`
	MaxWait       Duration `json:"maxWait" bson:"maxWait"`
}

type TrackMetrics struct {
	ErrorRate float64 `json:"errorRate" bson:"errorRate"`
	P99Ms     float64 `json:"p99Ms" bson:"p99Ms"`
	Requests  float64 `json:"requests" bson:"requests"`
}

type LastCheck struct {
	At      time.Time    `json:"at" bson:"at"`
	Stable  TrackMetrics `json:"stable" bson:"stable"`
	Canary  TrackMetrics `json:"canary" bson:"canary"`
	Verdict string       `json:"verdict" bson:"verdict"`
}

type Rollout struct {
	ID          bson.ObjectID `json:"id" bson:"_id,omitempty"`
	Namespace   string        `json:"namespace" bson:"namespace"`
	App         string        `json:"app" bson:"app"`
	StableImage string        `json:"stableImage" bson:"stableImage"`
	CanaryImage string        `json:"canaryImage" bson:"canaryImage"`
	Steps       []int         `json:"steps" bson:"steps"`
	CurrentStep int           `json:"currentStep" bson:"currentStep"`
	Phase       Phase         `json:"phase" bson:"phase"`
	Analysis    Analysis      `json:"analysis" bson:"analysis"`
	LastCheck   *LastCheck    `json:"lastCheck,omitempty" bson:"lastCheck,omitempty"`
	Active      bool          `json:"active" bson:"active"`
	Owner       string        `json:"owner,omitempty" bson:"owner,omitempty"`
	Version     int64         `json:"version" bson:"version"`
	Reason      string        `json:"reason,omitempty" bson:"reason,omitempty"`
	CreatedAt   time.Time     `json:"createdAt" bson:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt" bson:"updatedAt"`
	FinishedAt  *time.Time    `json:"finishedAt,omitempty" bson:"finishedAt,omitempty"`
}

type Event struct {
	ID        bson.ObjectID      `json:"id" bson:"_id,omitempty"`
	RolloutID bson.ObjectID      `json:"rolloutId" bson:"rolloutId"`
	At        time.Time          `json:"at" bson:"at"`
	Type      string             `json:"type" bson:"type"`
	Message   string             `json:"message" bson:"message"`
	Metrics   map[string]float64 `json:"metrics,omitempty" bson:"metrics,omitempty"`
}
