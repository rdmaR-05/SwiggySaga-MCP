package swiggy

import "context"

// MCPRequestWrapper is the envelope for Swiggy MCP tool calls.
type MCPRequestWrapper struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"`
}

// FoodAPI wraps APIClient with typed methods for the Food domain.
type FoodAPI struct {
	client *APIClient
}

func NewFoodAPI(client *APIClient) *FoodAPI {
	return &FoodAPI{client: client}
}

func (f *FoodAPI) ApplyCoupon(ctx context.Context, req ApplyFoodCouponRequest) (*ApplyFoodCouponResponse, error) {
	var resp ApplyFoodCouponResponse
	payload := MCPRequestWrapper{Name: "apply_food_coupon", Arguments: req}
	if err := f.client.BasePost(ctx, "/food", payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (f *FoodAPI) FlushCart(ctx context.Context) error {
	payload := MCPRequestWrapper{Name: "flush_food_cart", Arguments: map[string]interface{}{}}
	return f.client.BasePost(ctx, "/food", payload, nil)
}

func (f *FoodAPI) PlaceOrder(ctx context.Context, req PlaceFoodOrderRequest) (*PlaceFoodOrderResponse, error) {
	var resp PlaceFoodOrderResponse
	payload := MCPRequestWrapper{Name: "place_food_order", Arguments: req}
	if err := f.client.BasePost(ctx, "/food", payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (f *FoodAPI) ReportError(ctx context.Context, req ReportErrorRequest) error {
	payload := MCPRequestWrapper{Name: "report_error", Arguments: req}
	return f.client.BasePost(ctx, "/food", payload, nil)
}
