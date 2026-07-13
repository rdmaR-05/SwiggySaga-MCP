package handlers

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// responseRecorder captures the status code and body written by a handler.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // default
		body:           &bytes.Buffer{},
	}
}

func (rw *responseRecorder) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseRecorder) Write(b []byte) (int, error) {
	rw.body.Write(b)
	return rw.ResponseWriter.Write(b)
}

// IdempotencyMiddleware short-circuits duplicate requests using a Redis-backed response cache.
// Callers that don't send an Idempotency-Key header, or when Redis is unavailable, pass through unchanged.
func IdempotencyMiddleware(redisClient *redis.Client) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			idempKey := r.Header.Get("Idempotency-Key")
			if idempKey == "" || redisClient == nil {
				next(w, r)
				return
			}

			cacheKey := "idemp:" + idempKey

			cachedResp, err := redisClient.Get(r.Context(), cacheKey).Result()
			if err == nil {
				slog.Info("Idempotency hit, returning cached response", "key", idempKey)
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Idempotent-Cache", "HIT")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(cachedResp))
				return
			} else if err != redis.Nil {
				// degrade gracefully; don't block the request on a Redis hiccup
				slog.Error("Redis error during idempotency check", "error", err)
			}

			recorder := newResponseRecorder(w)
			next(recorder, r)

			if recorder.statusCode == http.StatusOK {
				err := redisClient.Set(context.Background(), cacheKey, recorder.body.Bytes(), 24*time.Hour).Err()
				if err != nil {
					slog.Error("Failed to cache idempotent response", "error", err)
				}
			}
		}
	}
}
