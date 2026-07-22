package handlers_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"swiggy-saga-mcp/internal/handlers"
	"swiggy-saga-mcp/internal/locking"
	"swiggy-saga-mcp/internal/saga"
	"swiggy-saga-mcp/internal/swiggy"
	"swiggy-saga-mcp/internal/workflows"

	"github.com/go-playground/validator/v10"
)

func newTestAPI(webhookSecret string) (*handlers.API, *http.ServeMux) {
	mock := swiggy.NewMockClient(0)
	food := workflows.NewFoodOrderWorkflow(&swiggy.MockFoodAPI{Mock: mock}, &saga.NoOpStore{})
	instamart := workflows.NewInstamartCheckoutWorkflow(&swiggy.MockInstamartAPI{Mock: mock}, &saga.NoOpStore{})
	dineout := workflows.NewDineoutBookingWorkflow(&swiggy.MockDineoutAPI{Mock: mock}, &saga.NoOpStore{})

	api := handlers.NewAPI(food, instamart, dineout, &locking.NoOpLocker{}, validator.New(), nil, webhookSecret)
	mux := http.NewServeMux()
	api.RegisterRoutes(mux)
	return api, mux
}

func TestHandleFoodOrder_Success(t *testing.T) {
	_, mux := newTestAPI("")
	body, _ := json.Marshal(map[string]string{
		"addressId":     "addr-1",
		"paymentMethod": "UPI",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orchestrate/food-order", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleFoodOrder_MissingPaymentMethod_400(t *testing.T) {
	_, mux := newTestAPI("")
	body, _ := json.Marshal(map[string]string{
		"addressId": "addr-1",
		// paymentMethod omitted
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orchestrate/food-order", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing paymentMethod, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleFoodOrder_MissingAddressID_400(t *testing.T) {
	_, mux := newTestAPI("")
	body, _ := json.Marshal(map[string]string{
		"paymentMethod": "UPI",
		// addressId omitted
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orchestrate/food-order", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing addressId, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleFoodOrder_InvalidJSON_400(t *testing.T) {
	_, mux := newTestAPI("")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orchestrate/food-order", bytes.NewReader([]byte(`not json`)))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for malformed JSON, got %d", rr.Code)
	}
}

func TestHandleInstamartCheckout_Success(t *testing.T) {
	_, mux := newTestAPI("")
	body, _ := json.Marshal(map[string]interface{}{
		"items":         []map[string]interface{}{{"itemId": "item-1", "quantity": 2}},
		"addressId":     "addr-1",
		"paymentMethod": "CARD",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orchestrate/instamart-checkout", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleDineoutBooking_Success(t *testing.T) {
	_, mux := newTestAPI("")
	body, _ := json.Marshal(map[string]interface{}{
		"restaurantId": "rest-1",
		"guests":       2,
		"slotId":       "slot-7pm",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/orchestrate/dineout-booking", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// ---- Webhook HMAC tests ----

func signPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestHandleWebhook_ValidSignature_Accepted(t *testing.T) {
	secret := "test-secret-key"
	_, mux := newTestAPI(secret)

	payload, _ := json.Marshal(map[string]interface{}{
		"sagaId": "saga-123",
		"status": "CONFIRMED",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/swiggy", bytes.NewReader(payload))
	req.Header.Set("X-Swiggy-Signature", signPayload(secret, payload))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleWebhook_InvalidSignature_401(t *testing.T) {
	_, mux := newTestAPI("real-secret")

	payload, _ := json.Marshal(map[string]interface{}{
		"sagaId": "saga-123",
		"status": "CONFIRMED",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/swiggy", bytes.NewReader(payload))
	req.Header.Set("X-Swiggy-Signature", "sha256=deadbeef")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestHandleWebhook_MissingSignature_401(t *testing.T) {
	_, mux := newTestAPI("real-secret")

	payload, _ := json.Marshal(map[string]interface{}{
		"sagaId": "saga-123",
		"status": "CONFIRMED",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/swiggy", bytes.NewReader(payload))
	// no X-Swiggy-Signature header
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestHandleWebhook_NoSecret_PassesThrough(t *testing.T) {
	_, mux := newTestAPI("") // empty = auth disabled

	payload, _ := json.Marshal(map[string]interface{}{
		"sagaId": "saga-123",
		"status": "CONFIRMED",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/swiggy", bytes.NewReader(payload))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Errorf("expected 202 when no secret configured, got %d", rr.Code)
	}
}
