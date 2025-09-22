package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/config"
)

func TestIntegration_RateLimitedClient(t *testing.T) {
	// Test configuration with tight rate limiting for quick verification
	cfg := &config.Config{
		RateLimitDelay:         50 * time.Millisecond,
		MaxConcurrentRequests:  2,
		ExponentialBackoffBase: 100 * time.Millisecond,
		MaxBackoffDelay:        1 * time.Second,
	}

	// Create a test server that tracks request timing
	var requestTimes []time.Time
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestTimes = append(requestTimes, time.Now())
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Create rate limiter and transport
	rateLimiter := NewRateLimiter(cfg)
	transport := NewRateLimitedTransport(nil, rateLimiter)

	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	// Make multiple requests quickly
	ctx := context.Background()
	start := time.Now()

	for i := 0; i < 3; i++ {
		req, err := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
		if err != nil {
			t.Fatalf("Failed to create request %d: %v", i, err)
		}

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i, err)
		}
		_ = resp.Body.Close()
	}

	totalTime := time.Since(start)

	// Verify rate limiting worked
	if len(requestTimes) != 3 {
		t.Fatalf("Expected 3 requests, got %d", len(requestTimes))
	}

	// First request should be immediate, subsequent ones should be delayed
	if requestTimes[1].Sub(requestTimes[0]) < 45*time.Millisecond {
		t.Errorf("Second request was not properly rate limited: %v",
			requestTimes[1].Sub(requestTimes[0]))
	}

	if requestTimes[2].Sub(requestTimes[1]) < 45*time.Millisecond {
		t.Errorf("Third request was not properly rate limited: %v",
			requestTimes[2].Sub(requestTimes[1]))
	}

	// Total time should reflect rate limiting delays
	expectedMinTime := 2 * cfg.RateLimitDelay // Two delays between three requests
	if totalTime < expectedMinTime {
		t.Errorf("Total time %v was less than expected minimum %v", totalTime, expectedMinTime)
	}

	t.Logf("Rate limiting test completed in %v with delays: %v, %v",
		totalTime,
		requestTimes[1].Sub(requestTimes[0]),
		requestTimes[2].Sub(requestTimes[1]))
}

func TestIntegration_RateLimitError_Handling(t *testing.T) {
	cfg := &config.Config{
		RateLimitDelay:         10 * time.Millisecond,
		MaxConcurrentRequests:  5,
		ExponentialBackoffBase: 50 * time.Millisecond,
		MaxBackoffDelay:        500 * time.Millisecond,
	}

	// Create a test server that returns 429 on first request
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte("Rate limited"))
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		}
	}))
	defer server.Close()

	// Create rate limiter and transport
	rateLimiter := NewRateLimiter(cfg)
	transport := NewRateLimitedTransport(nil, rateLimiter)

	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	ctx := context.Background()

	// First request - should get 429
	req1, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	resp1, err := client.Do(req1)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}
	_ = resp1.Body.Close()

	if resp1.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Expected 429 status, got %d", resp1.StatusCode)
	}

	// The rate limiter should have processed the 429 response
	apiRateLimiter := rateLimiter.(*APIRateLimiter)
	if apiRateLimiter.consecutiveErrors != 1 {
		t.Errorf("Expected 1 consecutive error, got %d", apiRateLimiter.consecutiveErrors)
	}

	// Immediate second request should be delayed due to backoff
	start := time.Now()
	req2, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
	resp2, err := client.Do(req2)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Second request failed: %v", err)
	}
	_ = resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 status, got %d", resp2.StatusCode)
	}

	// Should have been delayed due to exponential backoff
	if elapsed < 40*time.Millisecond {
		t.Errorf("Second request was not delayed enough: %v", elapsed)
	}

	// Consecutive errors should be reset after successful response
	if apiRateLimiter.consecutiveErrors != 0 {
		t.Errorf("Expected consecutive errors to be reset to 0, got %d", apiRateLimiter.consecutiveErrors)
	}

	t.Logf("Rate limit error handling test completed with backoff delay: %v", elapsed)
}

func TestIntegration_ConcurrencyLimiting(t *testing.T) {
	cfg := &config.Config{
		RateLimitDelay:         1 * time.Millisecond, // Minimal delay
		MaxConcurrentRequests:  2,                    // Low limit for testing
		ExponentialBackoffBase: 50 * time.Millisecond,
		MaxBackoffDelay:        500 * time.Millisecond,
	}

	// Create a test server that simulates slow responses
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Simulate slow response
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))
	defer server.Close()

	// Create rate limiter and transport
	rateLimiter := NewRateLimiter(cfg)
	transport := NewRateLimitedTransport(nil, rateLimiter)

	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	ctx := context.Background()

	// Start 4 concurrent requests
	start := time.Now()
	responseChan := make(chan time.Time, 4)

	for i := 0; i < 4; i++ {
		go func(reqNum int) {
			req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
			resp, err := client.Do(req)
			if err != nil {
				t.Errorf("Request %d failed: %v", reqNum, err)
				return
			}
			_ = resp.Body.Close()
			responseChan <- time.Now()
		}(i)
	}

	// Collect all responses
	for i := 0; i < 4; i++ {
		select {
		case responseTime := <-responseChan:
			_ = responseTime // Response time captured but not analyzed in this test
		case <-time.After(3 * time.Second):
			t.Fatalf("Request %d timed out", i)
		}
	}

	totalTime := time.Since(start)

	// Due to concurrency limit of 2 and 100ms server delay,
	// we should see two batches of responses
	t.Logf("Concurrency test completed in %v", totalTime)

	// Should take at least 200ms (two batches of 100ms each)
	if totalTime < 180*time.Millisecond {
		t.Errorf("Requests completed too quickly, concurrency limiting may not be working: %v", totalTime)
	}

	// But should not take too much longer than 250ms
	if totalTime > 400*time.Millisecond {
		t.Errorf("Requests took too long, may indicate serialization instead of limited concurrency: %v", totalTime)
	}
}
