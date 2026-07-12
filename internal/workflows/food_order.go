package workflows

import (
	"context"

	"swiggy-saga-mcp/internal/saga"
	"swiggy-saga-mcp/internal/swiggy"
)

// FoodOrderRequest represents the input for a food order saga.
type FoodOrderRequest struct {
	AddressID     string `json:"addressId" validate:"required"`
	PaymentMethod string `json:"paymentMethod,omitempty"`
	CouponCode    string `json:"couponCode,omitempty"`
}

// FoodOrderWorkflow handles the complex flow of placing a food order.
type FoodOrderWorkflow struct {
	foodAPI *swiggy.FoodAPI
	store   saga.Store
}

func NewFoodOrderWorkflow(foodAPI *swiggy.FoodAPI, store saga.Store) *FoodOrderWorkflow {
	return &FoodOrderWorkflow{foodAPI: foodAPI, store: store}
}

// Execute triggers the food order saga.
func (f *FoodOrderWorkflow) Execute(ctx context.Context, req FoodOrderRequest) error {
	steps := []saga.Step{}

	// 1. Validate and apply discount (if provided)
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

	// 2. Execute final checkout mutation
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
			// Swiggy docs say food orders can't be cancelled programmatically via MCP,
			// user must call customer care. We report this error to Swiggy telemetry/support.
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
