package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/yashhkataria/canary-controller/internal/rollout"
	"github.com/yashhkataria/canary-controller/internal/store"
)

type Store interface {
	CreateRollout(ctx context.Context, r rollout.Rollout) (rollout.Rollout, error)
	GetRollout(ctx context.Context, id bson.ObjectID) (rollout.Rollout, error)
	ListRollouts(ctx context.Context, f store.ListFilter) ([]rollout.Rollout, error)
	UpdateRollout(ctx context.Context, r rollout.Rollout) (rollout.Rollout, error)
	AppendEvent(ctx context.Context, e rollout.Event) (rollout.Event, error)
	ListEvents(ctx context.Context, rolloutID bson.ObjectID) ([]rollout.Event, error)
	Ping(ctx context.Context) error
}

type Server struct {
	store  Store
	logger *slog.Logger
	reg    *prometheus.Registry
}

type CreateRolloutRequest struct {
	Namespace string          `json:"namespace"`
	App       string          `json:"app"`
	Image     string          `json:"image"`
	Steps     []int           `json:"steps"`
	Analysis  AnalysisRequest `json:"analysis"`
}

type AnalysisRequest struct {
	Interval      *rollout.Duration `json:"interval"`
	ChecksPerStep *int              `json:"checksPerStep"`
	FailureLimit  *int              `json:"failureLimit"`
	MaxErrorRate  *float64          `json:"maxErrorRate"`
	ErrorMargin   *float64          `json:"errorMargin"`
	MaxP99Ms      *float64          `json:"maxP99Ms"`
	LatencyFactor *float64          `json:"latencyFactor"`
	MinRequests   *int              `json:"minRequests"`
	MaxWait       *rollout.Duration `json:"maxWait"`
}

type errorResponse struct {
	Error  string            `json:"error"`
	Fields map[string]string `json:"fields,omitempty"`
}

type listResponse[T any] struct {
	Items []T `json:"items"`
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}
