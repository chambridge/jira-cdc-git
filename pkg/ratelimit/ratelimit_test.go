package ratelimit

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/config"
)

func TestNewRateLimiter(t *testing.T) {
	cfg := &config.Config{
		RateLimitDelay:         100 * time.Millisecond,
		MaxConcurrentRequests:  5,
		ExponentialBackoffBase: 1 * time.Second,
		MaxBackoffDelay:        30 * time.Second,
	}

	rateLimiter := NewRateLimiter(cfg)

	if rateLimiter == nil {
		t.Fatal("NewRateLimiter returned nil")
	}

	// Verify it implements the interface
	var _ = rateLimiter
}

func TestAPIRateLimiter_Wait_BasicDelay(t *testing.T) {
	cfg := &config.Config{
		RateLimitDelay:         100 * time.Millisecond,
		MaxConcurrentRequests:  5,
		ExponentialBackoffBase: 1 * time.Second,
		MaxBackoffDelay:        30 * time.Second,
	}

	rateLimiter := NewRateLimiter(cfg).(*APIRateLimiter)
	ctx := context.Background()

	// First request should be immediate
	start := time.Now()
	err := rateLimiter.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
	elapsed := time.Since(start)

	if elapsed > 50*time.Millisecond {
		t.Errorf("First request took too long: %v", elapsed)
	}

	// Second request should be delayed
	start = time.Now()
	err = rateLimiter.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
	elapsed = time.Since(start)

	// Should be at least the rate limit delay
	if elapsed < 95*time.Millisecond {
		t.Errorf("Second request was not delayed enough: %v", elapsed)
	}
}

func TestAPIRateLimiter_Wait_ContextCancellation(t *testing.T) {
	cfg := &config.Config{
		RateLimitDelay:         500 * time.Millisecond,
		MaxConcurrentRequests:  5,
		ExponentialBackoffBase: 1 * time.Second,
		MaxBackoffDelay:        30 * time.Second,
	}

	rateLimiter := NewRateLimiter(cfg).(*APIRateLimiter)

	// Set up a short-lived context
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// First request to set last request time
	_ = rateLimiter.Wait(context.Background())

	// Second request should be cancelled due to context timeout
	err := rateLimiter.Wait(ctx)
	if err == nil {
		t.Error("Expected context cancellation error")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
	}
}

func TestAPIRateLimiter_HandleResponse_RateLimit(t *testing.T) {
	cfg := &config.Config{
		RateLimitDelay:         100 * time.Millisecond,
		MaxConcurrentRequests:  5,
		ExponentialBackoffBase: 1 * time.Second,
		MaxBackoffDelay:        30 * time.Second,
	}

	rateLimiter := NewRateLimiter(cfg).(*APIRateLimiter)

	// Create a 429 response
	response := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     make(http.Header),
	}

	err := rateLimiter.HandleResponse(response)

	// Should return a rate limit error
	if err == nil {
		t.Error("Expected rate limit error")
	}

	rateLimitErr, ok := err.(*RateLimitError)
	if !ok {
		t.Errorf("Expected RateLimitError, got: %T", err)
	}

	if rateLimitErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Expected status code %d, got: %d", http.StatusTooManyRequests, rateLimitErr.StatusCode)
	}

	// Should trigger exponential backoff
	if rateLimiter.consecutiveErrors != 1 {
		t.Errorf("Expected 1 consecutive error, got: %d", rateLimiter.consecutiveErrors)
	}
}

func TestAPIRateLimiter_HandleResponse_RetryAfterHeader(t *testing.T) {
	cfg := &config.Config{
		RateLimitDelay:         100 * time.Millisecond,
		MaxConcurrentRequests:  5,
		ExponentialBackoffBase: 1 * time.Second,
		MaxBackoffDelay:        30 * time.Second,
	}

	rateLimiter := NewRateLimiter(cfg).(*APIRateLimiter)

	// Create a 429 response with Retry-After header
	response := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     make(http.Header),
	}
	response.Header.Set("Retry-After", "5")

	err := rateLimiter.HandleResponse(response)

	rateLimitErr, ok := err.(*RateLimitError)
	if !ok {
		t.Fatalf("Expected RateLimitError, got: %T", err)
	}

	// Retry after should be around 5 seconds (allowing some variance)
	if rateLimitErr.RetryAfter < 4*time.Second || rateLimitErr.RetryAfter > 6*time.Second {
		t.Errorf("Expected retry after around 5s, got: %v", rateLimitErr.RetryAfter)
	}
}

