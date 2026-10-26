package main

import "time"

const (
	envPort           = "PORT"
	envMongoURI       = "MONGO_URI"
	envMongoDatabase  = "MONGO_DATABASE"
	envPrometheusURL  = "PROMETHEUS_URL"
	envRedisAddr      = "REDIS_ADDR"
	envLogLevel       = "LOG_LEVEL"
	envEngineInterval = "ENGINE_INTERVAL"
	envKubeconfig     = "KUBECONFIG"
	envPodName        = "POD_NAME"
	envHostname       = "HOSTNAME"
)

const (
	defaultPort           = "8080"
	defaultMongoURI       = "mongodb://mongodb.canary-system:27017"
	defaultMongoDatabase  = "canary"
	defaultPrometheusURL  = "http://monitoring-kube-prometheus-prometheus.monitoring:9090"
	defaultRedisAddr      = "redis.canary-system:6379"
	defaultLogLevel       = "info"
	defaultEngineInterval = "2s"
	defaultOwner          = "controller"
)

const (
	minEngineInterval = 500 * time.Millisecond
	maxEngineInterval = time.Minute
)

const (
	startupTimeout    = 30 * time.Second
	shutdownTimeout   = 10 * time.Second
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
	engineStopTimeout = 15 * time.Second
)
