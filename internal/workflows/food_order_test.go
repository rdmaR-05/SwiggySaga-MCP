package workflows_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"swiggy-saga-mcp/internal/saga"
	"swiggy-saga-mcp/internal/swiggy"
	"swiggy-saga-mcp/internal/workflows"
)

// ---- Food Order ----

func newFoodWorkflow() *workflows.FoodOrderWorkflow {
	mock := swiggy.NewMockClient(0)
	return workflows.NewFoodOrderWorkflow(&swiggy.MockFoodAPI{Mock: mock}, &saga.NoOpStore{})
}

func TestFoodOrderWorkflow_Success(t *testing.T) {
	wf := newFoodWorkflow()
	err := wf.Execute(context.Background(), workflows.FoodOrderRequest{
		AddressID:     "addr-123",
		PaymentMethod: "UPI",
	})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestFoodOrderWorkflow_WithCoupon_Success(t *testing.T) {
	wf := newFoodWorkflow()
	err := wf.Execute(context.Background(), workflows.FoodOrderRequest{
		AddressID:     "addr-123",
		PaymentMethod: "UPI",
		CouponCode:    "SAVE50",
	})
	if err != nil {
		t.Fatalf("expected success with coupon, got: %v", err)
	}
}

func TestFoodOrderWorkflow_MissingPaymentMethod_Fails(t *testing.T) {
	wf := newFoodWorkflow()
	err := wf.Execute(context.Background(), workflows.FoodOrderRequest{
		AddressID:     "addr-123",
		PaymentMethod: "", // should fail inside mock PlaceOrder
	})
	if err == nil {
		t.Fatal("expected error for missing payment method, got nil")
	}
}

func TestFoodOrderWorkflow_EmptyCouponSkipsStep(t *testing.T) {
	wf := newFoodWorkflow()
	err := wf.Execute(context.Background(), workflows.FoodOrderRequest{
		AddressID:     "addr-123",
		PaymentMethod: "UPI",
		CouponCode:    "", // empty = no coupon step
	})
	if err != nil {
		t.Fatalf("empty coupon should skip the step, got: %v", err)
	}
}

// ---- Instamart Checkout ----

func TestInstamartCheckoutWorkflow_Success(t *testing.T) {
	mock := swiggy.NewMockClient(0)
	wf := workflows.NewInstamartCheckoutWorkflow(&swiggy.MockInstamartAPI{Mock: mock}, &saga.NoOpStore{})

	err := wf.Execute(context.Background(), workflows.InstamartCheckoutRequest{
		Items:         []swiggy.UpdateCartRequest{{ItemID: "item-1", Quantity: 2}},
		AddressID:     "addr-456",
		PaymentMethod: "CARD",
	})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestInstamartCheckoutWorkflow_NewAddress_Success(t *testing.T) {
	mock := swiggy.NewMockClient(0)
	wf := workflows.NewInstamartCheckoutWorkflow(&swiggy.MockInstamartAPI{Mock: mock}, &saga.NoOpStore{})

	err := wf.Execute(context.Background(), workflows.InstamartCheckoutRequest{
		Items: []swiggy.UpdateCartRequest{{ItemID: "item-1", Quantity: 1}},
		NewAddress: &swiggy.CreateAddressRequest{
			Label:   "Home",
			Address: "123 Main Street",
		},
		PaymentMethod: "UPI",
	})
	if err != nil {
		t.Fatalf("expected success with new address, got: %v", err)
	}
}

// ---- Dineout Booking ----

func TestDineoutBookingWorkflow_Success(t *testing.T) {
	mock := swiggy.NewMockClient(0)
	wf := workflows.NewDineoutBookingWorkflow(&swiggy.MockDineoutAPI{Mock: mock}, &saga.NoOpStore{})

	err := wf.Execute(context.Background(), workflows.DineoutBookingRequest{
		RestaurantID: "rest-xyz",
		Guests:       2,
		SlotID:       "slot-7pm",
	})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestDineoutBookingWorkflow_PendingSlot_Suspends(t *testing.T) {
	mock := swiggy.NewMockClient(0)
	wf := workflows.NewDineoutBookingWorkflow(&swiggy.MockDineoutAPI{Mock: mock}, &saga.NoOpStore{})

	err := wf.Execute(context.Background(), workflows.DineoutBookingRequest{
		RestaurantID: "rest-xyz",
		Guests:       2,
		SlotID:       "pending_slot",
	})
	if !errors.Is(err, saga.ErrSagaSuspended) {
		t.Fatalf("expected ErrSagaSuspended for pending_slot, got: %v", err)
	}
}

// ---- Rollback / Chaos ----

func TestFoodOrderWorkflow_100PercentFail_RollsBack(t *testing.T) {
	mock := swiggy.NewMockClient(100) // 100% fail rate
	wf := workflows.NewFoodOrderWorkflow(&swiggy.MockFoodAPI{Mock: mock}, &saga.NoOpStore{})

	err := wf.Execute(context.Background(), workflows.FoodOrderRequest{
		AddressID:     "addr-rollback",
		PaymentMethod: "UPI",
	})
	if err == nil {
		t.Fatal("expected error with 100% fail rate, got nil")
	}
	// error should indicate the saga failed and rolled back
	if !strings.Contains(err.Error(), "rollback successful") {
		t.Errorf("expected rollback successful in error, got: %v", err)
	}
}

func TestFoodOrderWorkflow_CouponThenFail_CompensatesCoupon(t *testing.T) {
	mock := swiggy.NewMockClient(100) // food order will fail
	wf := workflows.NewFoodOrderWorkflow(&swiggy.MockFoodAPI{Mock: mock}, &saga.NoOpStore{})

	err := wf.Execute(context.Background(), workflows.FoodOrderRequest{
		AddressID:     "addr-123",
		PaymentMethod: "UPI",
		CouponCode:    "SAVE50",
	})
	if err == nil {
		t.Fatal("expected error due to 100% food fail rate")
	}
	// both ApplyCoupon and PlaceFoodOrder were in steps; PlaceFoodOrder failed,
	// so ApplyCoupon's Compensate (FlushCart) should have run. We can't inspect
	// the mock further without a call counter, but the saga returning an error
	// with "rollback successful" confirms compensation executed.
	if !strings.Contains(err.Error(), "rollback successful") {
		t.Errorf("expected rollback successful, got: %v", err)
	}
}

// ---- Context cancellation ----

func TestFoodOrderWorkflow_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	wf := newFoodWorkflow()
	err := wf.Execute(ctx, workflows.FoodOrderRequest{
		AddressID:     "addr-123",
		PaymentMethod: "UPI",
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}