func TestAPIRateLimiter_HandleResponse_Success(t *testing.T) {
	cfg := &config.Config{
		RateLimitDelay:         100 * time.Millisecond,
		MaxConcurrentRequests:  5,
		ExponentialBackoffBase: 1 * time.Second,
		MaxBackoffDelay:        30 * time.Second,
	}

	rateLimiter := NewRateLimiter(cfg).(*APIRateLimiter)

	// Set some consecutive errors
	rateLimiter.consecutiveErrors = 3

	// Create a successful response
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
	}

	err := rateLimiter.HandleResponse(response)

	if err != nil {
		t.Errorf("Unexpected error for successful response: %v", err)
	}

	// Should reset consecutive errors
	if rateLimiter.consecutiveErrors != 0 {
		t.Errorf("Expected consecutive errors to be reset to 0, got: %d", rateLimiter.consecutiveErrors)
	}
}

func TestAPIRateLimiter_AcquireReleaseSlot(t *testing.T) {
	cfg := &config.Config{
		RateLimitDelay:         100 * time.Millisecond,
		MaxConcurrentRequests:  2, // Small limit for testing
		ExponentialBackoffBase: 1 * time.Second,
		MaxBackoffDelay:        30 * time.Second,
	}

	rateLimiter := NewRateLimiter(cfg)
	ctx := context.Background()

	// Acquire first slot
	err := rateLimiter.AcquireSlot(ctx)
	if err != nil {
		t.Fatalf("Failed to acquire first slot: %v", err)
	}

	// Acquire second slot
	err = rateLimiter.AcquireSlot(ctx)
	if err != nil {
		t.Fatalf("Failed to acquire second slot: %v", err)
	}

	// Third slot should block - test with timeout
	shortCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	err = rateLimiter.AcquireSlot(shortCtx)
	if err == nil {
		t.Error("Expected context timeout when acquiring third slot")
		rateLimiter.ReleaseSlot() // Clean up if it somehow succeeded
	}

	// Release one slot
	rateLimiter.ReleaseSlot()

	// Now third slot should succeed
	err = rateLimiter.AcquireSlot(ctx)
	if err != nil {
		t.Fatalf("Failed to acquire slot after release: %v", err)
	}

	// Clean up
	rateLimiter.ReleaseSlot()
	rateLimiter.ReleaseSlot()
}

func TestAPIRateLimiter_ExponentialBackoff(t *testing.T) {
	cfg := &config.Config{
		RateLimitDelay:         100 * time.Millisecond,
		MaxConcurrentRequests:  5,
		ExponentialBackoffBase: 100 * time.Millisecond,
		MaxBackoffDelay:        1 * time.Second,
	}

	rateLimiter := NewRateLimiter(cfg).(*APIRateLimiter)

	tests := []struct {
		consecutiveErrors int
		expectedDelay     time.Duration
	}{
		{0, 0},
		{1, 100 * time.Millisecond}, // base
		{2, 200 * time.Millisecond}, // base * 2
		{3, 400 * time.Millisecond}, // base * 4
		{4, 800 * time.Millisecond}, // base * 8
		{5, 1 * time.Second},        // capped at max
		{10, 1 * time.Second},       // still capped at max
	}

	for _, test := range tests {
		rateLimiter.consecutiveErrors = test.consecutiveErrors
		delay := rateLimiter.calculateBackoffDelay()

		if delay != test.expectedDelay {
			t.Errorf("For %d consecutive errors, expected delay %v, got %v",
				test.consecutiveErrors, test.expectedDelay, delay)
		}
	}
}

func TestRateLimitError_Error(t *testing.T) {
	err := &RateLimitError{
		StatusCode: http.StatusTooManyRequests,
		RetryAfter: 5 * time.Second,
		Message:    "test rate limit exceeded",
	}

	expected := "rate limit error (HTTP 429): test rate limit exceeded (retry after 5s)"
	if err.Error() != expected {
		t.Errorf("Expected error message: %s, got: %s", expected, err.Error())
	}
}

func TestIsRateLimitError(t *testing.T) {
	// Test with rate limit error
	rateLimitErr := &RateLimitError{
		StatusCode: http.StatusTooManyRequests,
		RetryAfter: 5 * time.Second,
		Message:    "rate limit exceeded",
	}

	if !IsRateLimitError(rateLimitErr) {
		t.Error("Expected IsRateLimitError to return true for RateLimitError")
	}

	// Test with regular error
	regularErr := errors.New("test error")

	if IsRateLimitError(regularErr) {
		t.Error("Expected IsRateLimitError to return false for regular error")
	}
}
