package main

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr string
	}{
		{
			name: "defaults",
			env:  map[string]string{},
			want: Config{
				Port:           defaultPort,
				MongoURI:       defaultMongoURI,
				MongoDatabase:  defaultMongoDatabase,
				PrometheusURL:  defaultPrometheusURL,
				RedisAddr:      defaultRedisAddr,
				LogLevel:       slog.LevelInfo,
				EngineInterval: 2 * time.Second,
				Owner:          defaultOwner,
			},
		},
		{
			name: "all set",
			env: map[string]string{
				envPort: "9000", envMongoURI: "mongodb://localhost:27017", envMongoDatabase: "test",
				envPrometheusURL: "https://prom.example:9090", envRedisAddr: "localhost:6379", envLogLevel: "debug",
				envEngineInterval: "5s", envKubeconfig: "/tmp/kc", envPodName: "controller-abc", envHostname: "ignored",
			},
			want: Config{
				Port: "9000", MongoURI: "mongodb://localhost:27017", MongoDatabase: "test",
				PrometheusURL: "https://prom.example:9090", RedisAddr: "localhost:6379", LogLevel: slog.LevelDebug,
				EngineInterval: 5 * time.Second, Kubeconfig: "/tmp/kc", Owner: "controller-abc",
			},
		},
		{name: "port not a number", env: map[string]string{envPort: "http"}, wantErr: envPort},
		{name: "port out of range", env: map[string]string{envPort: "70000"}, wantErr: envPort},
		{name: "mongo uri scheme", env: map[string]string{envMongoURI: "localhost:27017"}, wantErr: envMongoURI},
		{name: "mongo database with dot", env: map[string]string{envMongoDatabase: "canary.v1"}, wantErr: envMongoDatabase},
		{name: "prometheus without scheme", env: map[string]string{envPrometheusURL: "prometheus:9090"}, wantErr: envPrometheusURL},
		{name: "redis without port", env: map[string]string{envRedisAddr: "redis"}, wantErr: envRedisAddr},
		{name: "unknown log level", env: map[string]string{envLogLevel: "loud"}, wantErr: envLogLevel},
		{name: "engine interval not a duration", env: map[string]string{envEngineInterval: "2"}, wantErr: envEngineInterval},
		{name: "engine interval too fast", env: map[string]string{envEngineInterval: "100ms"}, wantErr: envEngineInterval},
		{name: "engine interval too slow", env: map[string]string{envEngineInterval: "2m"}, wantErr: envEngineInterval},
		{name: "several errors at once", env: map[string]string{envPort: "x", envRedisAddr: "redis"}, wantErr: envRedisAddr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadConfig(func(k string) string { return tt.env[k] })
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want it to mention %s", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v\nwant %+v", got, tt.want)
			}
		})
	}
}

func TestOwnerFallsBackToHostname(t *testing.T) {
	got, err := loadConfig(func(k string) string { return map[string]string{envHostname: "my-laptop"}[k] })
	if err != nil || got.Owner != "my-laptop" {
		t.Errorf("owner = %q (err %v), want my-laptop", got.Owner, err)
	}
}

func TestRedactURI(t *testing.T) {
	tests := map[string]string{
		"mongodb://mongodb.canary-system:27017":     "mongodb://mongodb.canary-system:27017",
		"mongodb://admin:s3cret@db:27017/?tls=true": "mongodb://admin:xxxxx@db:27017/?tls=true",
		"mongodb://admin@db:27017":                  "mongodb://admin@db:27017",
	}
	for in, want := range tests {
		if got := redactURI(in); got != want {
			t.Errorf("redactURI(%q) = %q, want %q", in, got, want)
		}
	}
}
