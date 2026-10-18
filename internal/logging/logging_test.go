package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestFromReturnsAttachedLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil)).With("request_id", "abc")

	From(With(context.Background(), logger)).Info("hello")

	if !strings.Contains(buf.String(), "request_id=abc") {
		t.Errorf("log line %q does not carry request_id", buf.String())
	}
}

func TestFromFallsBackToDefault(t *testing.T) {
	if From(context.Background()) != slog.Default() {
		t.Error("From without a logger should return slog.Default()")
	}
}
