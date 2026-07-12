package workflows

import (
	"context"

	"swiggy-saga-mcp/internal/saga"
	"swiggy-saga-mcp/internal/swiggy"
)

type InstamartCheckoutRequest struct {
	Items         []swiggy.UpdateCartRequest   `json:"items" validate:"required,min=1,dive"`
	AddressID     string                       `json:"addressId,omitempty"` // Existing or Empty if new
	NewAddress    *swiggy.CreateAddressRequest `json:"newAddress,omitempty"`
	PaymentMethod string                       `json:"paymentMethod,omitempty"`
}

type InstamartCheckoutWorkflow struct {
	instamartAPI *swiggy.InstamartAPI
	store        saga.Store
}

func NewInstamartCheckoutWorkflow(api *swiggy.InstamartAPI, store saga.Store) *InstamartCheckoutWorkflow {
	return &InstamartCheckoutWorkflow{instamartAPI: api, store: store}
}

func (w *InstamartCheckoutWorkflow) Execute(ctx context.Context, req InstamartCheckoutRequest) error {
	steps := []saga.Step{}

	addrID := req.AddressID

	// 1. Dynamically provision delivery address if omitted
	if req.NewAddress != nil && req.AddressID == "" {
		steps = append(steps, saga.Step{
			Name: "CreateAddress",
			Execute: func(ctx context.Context) error {
				resp, err := w.instamartAPI.CreateAddress(ctx, *req.NewAddress)
				if err != nil {
					return err
				}
				addrID = resp.AddressID
				return nil
			},
			Compensate: func(ctx context.Context) error {
				if addrID != "" {
					return w.instamartAPI.DeleteAddress(ctx, swiggy.DeleteAddressRequest{AddressID: addrID})
				}
				return nil
			},
		})
	}

	// 2. Add Items to Cart
	steps = append(steps, saga.Step{
		Name: "AddItems",
		Execute: func(ctx context.Context) error {
			for _, item := range req.Items {
				if err := w.instamartAPI.UpdateCart(ctx, item); err != nil {
					return err
				}
			}
			return nil
		},
		Compensate: func(ctx context.Context) error {
			// Only remove the specific items we added in this request
			return w.instamartAPI.RemoveItemsFromCart(ctx, req.Items)
		},
	})

	// 3. Finalize Instamart cart transaction
	steps = append(steps, saga.Step{
		Name: "InstamartCheckout",
		Execute: func(ctx context.Context) error {
			_, err := w.instamartAPI.Checkout(ctx, swiggy.CheckoutRequest{
				AddressID:     addrID,
				PaymentMethod: req.PaymentMethod,
			})
			return err
		},
		Compensate: func(ctx context.Context) error {
			return w.instamartAPI.ReportError(ctx, swiggy.ReportErrorRequest{
				Code:    swiggy.ErrCodeInternalError,
				Message: "Instamart checkout failed during saga, manual review required.",
				Context: "InstamartCheckoutWorkflow rollback",
			})
		},
	})

	orchestrator := saga.NewOrchestrator("InstamartCheckoutWorkflow", steps, w.store)
	return orchestrator.Run(ctx)
}
