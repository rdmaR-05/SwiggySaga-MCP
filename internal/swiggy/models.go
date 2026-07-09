package swiggy

import "encoding/json"

// BaseResponse represents the standard Swiggy MCP response structure.
type BaseResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Message string          `json:"message,omitempty"`
	Error   *ErrorDetails   `json:"error,omitempty"`
}

// ErrorDetails represents the error structure when success is false.
type ErrorDetails struct {
	Message    string `json:"message"`
	Code       string `json:"code,omitempty"` // Planned for roadmap
	ReportLink string `json:"reportLink,omitempty"`
	ReportHint string `json:"reportHint,omitempty"`
}

// Food Models
type ApplyFoodCouponRequest struct {
	CouponCode string `json:"couponCode"`
}

type ApplyFoodCouponResponse struct {
	CartTotal float64 `json:"cartTotal"`
	Discount  float64 `json:"discount"`
}

type PlaceFoodOrderRequest struct {
	AddressID     string `json:"addressId"`
	PaymentMethod string `json:"paymentMethod,omitempty"`
}

type PlaceFoodOrderResponse struct {
	OrderID       string `json:"orderId"`
	Status        string `json:"status"`
	PaymentMethod string `json:"paymentMethod"`
}

type ReportErrorRequest struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Context string `json:"context,omitempty"`
}

// Instamart Models
type CreateAddressRequest struct {
	Label    string `json:"label"`
	Address  string `json:"address"`
	Landmark string `json:"landmark,omitempty"`
}

type CreateAddressResponse struct {
	AddressID string `json:"addressId"`
}

type DeleteAddressRequest struct {
	AddressID string `json:"addressId"`
}

type UpdateCartRequest struct {
	ItemID   string `json:"itemId" validate:"required"`
	Quantity int    `json:"quantity" validate:"required,min=1"`
}

type CheckoutRequest struct {
	AddressID     string `json:"addressId"`
	PaymentMethod string `json:"paymentMethod,omitempty"`
}

type CheckoutResponse struct {
	OrderID string `json:"orderId"`
}

// Dineout Models
type CreateCartRequest struct {
	RestaurantID string `json:"restaurantId"`
	Guests       int    `json:"guests"`
}

type CreateCartResponse struct {
	CartID string `json:"cartId"`
}

type BookTableRequest struct {
	CartID string `json:"cartId"`
	SlotID string `json:"slotId"`
}

type BookTableResponse struct {
	BookingID string `json:"bookingId"`
}
