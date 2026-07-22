package workflows

import (
	"context"
	"fmt"

	"swiggy-saga-mcp/internal/saga"
	"swiggy-saga-mcp/internal/swiggy"
)

type DineoutBookingRequest struct {
	RestaurantID string `json:"restaurantId" validate:"required"`
	Guests       int    `json:"guests" validate:"required,min=1"`
	SlotID       string `json:"slotId" validate:"required"`
}

type DineoutBookingWorkflow struct {
	dineoutAPI swiggy.DineoutClient
	store      saga.Store
}

func NewDineoutBookingWorkflow(api swiggy.DineoutClient, store saga.Store) *DineoutBookingWorkflow {
	return &DineoutBookingWorkflow{dineoutAPI: api, store: store}
}

func (w *DineoutBookingWorkflow) Execute(ctx context.Context, req DineoutBookingRequest) error {
	var cartID string
	orchestrator := saga.NewOrchestrator("DineoutBookingWorkflow", nil, w.store)
	orchestrator.Steps = w.buildSteps(req, &cartID, orchestrator)
	return orchestrator.Run(ctx)
}

// buildSteps constructs saga steps with a shared cartID pointer.
// The orchestrator reference lets steps persist metadata for resume.
func (w *DineoutBookingWorkflow) buildSteps(req DineoutBookingRequest, cartID *string, orch *saga.Orchestrator) []saga.Step {
	steps := []saga.Step{}

	steps = append(steps, saga.Step{
		Name: "CreateCart",
		Execute: func(ctx context.Context) error {
			if *cartID != "" {
				return nil // already created (e.g. during resume)
			}
			resp, err := w.dineoutAPI.CreateCart(ctx, swiggy.CreateCartRequest{
				RestaurantID: req.RestaurantID,
				Guests:       req.Guests,
			})
			if err != nil {
				return err
			}
			*cartID = resp.CartID
			orch.SetMetadata("cartId", *cartID)
			return nil
		},
		Compensate: func(ctx context.Context) error {
			return nil
		},
	})

	steps = append(steps, saga.Step{
		Name: "BookTable",
		Execute: func(ctx context.Context) error {
			if req.SlotID == "pending_slot" {
				return saga.ErrSagaSuspended
			}
			_, err := w.dineoutAPI.BookTable(ctx, swiggy.BookTableRequest{
				CartID: *cartID,
				SlotID: req.SlotID,
			})
			return err
		},
		Compensate: func(ctx context.Context) error {
			return w.dineoutAPI.ReportError(ctx, swiggy.ReportErrorRequest{
				Code:    swiggy.ErrCodeInternalError,
				Message: "Dineout table booking saga failed.",
				Context: "DineoutBookingWorkflow rollback",
			})
		},
	})
	return steps
}

// Resume continues a suspended dineout booking when a webhook fires.
func (w *DineoutBookingWorkflow) Resume(ctx context.Context, sagaID string, req DineoutBookingRequest) error {
	state, err := w.store.LoadState(ctx, sagaID)
	if err != nil {
		return fmt.Errorf("failed to load saga state for resume: %w", err)
	}

	// recover cartID from persisted metadata
	cartID := ""
	if state != nil && state.Metadata != nil {
		cartID = state.Metadata["cartId"]
	}

	orchestrator := saga.NewOrchestrator("DineoutBookingWorkflow", nil, w.store)
	orchestrator.Steps = w.buildSteps(req, &cartID, orchestrator)
	return orchestrator.Resume(ctx, sagaID)
}
