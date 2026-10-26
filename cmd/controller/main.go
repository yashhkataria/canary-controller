package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yashhkataria/canary-controller/internal/api"
	"github.com/yashhkataria/canary-controller/internal/k8s"
	"github.com/yashhkataria/canary-controller/internal/rollout"
	"github.com/yashhkataria/canary-controller/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("controller stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)
	logger.Info("starting controller", cfg.logValue())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()

	startCtx, cancelStart := context.WithTimeout(ctx, startupTimeout)
	defer cancelStart()
	st, err := store.New(startCtx, cfg.MongoURI, cfg.MongoDatabase)
	if err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := st.Close(closeCtx); err != nil {
			logger.Warn("close mongodb", "err", err)
		}
	}()
	if err := st.EnsureIndexes(startCtx); err != nil {
		return err
	}
	logger.Info("connected to mongodb", "database", cfg.MongoDatabase)

	kc, err := k8s.New(cfg.Kubeconfig)
	if err != nil {
		return err
	}
	engine := rollout.NewEngine(st, kc, rollout.FixedWait{}, rollout.EngineOptions{
		Owner:    cfg.Owner,
		Interval: cfg.EngineInterval,
		Logger:   logger,
	})
	engineDone := make(chan struct{})
	go func() {
		defer close(engineDone)
		engine.Run(ctx)
	}()

	srv := &http.Server{
		Addr:              net.JoinHostPort("", cfg.Port),
		Handler:           api.New(st, logger).Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	logger.Info("controller listening", "addr", srv.Addr)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	stop()

	logger.Info("shutting down", "timeout", shutdownTimeout.String())
	select {
	case <-engineDone:
	case <-time.After(engineStopTimeout):
		logger.Warn("rollout engine did not stop in time")
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	logger.Info("shutdown complete")
	return nil
}
