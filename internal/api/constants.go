package api

import (
	"regexp"
	"time"

	"github.com/yashhkataria/canary-controller/internal/rollout"
)

const (
	headerRequestID   = "X-Request-ID"
	headerContentType = "Content-Type"
	contentTypeJSON   = "application/json"
)

const (
	maxBodyBytes     = 1 << 20
	maxNameLength    = 63
	maxImageLength   = 512
	maxSteps         = 10
	stepUnit         = 100 / rollout.TotalReplicas
	minInterval      = 5 * time.Second
	readyTimeout     = 2 * time.Second
	maxRequestIDSize = 64
)

var (
	namePattern      = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
)

var quietPaths = map[string]bool{
	"/healthz": true,
	"/readyz":  true,
	"/metrics": true,
}
