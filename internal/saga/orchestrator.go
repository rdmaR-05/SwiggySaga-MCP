package saga

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"swiggy-saga-mcp/internal/swiggy"
)

// Orchestrator is the core state machine for distributed Saga transactions.
type Orchestrator struct {
	ID    string
	Name  string
	Steps []Step
	Store Store
}

// NewOrchestrator initializes a Saga state machine bound to a durable store.
func NewOrchestrator(name string, steps []Step, store Store) *Orchestrator {
	id := uuid.New().String()
	if store == nil {
		store = &NoOpStore{}
	}
	return &Orchestrator{
		ID:    id,
		Name:  name,
		Steps: steps,
		Store: store,
	}
}

// Run executes the Saga DAG sequentially. On failure, it triggers deterministic compensations (LIFO order).
func (o *Orchestrator) Run(ctx context.Context) error {
	ctx, span := otel.Tracer("swiggy.saga.mcp").Start(ctx, "Orchestrator.Run")
	span.SetAttributes(attribute.String("saga.id", o.ID), attribute.String("saga.name", o.Name))
	defer span.End()

	var executedSteps []Step
	var executedStepNames []string

	slog.Info("Starting Saga", "saga_id", o.ID, "saga_name", o.Name, "total_steps", len(o.Steps))
	startTime := time.Now()

	state := SagaState{
		SagaID:        o.ID,
		Name:          o.Name,
		Status:        "started",
		ExecutedSteps: []string{},
	}
	o.Store.SaveState(ctx, state)

	for _, step := range o.Steps {
		slog.Info("Executing Saga Step", "saga_id", o.ID, "saga_name", o.Name, "step", step.Name)
		
		stepCtx, stepSpan := otel.Tracer("swiggy.saga.mcp").Start(ctx, "Step."+step.Name)
		err := executeStepWithRetry(stepCtx, step.Execute)
		stepSpan.End()

		if err != nil {
			if errors.Is(err, swiggy.ErrNetworkTimeout) {
				slog.Warn("Saga Step timed out. Transitioning to unknown state", "saga_id", o.ID, "step", step.Name)
				state.Status = "unknown"
				o.Store.SaveState(ctx, state)
				return fmt.Errorf("saga execution paused at step %q due to network timeout (pending verification): %w", step.Name, err)
			}

			if errors.Is(err, ErrSagaSuspended) {
				slog.Info("Saga Suspended", "saga_id", o.ID, "step", step.Name)
				state.Status = "suspended"
				o.Store.SaveState(ctx, state)
				return err
			}

			slog.Error("Saga Step Failed", "saga_id", o.ID, "saga_name", o.Name, "step", step.Name, "error", err)

			state.Status = "failed"
			o.Store.SaveState(ctx, state)

			// Rollback phase
			slog.Info("Initiating Saga Rollback", "saga_id", o.ID, "saga_name", o.Name, "failed_step", step.Name)
			rollbackErr := o.rollback(ctx, executedSteps)
			
			if rollbackErr == nil {
				state.Status = "rolled_back"
				o.Store.SaveState(ctx, state)
				return fmt.Errorf("saga execution failed at step %q: %w (rollback successful)", step.Name, err)
			}
			return fmt.Errorf("saga execution failed at step %q: %w, AND rollback failed: %v", step.Name, err, rollbackErr)
		}

		executedSteps = append(executedSteps, step)
		executedStepNames = append(executedStepNames, step.Name)
		
		state.ExecutedSteps = executedStepNames
		o.Store.SaveState(ctx, state)
	}

	state.Status = "completed"
	o.Store.SaveState(ctx, state)

	slog.Info("Saga Completed Successfully", "saga_id", o.ID, "saga_name", o.Name, "duration", time.Since(startTime))
	return nil
}

