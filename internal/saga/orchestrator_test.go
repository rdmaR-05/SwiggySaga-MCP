package saga_test

import (
	"context"
	"errors"
	"testing"

	"swiggy-saga-mcp/internal/saga"
)

func TestOrchestrator_Success(t *testing.T) {
	called := []string{}
	steps := []saga.Step{
		{Name: "A", Execute: func(ctx context.Context) error { called = append(called, "A"); return nil }},
		{Name: "B", Execute: func(ctx context.Context) error { called = append(called, "B"); return nil }},
	}

	orch := saga.NewOrchestrator("TestSaga", steps, &saga.NoOpStore{})
	if err := orch.Run(context.Background()); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if len(called) != 2 || called[0] != "A" || called[1] != "B" {
		t.Errorf("expected steps [A B], got %v", called)
	}
}

func TestOrchestrator_RollbackOnFailure(t *testing.T) {
	compensated := false
	errBoom := errors.New("step B exploded")

	steps := []saga.Step{
		{
			Name:       "A",
			Execute:    func(ctx context.Context) error { return nil },
			Compensate: func(ctx context.Context) error { compensated = true; return nil },
		},
		{
			Name:    "B",
			Execute: func(ctx context.Context) error { return errBoom },
		},
	}

	orch := saga.NewOrchestrator("TestSaga", steps, &saga.NoOpStore{})
	if err := orch.Run(context.Background()); err == nil {
		t.Fatal("expected error, got nil")
	}
	if !compensated {
		t.Error("expected step A compensation to run after step B failure")
	}
}

func TestOrchestrator_RollbackIsLIFO(t *testing.T) {
	var order []string

	steps := []saga.Step{
		{
			Name:       "A",
			Execute:    func(ctx context.Context) error { return nil },
			Compensate: func(ctx context.Context) error { order = append(order, "A"); return nil },
		},
		{
			Name:       "B",
			Execute:    func(ctx context.Context) error { return nil },
			Compensate: func(ctx context.Context) error { order = append(order, "B"); return nil },
		},
		{
			Name:    "C",
			Execute: func(ctx context.Context) error { return errors.New("boom") },
		},
	}

	saga.NewOrchestrator("TestSaga", steps, &saga.NoOpStore{}).Run(context.Background()) //nolint:errcheck

	if len(order) != 2 || order[0] != "B" || order[1] != "A" {
		t.Errorf("expected LIFO rollback [B A], got %v", order)
	}
}

func TestOrchestrator_NilCompensateSkipped(t *testing.T) {
	steps := []saga.Step{
		{
			Name:       "A",
			Execute:    func(ctx context.Context) error { return nil },
			Compensate: nil, // no rollback defined
		},
		{
			Name:    "B",
			Execute: func(ctx context.Context) error { return errors.New("fail") },
		},
	}

	// should not panic even though A has no Compensate
	orch := saga.NewOrchestrator("TestSaga", steps, &saga.NoOpStore{})
	err := orch.Run(context.Background())
	if err == nil {
		t.Fatal("expected error from step B")
	}
}

func TestOrchestrator_EmptySteps(t *testing.T) {
	orch := saga.NewOrchestrator("EmptySaga", []saga.Step{}, &saga.NoOpStore{})
	if err := orch.Run(context.Background()); err != nil {
		t.Fatalf("empty saga should succeed, got: %v", err)
	}
}

func TestOrchestrator_SuspendedStep(t *testing.T) {
	steps := []saga.Step{
		{
			Name:    "Suspend",
			Execute: func(ctx context.Context) error { return saga.ErrSagaSuspended },
		},
	}

	orch := saga.NewOrchestrator("SuspendSaga", steps, &saga.NoOpStore{})
	err := orch.Run(context.Background())
	if !errors.Is(err, saga.ErrSagaSuspended) {
		t.Errorf("expected ErrSagaSuspended, got %v", err)
	}
}
