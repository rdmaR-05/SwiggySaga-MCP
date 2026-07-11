package swiggy

import "context"

// InstamartAPI provides strongly-typed bindings for the Swiggy Instamart grocery domain.
type InstamartAPI struct {
	client *APIClient
}

func NewInstamartAPI(client *APIClient) *InstamartAPI {
	return &InstamartAPI{client: client}
}

func (i *InstamartAPI) CreateAddress(ctx context.Context, req CreateAddressRequest) (*CreateAddressResponse, error) {
	var resp CreateAddressResponse
	payload := MCPRequestWrapper{Name: "create_address", Arguments: req}
	if err := i.client.BasePost(ctx, "/instamart", payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (i *InstamartAPI) DeleteAddress(ctx context.Context, req DeleteAddressRequest) error {
	payload := MCPRequestWrapper{Name: "delete_address", Arguments: req}
	return i.client.BasePost(ctx, "/instamart", payload, nil)
}

func (i *InstamartAPI) UpdateCart(ctx context.Context, req UpdateCartRequest) error {
	payload := MCPRequestWrapper{Name: "update_cart", Arguments: req}
	return i.client.BasePost(ctx, "/instamart", payload, nil)
}

func (i *InstamartAPI) ClearCart(ctx context.Context) error {
	payload := MCPRequestWrapper{Name: "clear_cart", Arguments: map[string]interface{}{}}
	return i.client.BasePost(ctx, "/instamart", payload, nil)
}

// RemoveItemsFromCart removes specific items from the instamart cart (Delta Compensation).
func (i *InstamartAPI) RemoveItemsFromCart(ctx context.Context, items []UpdateCartRequest) error {
	payload := MCPRequestWrapper{Name: "remove_items_from_cart", Arguments: map[string]interface{}{"items": items}}
	return i.client.BasePost(ctx, "/instamart", payload, nil)
}

func (i *InstamartAPI) Checkout(ctx context.Context, req CheckoutRequest) (*CheckoutResponse, error) {
	var resp CheckoutResponse
	payload := MCPRequestWrapper{Name: "checkout", Arguments: req}
	if err := i.client.BasePost(ctx, "/instamart", payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (i *InstamartAPI) ReportError(ctx context.Context, req ReportErrorRequest) error {
	payload := MCPRequestWrapper{Name: "report_error", Arguments: req}
	return i.client.BasePost(ctx, "/instamart", payload, nil)
}
