package workflows

import (
	"context"

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
	steps := w.getSteps(req, "")
	orchestrator := saga.NewOrchestrator("DineoutBookingWorkflow", steps, w.store)
	return orchestrator.Run(ctx)
}

func (w *DineoutBookingWorkflow) getSteps(req DineoutBookingRequest, cartID string) []saga.Step {
	steps := []saga.Step{}

	steps = append(steps, saga.Step{
		Name: "CreateCart",
		Execute: func(ctx context.Context) error {
			if cartID != "" {
				return nil // skip if already created (e.g. during resume)
			}
			resp, err := w.dineoutAPI.CreateCart(ctx, swiggy.CreateCartRequest{
				RestaurantID: req.RestaurantID,
				Guests:       req.Guests,
			})
			if err != nil {
				return err
			}
			cartID = resp.CartID
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
				CartID: cartID,
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

// Resume allows resuming a suspended dineout booking when a webhook fires.
func (w *DineoutBookingWorkflow) Resume(ctx context.Context, sagaID string, req DineoutBookingRequest) error {
	// TODO: load cartID from SagaState.Metadata on resume instead of passing a placeholder
	steps := w.getSteps(req, "resumed_cart_id")
	orchestrator := saga.NewOrchestrator("DineoutBookingWorkflow", steps, w.store)
	return orchestrator.Resume(ctx, sagaID)
}
