package main

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/yashhkataria/canary-controller/internal/demoapp"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("demo-app stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := demoapp.ConfigFromEnv(os.Getenv)
	if err != nil {
		return err
	}
	if cfg.Track == demoapp.DefaultTrack {
		slog.Warn("TRACK is not set; metrics will carry track=\"unknown\"")
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           demoapp.New(cfg, rand.Float64).Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      cfg.ExtraLatency + writeTimeoutExtra,
		IdleTimeout:       idleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	slog.Info("demo-app listening", "addr", addr, "version", cfg.Version, "track", cfg.Track,
		"error_rate", cfg.ErrorRate, "extra_latency_ms", cfg.ExtraLatency.Milliseconds())

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down", "timeout", shutdownTimeout.String())
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	slog.Info("shutdown complete")
	return nil
}
