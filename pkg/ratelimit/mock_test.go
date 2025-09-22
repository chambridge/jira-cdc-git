package ratelimit

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestNewMockRateLimiter(t *testing.T) {
	mock := NewMockRateLimiter()

	if mock == nil {
		t.Fatal("NewMockRateLimiter returned nil")
	}

	// Verify it implements the interface
	var _ RateLimiter = mock

	// Test default behavior does nothing
	ctx := context.Background()

	err := mock.Wait(ctx)
	if err != nil {
		t.Errorf("Default Wait should not return error, got: %v", err)
	}

	err = mock.HandleResponse(&http.Response{})
	if err != nil {
		t.Errorf("Default HandleResponse should not return error, got: %v", err)
	}

	err = mock.AcquireSlot(ctx)
	if err != nil {
		t.Errorf("Default AcquireSlot should not return error, got: %v", err)
	}

	mock.ReleaseSlot() // Should not panic
}

func TestMockRateLimiter_CallTracking(t *testing.T) {
	mock := NewMockRateLimiter()
	ctx := context.Background()
	response := &http.Response{StatusCode: http.StatusOK}

	// Make calls
	_ = mock.Wait(ctx)
	_ = mock.Wait(ctx)
	_ = mock.HandleResponse(response)
	_ = mock.AcquireSlot(ctx)
	_ = mock.AcquireSlot(ctx)
	_ = mock.AcquireSlot(ctx)
	mock.ReleaseSlot()
	mock.ReleaseSlot()

	// Verify call tracking
	if len(mock.WaitCalls) != 2 {
		t.Errorf("Expected 2 Wait calls, got %d", len(mock.WaitCalls))
	}

	if len(mock.HandleResponseCalls) != 1 {
		t.Errorf("Expected 1 HandleResponse call, got %d", len(mock.HandleResponseCalls))
	}

	if mock.HandleResponseCalls[0] != response {
		t.Error("HandleResponse call did not track the correct response")
	}

	if len(mock.AcquireSlotCalls) != 3 {
		t.Errorf("Expected 3 AcquireSlot calls, got %d", len(mock.AcquireSlotCalls))
	}

	if mock.ReleaseSlotCalls != 2 {
		t.Errorf("Expected 2 ReleaseSlot calls, got %d", mock.ReleaseSlotCalls)
	}

	if mock.SlotsAcquired != 1 { // 3 acquired - 2 released
		t.Errorf("Expected 1 slot acquired, got %d", mock.SlotsAcquired)
	}
}

func TestMockRateLimiter_Reset(t *testing.T) {
	mock := NewMockRateLimiter()
	ctx := context.Background()

	// Make some calls
	_ = mock.Wait(ctx)
	_ = mock.HandleResponse(&http.Response{})
	_ = mock.AcquireSlot(ctx)
	mock.ReleaseSlot()

	// Verify calls were tracked
	if len(mock.WaitCalls) == 0 {
		t.Error("Expected calls to be tracked before reset")
	}

	// Reset
	mock.Reset()

	// Verify all tracking was cleared
	if len(mock.WaitCalls) != 0 {
		t.Errorf("Expected 0 Wait calls after reset, got %d", len(mock.WaitCalls))
	}

	if len(mock.HandleResponseCalls) != 0 {
		t.Errorf("Expected 0 HandleResponse calls after reset, got %d", len(mock.HandleResponseCalls))
	}

	if len(mock.AcquireSlotCalls) != 0 {
		t.Errorf("Expected 0 AcquireSlot calls after reset, got %d", len(mock.AcquireSlotCalls))
	}

	if mock.ReleaseSlotCalls != 0 {
		t.Errorf("Expected 0 ReleaseSlot calls after reset, got %d", mock.ReleaseSlotCalls)
	}

	if mock.SlotsAcquired != 0 {
		t.Errorf("Expected 0 slots acquired after reset, got %d", mock.SlotsAcquired)
	}
}

func TestMockRateLimiter_SetWaitDelay(t *testing.T) {
	mock := NewMockRateLimiter()
	delay := 100 * time.Millisecond

	mock.SetWaitDelay(delay)

	ctx := context.Background()
	start := time.Now()

	err := mock.Wait(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Wait should not return error, got: %v", err)
	}

	// Should delay for approximately the specified duration
	if elapsed < 95*time.Millisecond || elapsed > 150*time.Millisecond {
		t.Errorf("Expected delay around %v, got %v", delay, elapsed)
	}
}

