package analysis

import (
	"time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

type Prometheus struct {
	api     v1.API
	timeout time.Duration
}
