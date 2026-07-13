package swiggy

import (
	"context"
)

type Client interface {

	BookTable(ctx context.Context, restaurantID string, slotID int, itemID string, reservationTime int64, guestCount int, lat float64, lng float64) (bookingID string, err error)

	CancelBooking(ctx context.Context, bookingID string) error

	PlaceFoodOrder(ctx context.Context, paymentMethod string, addressId string) (orderID string, err error)

	CheckoutInstamart(ctx context.Context, paymentMethod string, addressId string) (orderID string, err error)

	CancelOrder(ctx context.Context, orderID string) error

	ReportError(ctx context.Context, tool string, domain string, errorMessage string, flowDescription string, toolContext map[string]interface{}, userNotes string) error

	InitiateUPIPayment(ctx context.Context, referenceID string, amountINR float64) (transactionID string, err error)

	TriggerRefund(ctx context.Context, transactionID string, amountINR float64, reason string) error
}
