package ratelimit

import (
	"context"
	"net/http"
	"time"
)

// MockRateLimiter provides a mock implementation for testing
type MockRateLimiter struct {
	// Function stubs for customizing behavior in tests
	WaitFunc           func(ctx context.Context) error
	HandleResponseFunc func(response *http.Response) error
	AcquireSlotFunc    func(ctx context.Context) error
	ReleaseSlotFunc    func()

	// Call tracking for verification in tests
	WaitCalls           []context.Context
	HandleResponseCalls []*http.Response
	AcquireSlotCalls    []context.Context
	ReleaseSlotCalls    int

	// State tracking
	SlotsAcquired int
	LastWaitTime  time.Time
}

// NewMockRateLimiter creates a new mock rate limiter with default behavior
func NewMockRateLimiter() *MockRateLimiter {
	return &MockRateLimiter{
		// Default implementations that do nothing
		WaitFunc:           func(ctx context.Context) error { return nil },
		HandleResponseFunc: func(response *http.Response) error { return nil },
		AcquireSlotFunc:    func(ctx context.Context) error { return nil },
		ReleaseSlotFunc:    func() {},
	}
}

// Wait implements RateLimiter interface
func (m *MockRateLimiter) Wait(ctx context.Context) error {
	m.WaitCalls = append(m.WaitCalls, ctx)
	m.LastWaitTime = time.Now()

	if m.WaitFunc != nil {
		return m.WaitFunc(ctx)
	}
	return nil
}

// HandleResponse implements RateLimiter interface
func (m *MockRateLimiter) HandleResponse(response *http.Response) error {
	m.HandleResponseCalls = append(m.HandleResponseCalls, response)

	if m.HandleResponseFunc != nil {
		return m.HandleResponseFunc(response)
	}
	return nil
}

// AcquireSlot implements RateLimiter interface
func (m *MockRateLimiter) AcquireSlot(ctx context.Context) error {
	m.AcquireSlotCalls = append(m.AcquireSlotCalls, ctx)
	m.SlotsAcquired++

	if m.AcquireSlotFunc != nil {
		return m.AcquireSlotFunc(ctx)
	}
	return nil
}

// ReleaseSlot implements RateLimiter interface
func (m *MockRateLimiter) ReleaseSlot() {
	m.ReleaseSlotCalls++
	if m.SlotsAcquired > 0 {
		m.SlotsAcquired--
	}

	if m.ReleaseSlotFunc != nil {
		m.ReleaseSlotFunc()
	}
}

// Reset clears all call tracking for reuse in tests
func (m *MockRateLimiter) Reset() {
	m.WaitCalls = nil
	m.HandleResponseCalls = nil
	m.AcquireSlotCalls = nil
	m.ReleaseSlotCalls = 0
	m.SlotsAcquired = 0
}

// SetWaitDelay configures the mock to introduce a delay in Wait calls
func (m *MockRateLimiter) SetWaitDelay(delay time.Duration) {
	m.WaitFunc = func(ctx context.Context) error {
		select {
		case <-time.After(delay):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// SetRateLimitError configures the mock to return rate limit errors
func (m *MockRateLimiter) SetRateLimitError(retryAfter time.Duration) {
	m.HandleResponseFunc = func(response *http.Response) error {
		if response != nil && response.StatusCode == http.StatusTooManyRequests {
			return &RateLimitError{
				StatusCode: http.StatusTooManyRequests,
				RetryAfter: retryAfter,
				Message:    "mock rate limit exceeded",
			}
		}
		return nil
	}
}

// SetConcurrencyLimit configures the mock to limit concurrent requests
func (m *MockRateLimiter) SetConcurrencyLimit(limit int) {
	semaphore := make(chan struct{}, limit)

	m.AcquireSlotFunc = func(ctx context.Context) error {
		select {
		case semaphore <- struct{}{}:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	m.ReleaseSlotFunc = func() {
		select {
		case <-semaphore:
			// Slot released
		default:
			// No slot to release
		}
	}
}
