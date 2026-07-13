package swiggy

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrInvalidRequest     = errors.New("swiggy mcp: 400 bad request (invalid input)")
	ErrNotFound           = errors.New("swiggy mcp: 404 resource not found")
	ErrInventoryConflict  = errors.New("swiggy mcp: 409 conflict (out of stock)")
	ErrServiceUnavailable = errors.New("swiggy mcp: 503 service unavailable")
	ErrPaymentTimeout     = errors.New("swiggy mcp: 504 gateway timeout")
)

// MockClient is a stateful in-memory simulation of the Swiggy MCP APIs.
// Use USE_MOCK_CLIENT=true in main to activate it without real credentials.
type MockClient struct {
	mu           sync.RWMutex
	foodFailRate int

	activeBookings map[string]bool
	activeOrders   map[string]bool
	activePayments map[string]float64

	counter uint64
}

func NewMockClient(foodFailRate int) *MockClient {
	return &MockClient{
		foodFailRate:   foodFailRate,
		activeBookings: make(map[string]bool),
		activeOrders:   make(map[string]bool),
		activePayments: make(map[string]float64),
	}
}

func (m *MockClient) generateID(prefix string) string {
	val := atomic.AddUint64(&m.counter, 1)
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixMilli(), val)
}

func (m *MockClient) BookTable(ctx context.Context, restaurantID string, slotID int, itemID string, reservationTime int64, guestCount int, lat float64, lng float64) (string, error) {
	time.Sleep(200 * time.Millisecond)
	if restaurantID == "" || guestCount <= 0 {
		return "", fmt.Errorf("%w: invalid restaurant or guest count", ErrInvalidRequest)
	}
	bookingID := m.generateID("DO")
	m.mu.Lock()
	m.activeBookings[bookingID] = true
	m.mu.Unlock()
	return bookingID, nil
}

func (m *MockClient) CancelBooking(ctx context.Context, bookingID string) error {
	time.Sleep(150 * time.Millisecond)
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.activeBookings[bookingID] {
		return fmt.Errorf("cannot cancel %s: %w", bookingID, ErrNotFound)
	}
	delete(m.activeBookings, bookingID)
	return nil
}

func (m *MockClient) PlaceFoodOrder(ctx context.Context, paymentMethod string, addressId string) (string, error) {
	time.Sleep(300 * time.Millisecond)
	if paymentMethod == "" {
		return "", fmt.Errorf("%w: payment method required", ErrInvalidRequest)
	}
	// Inject chaos proportional to foodFailRate (0-100).
	if (time.Now().UnixNano() % 100) < int64(m.foodFailRate) {
		return "", ErrInventoryConflict
	}
	orderID := m.generateID("SF")
	m.mu.Lock()
	m.activeOrders[orderID] = true
	m.mu.Unlock()
	return orderID, nil
}

func (m *MockClient) CheckoutInstamart(ctx context.Context, paymentMethod string, addressId string) (string, error) {
	time.Sleep(250 * time.Millisecond)
	if paymentMethod == "" {
		return "", fmt.Errorf("%w: payment method required", ErrInvalidRequest)
	}
	orderID := m.generateID("IM")
	m.mu.Lock()
	m.activeOrders[orderID] = true
	m.mu.Unlock()
	return orderID, nil
}

func (m *MockClient) CancelOrder(ctx context.Context, orderID string) error {
	time.Sleep(150 * time.Millisecond)
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.activeOrders[orderID] {
		return fmt.Errorf("cannot cancel %s: %w", orderID, ErrNotFound)
	}
	delete(m.activeOrders, orderID)
	return nil
}

func (m *MockClient) ReportError(ctx context.Context, tool string, domain string, errorMessage string, flowDescription string, toolContext map[string]interface{}, userNotes string) error {
	return nil
}

func (m *MockClient) InitiateUPIPayment(ctx context.Context, referenceID string, amountINR float64) (string, error) {
	time.Sleep(600 * time.Millisecond)
	if amountINR <= 0 {
		return "", fmt.Errorf("%w: amount must be positive", ErrInvalidRequest)
	}
	txnID := m.generateID("TXN-UPI")
	m.mu.Lock()
	m.activePayments[txnID] = amountINR
	m.mu.Unlock()
	return txnID, nil
}

func (m *MockClient) TriggerRefund(ctx context.Context, transactionID string, amountINR float64, reason string) error {
	time.Sleep(400 * time.Millisecond)
	m.mu.Lock()
	defer m.mu.Unlock()
	originalAmount, exists := m.activePayments[transactionID]
	if !exists {
		return fmt.Errorf("refund failed for %s: %w", transactionID, ErrNotFound)
	}
	if amountINR > originalAmount {
		return fmt.Errorf("%w: refund exceeds original transaction amount", ErrInvalidRequest)
	}
	m.activePayments[transactionID] -= amountINR
	return nil
}