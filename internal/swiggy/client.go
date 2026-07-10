package swiggy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sony/gobreaker"
)

var (
	ErrNetworkTimeout = errors.New("network timeout occurred while waiting for upstream response")
)

// APIClient is the Swiggy API client for executing MCP tools natively in Go.
type APIClient struct {
	httpClient *http.Client
	baseURL    string
	token      string
	breaker    *gobreaker.CircuitBreaker
}

// NewAPIClient initializes a new Swiggy API Client.
// Usually the MCP token is passed around contextually, but for simplicity we bind it to the client for this middleware flow.
func NewAPIClient(baseURL, token string) *APIClient {
	st := gobreaker.Settings{
		Name:        "SwiggyAPI",
		MaxRequests: 3,                // Max requests allowed in half-open state
		Interval:    10 * time.Second, // Time to keep counts
		Timeout:     30 * time.Second, // Time circuit stays open before half-open
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			// Trip if 5 consecutive failures
			return counts.ConsecutiveFailures >= 5
		},
	}

	return &APIClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: baseURL,
		token:   token,
		breaker: gobreaker.NewCircuitBreaker(st),
	}
}

// BasePost executes a mutating POST request to the Swiggy endpoint.
func (c *APIClient) BasePost(ctx context.Context, endpoint string, payload interface{}, out interface{}) error {
	// Execute via circuit breaker
	_, err := c.breaker.Execute(func() (interface{}, error) {
		url := fmt.Sprintf("%s%s", c.baseURL, endpoint)

		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request payload: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		if c.token != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.token))
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
				return nil, ErrNetworkTimeout
			}
			return nil, fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		var baseResp BaseResponse
		respBodyBytes, _ := io.ReadAll(resp.Body)
		if len(respBodyBytes) > 0 {
			json.Unmarshal(respBodyBytes, &baseResp)
		}

		if resp.StatusCode >= 400 || !baseResp.Success {
			return nil, parseSwiggyError(resp.StatusCode, baseResp)
		}

		// If there's an expected out object, unmarshal the inner data payload
		if out != nil && len(baseResp.Data) > 0 {
			if err := json.Unmarshal(baseResp.Data, out); err != nil {
				return nil, fmt.Errorf("failed to unmarshal success payload: %w", err)
			}
		}

		return nil, nil
	})

	if err != nil {
		if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
			return &APIError{Code: "CIRCUIT_OPEN", Message: "Swiggy API is temporarily unavailable"}
		}
		return err
	}
	return nil
}

func parseSwiggyError(statusCode int, baseResp BaseResponse) *APIError {
	// If the planned error.code ships, use it.
	if baseResp.Error != nil && baseResp.Error.Code != "" {
		return &APIError{Code: baseResp.Error.Code, Message: baseResp.Error.Message}
	}

	msg := ""
	if baseResp.Error != nil {
		msg = baseResp.Error.Message
	}

	// Classify by HTTP status and message prefix
	if statusCode == 401 {
		return &APIError{Code: ErrCodeUnauthenticated, Message: msg}
	}
	if statusCode == 400 {
		if strings.HasPrefix(msg, "Invalid") || strings.HasPrefix(msg, "Missing") {
			return &APIError{Code: ErrCodeValidation, Message: msg}
		}
	}
	if statusCode == 504 || strings.Contains(strings.ToLower(msg), "timeout") {
		return &APIError{Code: ErrCodeUpstreamTimeout, Message: msg}
	}
	if statusCode == 502 || statusCode == 503 {
		return &APIError{Code: ErrCodeUpstreamError, Message: msg}
	}
	if statusCode >= 500 {
		return &APIError{Code: ErrCodeInternalError, Message: msg}
	}
	if !baseResp.Success {
		// Domain failure
		return &APIError{Code: "DOMAIN_FAILURE", Message: msg}
	}

	return &APIError{Message: fmt.Sprintf("Unknown error (HTTP %d)", statusCode)}
}
