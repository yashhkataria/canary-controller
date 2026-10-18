package main

import "log/slog"

type Config struct {
	Port          string
	MongoURI      string
	MongoDatabase string
	PrometheusURL string
	RedisAddr     string
	LogLevel      slog.Level
}
