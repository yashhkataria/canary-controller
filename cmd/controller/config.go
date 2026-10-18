package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func loadConfig(getenv func(string) string) (Config, error) {
	cfg := Config{
		Port:          withDefault(getenv(envPort), defaultPort),
		MongoURI:      withDefault(getenv(envMongoURI), defaultMongoURI),
		MongoDatabase: withDefault(getenv(envMongoDatabase), defaultMongoDatabase),
		PrometheusURL: withDefault(getenv(envPrometheusURL), defaultPrometheusURL),
		RedisAddr:     withDefault(getenv(envRedisAddr), defaultRedisAddr),
	}

	var errs []error
	if p, err := strconv.Atoi(cfg.Port); err != nil || p < 1 || p > 65535 {
		errs = append(errs, fmt.Errorf("%s must be a port from 1 to 65535, got %q", envPort, cfg.Port))
	}
	if !strings.HasPrefix(cfg.MongoURI, "mongodb://") && !strings.HasPrefix(cfg.MongoURI, "mongodb+srv://") {
		errs = append(errs, fmt.Errorf("%s must start with mongodb:// or mongodb+srv://", envMongoURI))
	}
	if strings.ContainsAny(cfg.MongoDatabase, `/\. "$`) {
		errs = append(errs, fmt.Errorf("%s contains characters MongoDB does not allow in database names", envMongoDatabase))
	}
	if u, err := url.Parse(cfg.PrometheusURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		errs = append(errs, fmt.Errorf("%s must be an http(s) URL, got %q", envPrometheusURL, cfg.PrometheusURL))
	}
	if host, port, err := net.SplitHostPort(cfg.RedisAddr); err != nil || host == "" || port == "" {
		errs = append(errs, fmt.Errorf("%s must be host:port, got %q", envRedisAddr, cfg.RedisAddr))
	}
	if err := cfg.LogLevel.UnmarshalText([]byte(withDefault(getenv(envLogLevel), defaultLogLevel))); err != nil {
		errs = append(errs, fmt.Errorf("%s must be debug, info, warn or error", envLogLevel))
	}
	return cfg, errors.Join(errs...)
}

func withDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func (c Config) logValue() slog.Attr {
	return slog.Group("config",
		"port", c.Port,
		"mongo_uri", redactURI(c.MongoURI),
		"mongo_database", c.MongoDatabase,
		"prometheus_url", c.PrometheusURL,
		"redis_addr", c.RedisAddr,
		"log_level", c.LogLevel.String(),
	)
}

func redactURI(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	if _, hasPassword := u.User.Password(); hasPassword {
		u.User = url.UserPassword(u.User.Username(), "xxxxx")
	}
	return u.String()
}