func executeStepWithRetry(ctx context.Context, fn func(ctx context.Context) error) error {
	delay := 500 * time.Millisecond
	maxDelay := 8 * time.Second
	maxRetries := 5 // Default absolute maximum cap

	for i := 0; ; i++ {
		err := fn(ctx)
		if err == nil {
			return nil
		}
		
		var apiErr *swiggy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.Code {
			case swiggy.ErrCodeUpstreamTimeout, swiggy.ErrCodeUpstreamError:
				maxRetries = 5
			case swiggy.ErrCodeInternalError:
				maxRetries = 1
			default:
				// Validation, Auth, Domain failures shouldn't be retried
				return err
			}
		} else if errors.Is(err, swiggy.ErrNetworkTimeout) {
			maxRetries = 5
		} else if errors.Is(err, ErrSagaSuspended) {
			return err
		} else {
			// Unknown generic error, do not retry
			return err
		}

		if i >= maxRetries {
			return fmt.Errorf("max retries reached: %w", err)
		}

		slog.Warn("Retrying step due to error", "error", err, "attempt", i+1)

		jitter := time.Duration(rand.Int63n(int64(delay) / 2))
		sleepDuration := delay + jitter

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleepDuration):
			delay *= 2
			if delay > maxDelay {
				delay = maxDelay
			}
		}
	}
}

// Resume reconstructs and continues a suspended Saga from persistent state (e.g., after an async webhook callback).
func (o *Orchestrator) Resume(ctx context.Context, sagaID string) error {
	o.ID = sagaID
	state, err := o.Store.LoadState(ctx, sagaID)
	if err != nil || state == nil {
		return fmt.Errorf("failed to load saga state for %s: %w", sagaID, err)
	}

	if state.Status != "suspended" {
		return fmt.Errorf("cannot resume saga %s in status %s", sagaID, state.Status)
	}

	slog.Info("Resuming Suspended Saga", "saga_id", o.ID, "saga_name", o.Name)
	state.Status = "started"
	o.Store.SaveState(ctx, *state)

	var executedSteps []Step

	for _, step := range o.Steps {
		// Skip steps that have already been executed
		alreadyExecuted := false
		for _, executedName := range state.ExecutedSteps {
			if executedName == step.Name {
				alreadyExecuted = true
				break
			}
		}

		if alreadyExecuted {
			executedSteps = append(executedSteps, step)
			continue
		}

		// Execute remaining steps
		slog.Info("Executing Saga Step (Resumed)", "saga_id", o.ID, "saga_name", o.Name, "step", step.Name)
		
		stepCtx, stepSpan := otel.Tracer("swiggy.saga.mcp").Start(ctx, "Step."+step.Name)
		err := step.Execute(stepCtx)
		stepSpan.End()

		if err != nil {
			state.Status = "failed"
			o.Store.SaveState(ctx, *state)
			o.rollback(ctx, executedSteps)
			return err
		}

		executedSteps = append(executedSteps, step)
		state.ExecutedSteps = append(state.ExecutedSteps, step.Name)
		o.Store.SaveState(ctx, *state)
	}

	state.Status = "completed"
	o.Store.SaveState(ctx, *state)
	slog.Info("Resumed Saga Completed Successfully", "saga_id", o.ID, "saga_name", o.Name)
	return nil
}

// rollback guarantees eventual consistency by executing compensation handlers in LIFO order.
func (o *Orchestrator) rollback(ctx context.Context, executedSteps []Step) error {
	var rollbackErrors []error

	for i := len(executedSteps) - 1; i >= 0; i-- {
		step := executedSteps[i]
		if step.Compensate != nil {
			slog.Info("Compensating Saga Step", "saga_name", o.Name, "step", step.Name)

			// Use retry logic for compensation to ensure highest chance of rollback success
			err := WithRetry(ctx, 3, 500*time.Millisecond, step.Compensate)
			if err != nil {
				slog.Error("Failed to Compensate Saga Step", "saga_name", o.Name, "step", step.Name, "error", err)
				rollbackErrors = append(rollbackErrors, fmt.Errorf("failed to compensate %q: %w", step.Name, err))
			}
		} else {
			slog.Debug("No compensation defined for step, skipping", "saga_name", o.Name, "step", step.Name)
		}
	}

	if len(rollbackErrors) > 0 {
		return errors.Join(rollbackErrors...)
	}
	return nil
}
