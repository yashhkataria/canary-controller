package demoapp

const (
	DefaultVersion = "dev"
	DefaultTrack   = "unknown"
)

const (
	envVersion      = "VERSION"
	envTrack        = "TRACK"
	envErrorRate    = "ERROR_RATE"
	envExtraLatency = "EXTRA_LATENCY_MS"
)

const (
	metricRequests = "http_requests_total"
	metricDuration = "http_request_duration_seconds"
	labelTrack     = "track"
	labelCode      = "code"
)

const injectedErrorMessage = "injected error"
