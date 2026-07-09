package swiggy

import "fmt"

// APIError represents an error returned by the Swiggy API.
type APIError struct {
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("swiggy api error [%s]: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("swiggy api error: %s", e.Message)
}

// Core error codes (planned)
const (
	ErrCodeUnauthenticated = "UNAUTHENTICATED"
	ErrCodeTokenExpired    = "TOKEN_EXPIRED"
	ErrCodeSessionRevoked  = "SESSION_REVOKED"
	ErrCodeInsufficientScope = "INSUFFICIENT_SCOPE"
	ErrCodeRateLimited     = "RATE_LIMITED"
	ErrCodeValidation      = "VALIDATION_ERROR"
	ErrCodeNotFound        = "NOT_FOUND"
	ErrCodeUpstreamTimeout = "UPSTREAM_TIMEOUT"
	ErrCodeUpstreamError   = "UPSTREAM_ERROR"
	ErrCodeInternalError   = "INTERNAL_ERROR"

	// Domain codes: Instamart
	ErrCodeItemOutOfStock       = "ITEM_OUT_OF_STOCK"
	ErrCodeCartExpired          = "CART_EXPIRED"
	ErrCodeAddressNotServiceable = "ADDRESS_NOT_SERVICEABLE"
	ErrCodeMinOrderNotMet       = "MIN_ORDER_NOT_MET"

	// Domain codes: Food
	ErrCodeRestaurantClosed           = "RESTAURANT_CLOSED"
	ErrCodeItemUnavailable            = "ITEM_UNAVAILABLE"
	ErrCodeCouponInvalid              = "COUPON_INVALID"
	ErrCodeCouponNotApplicable        = "COUPON_NOT_APPLICABLE"
	ErrCodeCouponRequiresOnlinePayment = "COUPON_REQUIRES_ONLINE_PAYMENT"

	// Domain codes: Dineout
	ErrCodeSlotUnavailable       = "SLOT_UNAVAILABLE"
	ErrCodeRestaurantNotBookable = "RESTAURANT_NOT_BOOKABLE"
	ErrCodeBookingWindowClosed   = "BOOKING_WINDOW_CLOSED"
)
