package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"swiggy-saga-mcp/internal/handlers"
	"swiggy-saga-mcp/internal/locking"
	"swiggy-saga-mcp/internal/saga"
	"swiggy-saga-mcp/internal/swiggy"
	"swiggy-saga-mcp/internal/telemetry"
	"swiggy-saga-mcp/internal/workflows"

	"github.com/go-playground/validator/v10"
	"github.com/redis/go-redis/v9"
)

func main() {
	shutdownOTEL, err := telemetry.InitTracer()
	if err != nil {
		slog.Error("Failed to initialize OpenTelemetry", "error", err)
	} else {
		defer shutdownOTEL(context.Background())
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	// USE_MOCK_CLIENT=true to run without real MCP credentials.
	// MOCK_FOOD_FAIL_RATE=20 (0-100) injects random failures for rollback testing.
	var foodAPI swiggy.FoodClient
	var instamartAPI swiggy.InstamartClient
	var dineoutAPI swiggy.DineoutClient

	if os.Getenv("USE_MOCK_CLIENT") == "true" {
		failRate := 0
		if v := os.Getenv("MOCK_FOOD_FAIL_RATE"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 100 {
				failRate = n
			}
		}
		slog.Warn("USE_MOCK_CLIENT=true — using in-memory MockClient (NOT for production!)",
			"food_fail_rate", failRate)
		mock := swiggy.NewMockClient(failRate)
		foodAPI = &swiggy.MockFoodAPI{Mock: mock}
		instamartAPI = &swiggy.MockInstamartAPI{Mock: mock}
		dineoutAPI = &swiggy.MockDineoutAPI{Mock: mock}
	} else {
		swiggyBaseURL := os.Getenv("SWIGGY_MCP_BASE_URL")
		if swiggyBaseURL == "" {
			swiggyBaseURL = "https://mcp.swiggy.com"
		}
		swiggyToken := os.Getenv("SWIGGY_MCP_TOKEN")
		realClient := swiggy.NewAPIClient(swiggyBaseURL, swiggyToken)
		foodAPI = swiggy.NewFoodAPI(realClient)
		instamartAPI = swiggy.NewInstamartAPI(realClient)
		dineoutAPI = swiggy.NewDineoutAPI(realClient)
		slog.Info("Using real Swiggy MCP client", "base_url", os.Getenv("SWIGGY_MCP_BASE_URL"))
	}

	// Initialize Redis Locker and Store
	redisAddr := os.Getenv("REDIS_ADDR")
	var locker locking.Locker
	var redisClient *redis.Client
	var store saga.Store

	if redisAddr != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr: redisAddr,
		})
		locker = locking.NewRedisLocker(redisClient)
		store = saga.NewRedisStore(redisClient)
		slog.Info("Initialized Redis Distributed Locker & Idempotency", "addr", redisAddr)

		// Start Recovery Daemon
		go StartRecoveryDaemon(context.Background(), store, redisClient)
	} else {
		locker = &locking.NoOpLocker{}
		store = &saga.NoOpStore{}
		slog.Warn("REDIS_ADDR not set, falling back to NoOp components (Not for Production!)")
	}

	// Initialize Saga Workflows
	foodWorkflow := workflows.NewFoodOrderWorkflow(foodAPI, store)
	instamartWorkflow := workflows.NewInstamartCheckoutWorkflow(instamartAPI, store)
	dineoutWorkflow := workflows.NewDineoutBookingWorkflow(dineoutAPI, store)

	// Initialize Validator
	validate := validator.New()

	// Webhook secret — if unset, webhook auth is disabled; log a warning.
	webhookSecret := os.Getenv("SWIGGY_WEBHOOK_SECRET")
	if webhookSecret == "" {
		slog.Warn("SWIGGY_WEBHOOK_SECRET not set; webhook endpoint is unauthenticated")
	}

	// Initialize Handlers
	api := handlers.NewAPI(foodWorkflow, instamartWorkflow, dineoutWorkflow, locker, validate, redisClient, webhookSecret)

	mux := http.NewServeMux()
	api.RegisterRoutes(mux)

	// Health check — returns degraded if Redis is configured but unreachable.
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if redisClient != nil {
			if err := redisClient.Ping(r.Context()).Err(); err != nil {
				slog.Warn("Health check: Redis unreachable", "error", err)
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte(`{"status":"degraded","redis":"down"}`))
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Layer middleware: panic recovery → 1 MB body cap → 30 s hard deadline.
	handler := handlers.PanicRecoveryMiddleware(http.MaxBytesHandler(mux, 1<<20))
	handler = http.TimeoutHandler(handler, 30*time.Second, `{"error":"request timeout"}`)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,  // mitigate Slowloris
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      35 * time.Second, // > TimeoutHandler duration to avoid race
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("Server is starting", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Could not listen on addr", "addr", server.Addr, "error", err)
			os.Exit(1)
		}
	}()

	<-stop
	slog.Info("Server is shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Server exited gracefully")
}
