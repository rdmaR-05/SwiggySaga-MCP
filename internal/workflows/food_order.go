package workflows

import (
	"context"

	"swiggy-saga-mcp/internal/saga"
	"swiggy-saga-mcp/internal/swiggy"
)

// FoodOrderRequest represents the input for a food order saga.
type FoodOrderRequest struct {
	AddressID     string `json:"addressId" validate:"required"`
	PaymentMethod string `json:"paymentMethod" validate:"required"`
	CouponCode    string `json:"couponCode,omitempty"`
}

// FoodOrderWorkflow orchestrates placing a food order.
type FoodOrderWorkflow struct {
	foodAPI swiggy.FoodClient
	store   saga.Store
}

func NewFoodOrderWorkflow(foodAPI swiggy.FoodClient, store saga.Store) *FoodOrderWorkflow {
	return &FoodOrderWorkflow{foodAPI: foodAPI, store: store}
}

func (f *FoodOrderWorkflow) Execute(ctx context.Context, req FoodOrderRequest) error {
	steps := []saga.Step{}

	if req.CouponCode != "" {
		steps = append(steps, saga.Step{
			Name: "ApplyCoupon",
			Execute: func(ctx context.Context) error {
				_, err := f.foodAPI.ApplyCoupon(ctx, swiggy.ApplyFoodCouponRequest{
					CouponCode: req.CouponCode,
				})
				return err
			},
			Compensate: func(ctx context.Context) error {
				// To undo coupon apply, we flush the cart or update it to remove it.
				// Based on docs, flush_food_cart exists.
				return f.foodAPI.FlushCart(ctx)
			},
		})
	}

	steps = append(steps, saga.Step{
		Name: "PlaceFoodOrder",
		Execute: func(ctx context.Context) error {
			_, err := f.foodAPI.PlaceOrder(ctx, swiggy.PlaceFoodOrderRequest{
				AddressID:     req.AddressID,
				PaymentMethod: req.PaymentMethod,
			})
			return err
		},
		Compensate: func(ctx context.Context) error {
			// food orders aren't cancellable via MCP; report to Swiggy support instead
			return f.foodAPI.ReportError(ctx, swiggy.ReportErrorRequest{
				Code:    swiggy.ErrCodeInternalError,
				Message: "Food order placement failed during saga.",
				Context: "FoodOrderWorkflow rollback",
			})
		},
	})

	orchestrator := saga.NewOrchestrator("FoodOrderWorkflow", steps, f.store)
	return orchestrator.Run(ctx)
}
