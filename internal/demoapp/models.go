package demoapp

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Config struct {
	Version      string
	Track        string
	ErrorRate    float64
	ExtraLatency time.Duration
}

type App struct {
	cfg     Config
	rand    func() float64
	reg     *prometheus.Registry
	metrics *metrics
}

type metrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

type rootResponse struct {
	Version string `json:"version"`
	Track   string `json:"track"`
	Error   string `json:"error,omitempty"`
}
