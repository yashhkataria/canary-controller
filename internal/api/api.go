package api

import (
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func New(s Store, logger *slog.Logger) *Server {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return &Server{store: s, logger: logger, reg: reg}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/create-rollout", s.createRollout)
	mux.HandleFunc("GET /v1/rollouts", s.listRollouts)
	mux.HandleFunc("GET /v1/rollouts/{id}", s.getRollout)
	mux.HandleFunc("GET /v1/rollouts/{id}/events", s.listEvents)
	mux.HandleFunc("POST /v1/rollouts/{id}/abort", s.notImplemented("abort"))
	mux.HandleFunc("POST /v1/rollouts/{id}/promote", s.notImplemented("promote"))
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /readyz", s.readyz)
	mux.Handle("GET /metrics", promhttp.HandlerFor(s.reg, promhttp.HandlerOpts{}))

	return s.withRequestID(s.logRequests(s.recoverPanics(mux)))
}
