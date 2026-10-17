package demoapp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"
)

func withDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func parseErrorRate(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	rate, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(rate) || rate < 0 || rate > 1 {
		return 0, fmt.Errorf("%s must be a number from 0 to 1, got %q", envErrorRate, s)
	}
	return rate, nil
}

func parseExtraLatency(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	ms, err := strconv.Atoi(s)
	if err != nil || ms < 0 {
		return 0, fmt.Errorf("%s must be a whole number >= 0, got %q", envExtraLatency, s)
	}
	return time.Duration(ms) * time.Millisecond, nil
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-ctx.Done():
		return false
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("write response", "err", err)
	}
}
