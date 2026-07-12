package saga

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

// RetryableFunc is a function that returns an error which can be retried.
type RetryableFunc func(ctx context.Context) error

var (
	ErrMaxRetriesReached = errors.New("maximum retries reached")
)

// WithRetry provides jittered exponential backoff for transient fault tolerance without overwhelming downstream services.
func WithRetry(ctx context.Context, maxRetries int, initialDelay time.Duration, fn RetryableFunc) error {
	delay := initialDelay

	for i := 0; i < maxRetries; i++ {
		err := fn(ctx)
		if err == nil {
			return nil
		}

		// If it's a context error, we should abort retries
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}

		// Calculate jitter to avoid thundering herd problem
		jitter := time.Duration(rand.Int63n(int64(delay) / 2))
		sleepDuration := delay + jitter

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleepDuration):
			// Exponential backoff
			delay *= 2
		}
	}

	return ErrMaxRetriesReached
}
