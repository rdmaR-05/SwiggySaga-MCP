package swiggy

import (
	"context"
	"fmt"
)

// MockFoodAPI satisfies FoodClient using MockClient for local testing.
type MockFoodAPI struct {
	Mock *MockClient
}

func (m *MockFoodAPI) ApplyCoupon(ctx context.Context, req ApplyFoodCouponRequest) (*ApplyFoodCouponResponse, error) {
	if req.CouponCode == "" {
		return nil, fmt.Errorf("%w: coupon code required", ErrInvalidRequest)
	}
	return &ApplyFoodCouponResponse{CartTotal: 299.0, Discount: 50.0}, nil
}

func (m *MockFoodAPI) FlushCart(ctx context.Context) error { return nil }

func (m *MockFoodAPI) PlaceOrder(ctx context.Context, req PlaceFoodOrderRequest) (*PlaceFoodOrderResponse, error) {
	orderID, err := m.Mock.PlaceFoodOrder(ctx, req.PaymentMethod, req.AddressID)
	if err != nil {
		return nil, err
	}
	return &PlaceFoodOrderResponse{
		OrderID:       orderID,
		Status:        "placed",
		PaymentMethod: req.PaymentMethod,
	}, nil
}

func (m *MockFoodAPI) ReportError(ctx context.Context, req ReportErrorRequest) error {
	return m.Mock.ReportError(ctx, "food", req.Code, req.Message, req.Context, nil, "")
}

// MockInstamartAPI satisfies InstamartClient using MockClient for local testing.
type MockInstamartAPI struct {
	Mock *MockClient
}

func (m *MockInstamartAPI) CreateAddress(ctx context.Context, req CreateAddressRequest) (*CreateAddressResponse, error) {
	if req.Address == "" {
		return nil, fmt.Errorf("%w: address required", ErrInvalidRequest)
	}
	return &CreateAddressResponse{AddressID: m.Mock.generateID("ADDR")}, nil
}

func (m *MockInstamartAPI) DeleteAddress(ctx context.Context, req DeleteAddressRequest) error { return nil }

func (m *MockInstamartAPI) UpdateCart(ctx context.Context, req UpdateCartRequest) error {
	if req.ItemID == "" || req.Quantity < 1 {
		return fmt.Errorf("%w: invalid item or quantity", ErrInvalidRequest)
	}
	return nil
}

func (m *MockInstamartAPI) ClearCart(ctx context.Context) error { return nil }

func (m *MockInstamartAPI) RemoveItemsFromCart(ctx context.Context, items []UpdateCartRequest) error {
	return nil
}

func (m *MockInstamartAPI) Checkout(ctx context.Context, req CheckoutRequest) (*CheckoutResponse, error) {
	orderID, err := m.Mock.CheckoutInstamart(ctx, req.PaymentMethod, req.AddressID)
	if err != nil {
		return nil, err
	}
	return &CheckoutResponse{OrderID: orderID}, nil
}

func (m *MockInstamartAPI) ReportError(ctx context.Context, req ReportErrorRequest) error {
	return m.Mock.ReportError(ctx, "instamart", req.Code, req.Message, req.Context, nil, "")
}

// MockDineoutAPI satisfies DineoutClient using MockClient for local testing.
type MockDineoutAPI struct {
	Mock *MockClient
}

func (m *MockDineoutAPI) CreateCart(ctx context.Context, req CreateCartRequest) (*CreateCartResponse, error) {
	if req.RestaurantID == "" {
		return nil, fmt.Errorf("%w: restaurantId required", ErrInvalidRequest)
	}
	return &CreateCartResponse{CartID: m.Mock.generateID("CART")}, nil
}

// BookTable uses CartID as the restaurant identifier; guestCount defaults to 1
// since the mock only validates > 0 and guest data isn't in BookTableRequest.
func (m *MockDineoutAPI) BookTable(ctx context.Context, req BookTableRequest) (*BookTableResponse, error) {
	bookingID, err := m.Mock.BookTable(ctx, req.CartID, 0, req.SlotID, 0, 1, 0, 0)
	if err != nil {
		return nil, err
	}
	return &BookTableResponse{BookingID: bookingID}, nil
}

func (m *MockDineoutAPI) ReportError(ctx context.Context, req ReportErrorRequest) error {
	return m.Mock.ReportError(ctx, "dineout", req.Code, req.Message, req.Context, nil, "")
}
