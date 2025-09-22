package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewRateLimitedTransport(t *testing.T) {
	mockRateLimiter := NewMockRateLimiter()

	// Test with nil base transport
	transport := NewRateLimitedTransport(nil, mockRateLimiter)
	if transport.Base != http.DefaultTransport {
		t.Error("Expected http.DefaultTransport when base is nil")
	}

	// Test with custom base transport
	customBase := &http.Transport{}
	transport = NewRateLimitedTransport(customBase, mockRateLimiter)
	if transport.Base != customBase {
		t.Error("Expected custom base transport to be used")
	}
}

func TestRateLimitedTransport_RoundTrip(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response"))
	}))
	defer server.Close()

	mockRateLimiter := NewMockRateLimiter()
	transport := NewRateLimitedTransport(nil, mockRateLimiter)

	// Create a test request
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Execute the request
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Verify rate limiter was called
	if len(mockRateLimiter.AcquireSlotCalls) != 1 {
		t.Errorf("Expected 1 AcquireSlot call, got %d", len(mockRateLimiter.AcquireSlotCalls))
	}

	if len(mockRateLimiter.WaitCalls) != 1 {
		t.Errorf("Expected 1 Wait call, got %d", len(mockRateLimiter.WaitCalls))
	}

	if mockRateLimiter.ReleaseSlotCalls != 1 {
		t.Errorf("Expected 1 ReleaseSlot call, got %d", mockRateLimiter.ReleaseSlotCalls)
	}

	if len(mockRateLimiter.HandleResponseCalls) != 1 {
		t.Errorf("Expected 1 HandleResponse call, got %d", len(mockRateLimiter.HandleResponseCalls))
	}

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestRateLimitedTransport_RoundTrip_AcquireSlotError(t *testing.T) {
	mockRateLimiter := NewMockRateLimiter()

	// Configure mock to return error on AcquireSlot
	mockRateLimiter.AcquireSlotFunc = func(ctx context.Context) error {
		return context.DeadlineExceeded
	}

	transport := NewRateLimitedTransport(nil, mockRateLimiter)

	req, _ := http.NewRequest("GET", "http://example.com", nil)

	_, err := transport.RoundTrip(req)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
	}

	// Should not call Wait or ReleaseSlot if AcquireSlot fails
	if len(mockRateLimiter.WaitCalls) != 0 {
		t.Errorf("Expected 0 Wait calls, got %d", len(mockRateLimiter.WaitCalls))
	}

	if mockRateLimiter.ReleaseSlotCalls != 0 {
		t.Errorf("Expected 0 ReleaseSlot calls, got %d", mockRateLimiter.ReleaseSlotCalls)
	}
}

func TestRateLimitedTransport_RoundTrip_WaitError(t *testing.T) {
	mockRateLimiter := NewMockRateLimiter()

	// Configure mock to return error on Wait
	mockRateLimiter.WaitFunc = func(ctx context.Context) error {
		return context.DeadlineExceeded
	}

	transport := NewRateLimitedTransport(nil, mockRateLimiter)

	req, _ := http.NewRequest("GET", "http://example.com", nil)

	_, err := transport.RoundTrip(req)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
	}

	// Should still call ReleaseSlot even if Wait fails
	if mockRateLimiter.ReleaseSlotCalls != 1 {
		t.Errorf("Expected 1 ReleaseSlot call, got %d", mockRateLimiter.ReleaseSlotCalls)
	}
}

func TestNewBearerTokenRateLimitedTransport(t *testing.T) {
	mockRateLimiter := NewMockRateLimiter()
	token := "test-token"

	transport := NewBearerTokenRateLimitedTransport(token, mockRateLimiter)

	if transport.Token != token {
		t.Errorf("Expected token %s, got %s", token, transport.Token)
	}

	if transport.RateLimiter != mockRateLimiter {
		t.Error("Expected rate limiter to be set")
	}

	if transport.Base != http.DefaultTransport {
		t.Error("Expected http.DefaultTransport as base")
	}
}

func TestBearerTokenRateLimitedTransport_RoundTrip(t *testing.T) {
	// Create a test server that checks for Authorization header
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	mockRateLimiter := NewMockRateLimiter()
	token := "test-bearer-token"
	transport := NewBearerTokenRateLimitedTransport(token, mockRateLimiter)

	// Create a test request
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Execute the request
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Verify Authorization header was added
	expectedAuth := "Bearer " + token
	if receivedAuth != expectedAuth {
		t.Errorf("Expected Authorization header %s, got %s", expectedAuth, receivedAuth)
	}

	// Verify rate limiter was called
	if len(mockRateLimiter.AcquireSlotCalls) != 1 {
		t.Errorf("Expected 1 AcquireSlot call, got %d", len(mockRateLimiter.AcquireSlotCalls))
	}

	if len(mockRateLimiter.WaitCalls) != 1 {
		t.Errorf("Expected 1 Wait call, got %d", len(mockRateLimiter.WaitCalls))
	}

	if mockRateLimiter.ReleaseSlotCalls != 1 {
		t.Errorf("Expected 1 ReleaseSlot call, got %d", mockRateLimiter.ReleaseSlotCalls)
	}
}
