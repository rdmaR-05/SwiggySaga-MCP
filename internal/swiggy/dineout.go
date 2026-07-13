package swiggy

import "context"

// DineoutAPI wraps APIClient with typed methods for the Dineout domain.
type DineoutAPI struct {
	client *APIClient
}

func NewDineoutAPI(client *APIClient) *DineoutAPI {
	return &DineoutAPI{client: client}
}

func (d *DineoutAPI) CreateCart(ctx context.Context, req CreateCartRequest) (*CreateCartResponse, error) {
	var resp CreateCartResponse
	payload := MCPRequestWrapper{Name: "create_cart", Arguments: req}
	if err := d.client.BasePost(ctx, "/dineout", payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (d *DineoutAPI) BookTable(ctx context.Context, req BookTableRequest) (*BookTableResponse, error) {
	var resp BookTableResponse
	payload := MCPRequestWrapper{Name: "book_table", Arguments: req}
	if err := d.client.BasePost(ctx, "/dineout", payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (d *DineoutAPI) ReportError(ctx context.Context, req ReportErrorRequest) error {
	payload := MCPRequestWrapper{Name: "report_error", Arguments: req}
	return d.client.BasePost(ctx, "/dineout", payload, nil)
}
