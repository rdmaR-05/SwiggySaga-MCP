package swiggy

import "context"

// FoodClient defines the contract for Food domain operations used by FoodOrderWorkflow.
// Both FoodAPI (real HTTP) and MockFoodAPI (in-memory mock) satisfy this interface.
type FoodClient interface {
	ApplyCoupon(ctx context.Context, req ApplyFoodCouponRequest) (*ApplyFoodCouponResponse, error)
	FlushCart(ctx context.Context) error
	PlaceOrder(ctx context.Context, req PlaceFoodOrderRequest) (*PlaceFoodOrderResponse, error)
	ReportError(ctx context.Context, req ReportErrorRequest) error
}

// InstamartClient defines the contract for Instamart domain operations used by InstamartCheckoutWorkflow.
type InstamartClient interface {
	CreateAddress(ctx context.Context, req CreateAddressRequest) (*CreateAddressResponse, error)
	DeleteAddress(ctx context.Context, req DeleteAddressRequest) error
	UpdateCart(ctx context.Context, req UpdateCartRequest) error
	ClearCart(ctx context.Context) error
	RemoveItemsFromCart(ctx context.Context, items []UpdateCartRequest) error
	Checkout(ctx context.Context, req CheckoutRequest) (*CheckoutResponse, error)
	ReportError(ctx context.Context, req ReportErrorRequest) error
}

// DineoutClient defines the contract for Dineout domain operations used by DineoutBookingWorkflow.
type DineoutClient interface {
	CreateCart(ctx context.Context, req CreateCartRequest) (*CreateCartResponse, error)
	BookTable(ctx context.Context, req BookTableRequest) (*BookTableResponse, error)
	ReportError(ctx context.Context, req ReportErrorRequest) error
}
