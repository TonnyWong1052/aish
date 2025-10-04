package llm

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"
)

// HTTPClient provides a standardized HTTP client with retry logic for all LLM providers
type HTTPClient struct {
	client     *http.Client
	maxRetries int
	baseDelay  time.Duration
}

// NewHTTPClient creates a new HTTP client with retry capabilities
func NewHTTPClient(timeout time.Duration, maxRetries int) *HTTPClient {
	return &HTTPClient{
		client: &http.Client{
			Timeout: timeout,
		},
		maxRetries: maxRetries,
		baseDelay:  1 * time.Second,
	}
}

// Do executes an HTTP request with exponential backoff retry logic
func (c *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		// Clone the request for retry attempts
		reqClone := req.Clone(req.Context())

		// Execute the request
		resp, err := c.client.Do(reqClone)
		if err == nil {
			// Check for specific HTTP status codes that indicate success or non-retryable failure
			if resp.StatusCode < 500 {
				return resp, nil // Success or client error (don't retry client errors)
			}
			resp.Body.Close() // Close body for server errors (will retry)
			lastErr = fmt.Errorf("server error: status code %d", resp.StatusCode)
		} else {
			lastErr = err
		}

		// Don't wait after the last attempt
		if attempt == c.maxRetries {
			break
		}

		// Calculate delay with exponential backoff and jitter
		delay := c.calculateDelay(attempt)

		// Check if context is cancelled during delay
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(delay):
			// Continue with retry
		}
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", c.maxRetries+1, lastErr)
}

// calculateDelay computes the delay for exponential backoff with jitter
func (c *HTTPClient) calculateDelay(attempt int) time.Duration {
	// Exponential backoff: baseDelay * 2^attempt
	delay := float64(c.baseDelay) * math.Pow(2, float64(attempt))

	// Cap the delay at 30 seconds
	if delay > 30*float64(time.Second) {
		delay = 30 * float64(time.Second)
	}

	// Add jitter (±25% of the delay)
	jitter := delay * 0.25 * (2*float64(time.Now().UnixNano()%1000)/1000 - 1)

	return time.Duration(delay + jitter)
}

// HealthCheck performs a basic connectivity check
func (c *HTTPClient) HealthCheck(ctx context.Context, url string, headers map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	// Add headers
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("health check failed with status %d", resp.StatusCode)
	}

	return nil
}
