package demoapp

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func New(cfg Config, rand func() float64) *App {
	reg := newRegistry()
	return &App{cfg: cfg, rand: rand, reg: reg, metrics: newMetrics(reg)}
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", a.handleRoot)
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.Handle("GET /metrics", promhttp.HandlerFor(a.reg, promhttp.HandlerOpts{}))
	return mux
}
