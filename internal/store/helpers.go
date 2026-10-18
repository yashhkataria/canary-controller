package store

import (
	"context"
	"time"
)

func withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, opTimeout)
}

func now() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}

func clampLimit(n int) int {
	switch {
	case n <= 0:
		return DefaultListLimit
	case n > MaxListLimit:
		return MaxListLimit
	default:
		return n
	}
}
