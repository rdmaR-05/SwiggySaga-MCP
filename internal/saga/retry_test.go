package saga_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"swiggy-saga-mcp/internal/saga"
)

func TestWithRetry_SuccessOnFirstCall(t *testing.T) {
	calls := 0
	err := saga.WithRetry(context.Background(), 3, time.Millisecond, func(ctx context.Context) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestWithRetry_RetriesAndEventuallySucceeds(t *testing.T) {
	calls := 0
	err := saga.WithRetry(context.Background(), 5, time.Millisecond, func(ctx context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestWithRetry_ExhaustsMaxRetries(t *testing.T) {
	calls := 0
	err := saga.WithRetry(context.Background(), 3, time.Millisecond, func(ctx context.Context) error {
		calls++
		return errors.New("permanent failure")
	})
	if !errors.Is(err, saga.ErrMaxRetriesReached) {
		t.Errorf("expected ErrMaxRetriesReached, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected exactly 3 calls, got %d", calls)
	}
}

func TestWithRetry_AbortsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	err := saga.WithRetry(ctx, 10, time.Millisecond, func(ctx context.Context) error {
		calls++
		if calls == 1 {
			cancel() // cancel after first attempt
		}
		return errors.New("transient")
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestWithRetry_ZeroRetries(t *testing.T) {
	calls := 0
	err := saga.WithRetry(context.Background(), 0, time.Millisecond, func(ctx context.Context) error {
		calls++
		return errors.New("fail")
	})
	if !errors.Is(err, saga.ErrMaxRetriesReached) {
		t.Errorf("expected ErrMaxRetriesReached for zero retries, got %v", err)
	}
	if calls != 0 {
		t.Errorf("expected 0 calls for zero retries, got %d", calls)
	}
}
