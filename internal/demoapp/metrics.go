package demoapp

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func newRegistry() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return reg
}

func newMetrics(reg prometheus.Registerer) *metrics {
	f := promauto.With(reg)
	return &metrics{
		requests: f.NewCounterVec(prometheus.CounterOpts{
			Name: metricRequests,
			Help: "Requests to GET /, by track and status code.",
		}, []string{labelTrack, labelCode}),
		duration: f.NewHistogramVec(prometheus.HistogramOpts{
			Name:    metricDuration,
			Help:    "Latency of GET /, by track.",
			Buckets: prometheus.DefBuckets,
		}, []string{labelTrack}),
	}
}

func (m *metrics) observe(track string, code int, d time.Duration) {
	m.requests.WithLabelValues(track, strconv.Itoa(code)).Inc()
	m.duration.WithLabelValues(track).Observe(d.Seconds())
}