func TestMockRateLimiter_SetWaitDelay_ContextCancellation(t *testing.T) {
	mock := NewMockRateLimiter()
	mock.SetWaitDelay(500 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := mock.Wait(ctx)
	elapsed := time.Since(start)

	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded, got: %v", err)
	}

	// Should be cancelled before the full delay
	if elapsed > 150*time.Millisecond {
		t.Errorf("Expected early cancellation, took %v", elapsed)
	}
}

func TestMockRateLimiter_SetRateLimitError(t *testing.T) {
	mock := NewMockRateLimiter()
	retryAfter := 5 * time.Second

	mock.SetRateLimitError(retryAfter)

	// Test with 429 response
	response429 := &http.Response{
		StatusCode: http.StatusTooManyRequests,
	}

	err := mock.HandleResponse(response429)
	if err == nil {
		t.Error("Expected rate limit error for 429 response")
	}

	rateLimitErr, ok := err.(*RateLimitError)
	if !ok {
		t.Errorf("Expected RateLimitError, got %T", err)
	}

	if rateLimitErr.RetryAfter != retryAfter {
		t.Errorf("Expected retry after %v, got %v", retryAfter, rateLimitErr.RetryAfter)
	}

	// Test with non-429 response
	response200 := &http.Response{
		StatusCode: http.StatusOK,
	}

	err = mock.HandleResponse(response200)
	if err != nil {
		t.Errorf("Expected no error for 200 response, got: %v", err)
	}
}

func TestMockRateLimiter_SetConcurrencyLimit(t *testing.T) {
	mock := NewMockRateLimiter()
	limit := 2

	mock.SetConcurrencyLimit(limit)

	ctx := context.Background()

	// Acquire first slot
	err := mock.AcquireSlot(ctx)
	if err != nil {
		t.Fatalf("Failed to acquire first slot: %v", err)
	}

	// Acquire second slot
	err = mock.AcquireSlot(ctx)
	if err != nil {
		t.Fatalf("Failed to acquire second slot: %v", err)
	}

	// Third slot should block
	shortCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	err = mock.AcquireSlot(shortCtx)
	if err == nil {
		t.Error("Expected context timeout when acquiring third slot")
		mock.ReleaseSlot() // Clean up if it somehow succeeded
	}

	// Release one slot
	mock.ReleaseSlot()

	// Now third slot should succeed
	err = mock.AcquireSlot(ctx)
	if err != nil {
		t.Fatalf("Failed to acquire slot after release: %v", err)
	}

	// Clean up
	mock.ReleaseSlot()
	mock.ReleaseSlot()
}

func TestMockRateLimiter_CustomFunctions(t *testing.T) {
	mock := NewMockRateLimiter()

	// Set custom functions
	waitCalled := false
	mock.WaitFunc = func(ctx context.Context) error {
		waitCalled = true
		return nil
	}

	handleResponseCalled := false
	mock.HandleResponseFunc = func(response *http.Response) error {
		handleResponseCalled = true
		return nil
	}

	acquireSlotCalled := false
	mock.AcquireSlotFunc = func(ctx context.Context) error {
		acquireSlotCalled = true
		return nil
	}

	releaseSlotCalled := false
	mock.ReleaseSlotFunc = func() {
		releaseSlotCalled = true
	}

	// Call methods
	ctx := context.Background()
	_ = mock.Wait(ctx)
	_ = mock.HandleResponse(&http.Response{})
	_ = mock.AcquireSlot(ctx)
	mock.ReleaseSlot()

	// Verify custom functions were called
	if !waitCalled {
		t.Error("Custom WaitFunc was not called")
	}

	if !handleResponseCalled {
		t.Error("Custom HandleResponseFunc was not called")
	}

	if !acquireSlotCalled {
		t.Error("Custom AcquireSlotFunc was not called")
	}

	if !releaseSlotCalled {
		t.Error("Custom ReleaseSlotFunc was not called")
	}
}
