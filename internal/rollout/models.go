package rollout

import (
	"context"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/yashhkataria/canary-controller/internal/k8s"
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
	Verdict Verdict      `json:"verdict" bson:"verdict"`
	Reason  string       `json:"reason" bson:"reason"`
}

type StepChecks struct {
	Passes            int        `json:"passes" bson:"passes"`
	Failures          int        `json:"failures" bson:"failures"`
	InconclusiveSince *time.Time `json:"inconclusiveSince,omitempty" bson:"inconclusiveSince,omitempty"`
	LastAt            *time.Time `json:"lastAt,omitempty" bson:"lastAt,omitempty"`
}

type Rollout struct {
	ID            bson.ObjectID `json:"id" bson:"_id,omitempty"`
	Namespace     string        `json:"namespace" bson:"namespace"`
	App           string        `json:"app" bson:"app"`
	StableImage   string        `json:"stableImage" bson:"stableImage"`
	CanaryImage   string        `json:"canaryImage" bson:"canaryImage"`
	Steps         []int         `json:"steps" bson:"steps"`
	CurrentStep   int           `json:"currentStep" bson:"currentStep"`
	Phase         Phase         `json:"phase" bson:"phase"`
	Analysis      Analysis      `json:"analysis" bson:"analysis"`
	LastCheck     *LastCheck    `json:"lastCheck,omitempty" bson:"lastCheck,omitempty"`
	StepChecks    StepChecks    `json:"stepChecks" bson:"stepChecks"`
	Active        bool          `json:"active" bson:"active"`
	Owner         string        `json:"owner,omitempty" bson:"owner,omitempty"`
	Version       int64         `json:"version" bson:"version"`
	Reason        string        `json:"reason,omitempty" bson:"reason,omitempty"`
	StepStartedAt *time.Time    `json:"stepStartedAt,omitempty" bson:"stepStartedAt,omitempty"`
	WaitingSince  *time.Time    `json:"waitingSince,omitempty" bson:"waitingSince,omitempty"`
	CreatedAt     time.Time     `json:"createdAt" bson:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt" bson:"updatedAt"`
	FinishedAt    *time.Time    `json:"finishedAt,omitempty" bson:"finishedAt,omitempty"`
}

type Event struct {
	ID        bson.ObjectID      `json:"id" bson:"_id,omitempty"`
	RolloutID bson.ObjectID      `json:"rolloutId" bson:"rolloutId"`
	At        time.Time          `json:"at" bson:"at"`
	Type      string             `json:"type" bson:"type"`
	Message   string             `json:"message" bson:"message"`
	Metrics   map[string]float64 `json:"metrics,omitempty" bson:"metrics,omitempty"`
}

type Verdict string

type Store interface {
	ListActive(ctx context.Context) ([]Rollout, error)
	UpdateRollout(ctx context.Context, r Rollout) (Rollout, error)
	AppendEvent(ctx context.Context, e Event) (Event, error)
}

type Cluster interface {
	Status(ctx context.Context, namespace, name, container string) (k8s.DeploymentStatus, error)
	SetReplicas(ctx context.Context, namespace, name string, replicas int32) error
	SetImage(ctx context.Context, namespace, name, container, image string, labels map[string]string) error
}

type Analyzer interface {
	Check(ctx context.Context, r Rollout, now time.Time) (LastCheck, error)
}

type Engine struct {
	store    Store
	cluster  Cluster
	analyzer Analyzer
	owner    string
	interval time.Duration
	now      func() time.Time
	logger   *slog.Logger
}

type EngineOptions struct {
	Owner    string
	Interval time.Duration
	Logger   *slog.Logger
	Now      func() time.Time
}

type desiredState struct {
	stableImage string
	canaryImage string
	rolloutID   string
	stable      int32
	canary      int32
}
