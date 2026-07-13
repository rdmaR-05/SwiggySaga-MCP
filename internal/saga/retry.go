package saga

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

// RetryableFunc is the function signature expected by WithRetry.
type RetryableFunc func(ctx context.Context) error

var (
	ErrMaxRetriesReached = errors.New("maximum retries reached")
)

// WithRetry runs fn up to maxRetries times with jittered exponential backoff.
func WithRetry(ctx context.Context, maxRetries int, initialDelay time.Duration, fn RetryableFunc) error {
	delay := initialDelay

	for i := 0; i < maxRetries; i++ {
		err := fn(ctx)
		if err == nil {
			return nil
		}

		// abort on context cancellation
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}

		// jitter to spread concurrent retries
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
