package session

import (
	"context"
	"time"
)

// Sleep waits for d or returns early when ctx is cancelled.
func Sleep(ctx context.Context, d time.Duration) {
	if ctx == nil {
		ctx = context.Background()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

// Remaining returns the time left on ctx, or fallback without a deadline.
func Remaining(ctx context.Context, fallback time.Duration) time.Duration {
	if ctx == nil {
		return fallback
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return fallback
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return 0
	}
	return remaining
}

// Timeout caps a local operation at both fallback and the deadline on ctx.
func Timeout(ctx context.Context, fallback time.Duration) time.Duration {
	if ctx == nil {
		return fallback
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return fallback
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return 0
	}
	if remaining < fallback {
		return remaining
	}
	return fallback
}
