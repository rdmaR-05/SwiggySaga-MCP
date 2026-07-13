package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"swiggy-saga-mcp/internal/locking"
	"swiggy-saga-mcp/internal/swiggy"
	"swiggy-saga-mcp/internal/workflows"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
)

// API holds handlers for the saga orchestration endpoints.
type API struct {
	foodWorkflow      *workflows.FoodOrderWorkflow
	instamartWorkflow *workflows.InstamartCheckoutWorkflow
	dineoutWorkflow   *workflows.DineoutBookingWorkflow
	locker            locking.Locker
	validate          *validator.Validate
	redisClient       *redis.Client
}

func NewAPI(
	food *workflows.FoodOrderWorkflow,
	instamart *workflows.InstamartCheckoutWorkflow,
	dineout *workflows.DineoutBookingWorkflow,
	locker locking.Locker,
	validate *validator.Validate,
	redisClient *redis.Client,
) *API {
	return &API{
		foodWorkflow:      food,
		instamartWorkflow: instamart,
		dineoutWorkflow:   dineout,
		locker:            locker,
		validate:          validate,
		redisClient:       redisClient,
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func (api *API) HandleFoodOrder(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("swiggy.saga.mcp").Start(r.Context(), "HandleFoodOrder")
	defer span.End()

	var req workflows.FoodOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := api.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("validation error: %w", err))
		return
	}

	// prevent concurrent orders on the same address
	acquired, err := api.locker.AcquireLock(r.Context(), "food_"+req.AddressID, 30*time.Second)
	if err != nil || !acquired {
		writeError(w, http.StatusTooManyRequests, locking.ErrLockFailed)
		return
	}
	defer api.locker.ReleaseLock(context.Background(), "food_"+req.AddressID) // Background context for guaranteed release

	if err := api.foodWorkflow.Execute(ctx, req); err != nil {
		if _, ok := err.(*swiggy.APIError); ok {
			writeError(w, http.StatusBadGateway, err)
		} else {
			writeError(w, http.StatusInternalServerError, err)
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Food order saga completed successfully"})
}

func (api *API) HandleInstamartCheckout(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("swiggy.saga.mcp").Start(r.Context(), "HandleInstamartCheckout")
	defer span.End()

	var req workflows.InstamartCheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := api.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("validation error: %w", err))
		return
	}

	lockKey := req.AddressID
	if lockKey == "" {
		lockKey = "new_address_session" // Fallback if creating a new address without UserID
	}

	// Distributed Lock
	acquired, err := api.locker.AcquireLock(r.Context(), "instamart_"+lockKey, 30*time.Second)
	if err != nil || !acquired {
		writeError(w, http.StatusTooManyRequests, locking.ErrLockFailed)
		return
	}
	defer api.locker.ReleaseLock(context.Background(), "instamart_"+lockKey)

	if err := api.instamartWorkflow.Execute(ctx, req); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Instamart checkout saga completed successfully"})
}

func (api *API) HandleDineoutBooking(w http.ResponseWriter, r *http.Request) {
	ctx, span := otel.Tracer("swiggy.saga.mcp").Start(r.Context(), "HandleDineoutBooking")
	defer span.End()

	var req workflows.DineoutBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if err := api.validate.Struct(req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("validation error: %w", err))
		return
	}

	// Lock the exact restaurant slot to prevent double booking concurrently
	lockKey := fmt.Sprintf("dineout_%s_%s", req.RestaurantID, req.SlotID)
	
	acquired, err := api.locker.AcquireLock(r.Context(), lockKey, 30*time.Second)
	if err != nil || !acquired {
		writeError(w, http.StatusTooManyRequests, locking.ErrLockFailed)
		return
	}
	defer api.locker.ReleaseLock(context.Background(), lockKey)

	if err := api.dineoutWorkflow.Execute(ctx, req); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Dineout booking saga completed successfully"})
}

// RegisterRoutes registers all endpoints on the provided ServeMux.
func (api *API) RegisterRoutes(mux *http.ServeMux) {
	idempMw := IdempotencyMiddleware(api.redisClient)

	mux.HandleFunc("POST /api/v1/orchestrate/food-order", idempMw(api.HandleFoodOrder))
	mux.HandleFunc("POST /api/v1/orchestrate/instamart-checkout", idempMw(api.HandleInstamartCheckout))
	mux.HandleFunc("POST /api/v1/orchestrate/dineout-booking", idempMw(api.HandleDineoutBooking))
	mux.HandleFunc("POST /api/v1/webhooks/swiggy", api.HandleSwiggyWebhook)
}

type WebhookPayload struct {
	SagaID       string `json:"sagaId"`
	Status       string `json:"status"`
	RestaurantID string `json:"restaurantId"`
	SlotID       string `json:"slotId"`
	Guests       int    `json:"guests"`
}

func (api *API) HandleSwiggyWebhook(w http.ResponseWriter, r *http.Request) {
	var payload WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	if payload.Status == "CONFIRMED" && payload.SagaID != "" {
		req := workflows.DineoutBookingRequest{
			RestaurantID: payload.RestaurantID,
			SlotID:       payload.SlotID,
			Guests:       payload.Guests,
		}
		
		go func() {
			err := api.dineoutWorkflow.Resume(context.Background(), payload.SagaID, req)
			if err != nil {
				slog.Error("Failed to resume saga from webhook", "saga_id", payload.SagaID, "error", err)
			}
		}()
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status":"accepted"}`))
}
