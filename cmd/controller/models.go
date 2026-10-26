package main

import (
	"log/slog"
	"time"
)

type Config struct {
	Port           string
	MongoURI       string
	MongoDatabase  string
	PrometheusURL  string
	RedisAddr      string
	LogLevel       slog.Level
	EngineInterval time.Duration
	Kubeconfig     string
	Owner          string
}
