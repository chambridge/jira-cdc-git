package ratelimit

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/chambrid/jira-cdc-git/pkg/config"
)

// RateLimiter defines the interface for rate limiting operations
// This enables dependency injection and testing with mock implementations
type RateLimiter interface {
	// Wait blocks until it's safe to make a request based on rate limiting rules
	Wait(ctx context.Context) error

	// HandleResponse processes response headers to adjust rate limiting behavior
	HandleResponse(response *http.Response) error

	// AcquireSlot attempts to acquire a concurrency slot for parallel requests
	AcquireSlot(ctx context.Context) error

	// ReleaseSlot releases a concurrency slot
	ReleaseSlot()
}

// APIRateLimiter implements the RateLimiter interface with JIRA-specific logic
type APIRateLimiter struct {
	config *config.Config

	// Rate limiting state
	lastRequest time.Time
	mutex       sync.Mutex

	// Exponential backoff state
	consecutiveErrors int
	backoffUntil      time.Time

	// Concurrency control
	semaphore chan struct{}

	// Rate limit detection from headers
	rateLimitRemaining int
	rateLimitReset     time.Time
}

// NewRateLimiter creates a new rate limiter with the provided configuration
func NewRateLimiter(cfg *config.Config) RateLimiter {
	// Create semaphore for concurrency control
	semaphore := make(chan struct{}, cfg.MaxConcurrentRequests)

	return &APIRateLimiter{
		config:    cfg,
		semaphore: semaphore,

		// Initialize rate limit state (conservative defaults)
		rateLimitRemaining: 1000, // Assume reasonable remaining quota
		rateLimitReset:     time.Now().Add(1 * time.Hour),
	}
}

// Wait blocks until it's safe to make a request
func (r *APIRateLimiter) Wait(ctx context.Context) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Helper function to wait while releasing the mutex
	waitWithUnlock := func(waitTime time.Duration) error {
		r.mutex.Unlock()
		defer r.mutex.Lock()

		select {
		case <-time.After(waitTime):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Check if we're in exponential backoff period
	if time.Now().Before(r.backoffUntil) {
		waitTime := time.Until(r.backoffUntil)
		if err := waitWithUnlock(waitTime); err != nil {
			return err
		}
	}

	// Apply basic rate limiting delay
	timeSinceLastRequest := time.Since(r.lastRequest)
	if timeSinceLastRequest < r.config.RateLimitDelay {
		waitTime := r.config.RateLimitDelay - timeSinceLastRequest
		if err := waitWithUnlock(waitTime); err != nil {
			return err
		}
	}

	// Check JIRA-specific rate limits from headers
	if r.rateLimitRemaining <= 1 && time.Now().Before(r.rateLimitReset) {
		waitTime := time.Until(r.rateLimitReset)
		if err := waitWithUnlock(waitTime); err != nil {
			return err
		}
		// Rate limit window reset
		r.rateLimitRemaining = 1000 // Reset to conservative default
	}

	// Update last request time
	r.lastRequest = time.Now()

	return nil
}

// HandleResponse processes response headers to adjust rate limiting behavior
func (r *APIRateLimiter) HandleResponse(response *http.Response) error {
	if response == nil {
		return nil
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Handle rate limit responses (429 Too Many Requests)
	if response.StatusCode == http.StatusTooManyRequests {
		r.consecutiveErrors++

		// Calculate exponential backoff delay
		backoffDelay := r.calculateBackoffDelay()
		r.backoffUntil = time.Now().Add(backoffDelay)

		// Try to get retry-after header for more precise timing
		if retryAfterStr := response.Header.Get("Retry-After"); retryAfterStr != "" {
			if retryAfter, err := strconv.Atoi(retryAfterStr); err == nil {
				suggestedDelay := time.Duration(retryAfter) * time.Second
				if suggestedDelay > backoffDelay {
					r.backoffUntil = time.Now().Add(suggestedDelay)
				}
			}
		}

		return &RateLimitError{
			StatusCode: response.StatusCode,
			RetryAfter: time.Until(r.backoffUntil),
			Message:    "rate limit exceeded, backing off",
		}
	}

	// Parse JIRA rate limit headers if present
	if remaining := response.Header.Get("X-RateLimit-Remaining"); remaining != "" {
		if count, err := strconv.Atoi(remaining); err == nil {
			r.rateLimitRemaining = count
		}
	}

	if reset := response.Header.Get("X-RateLimit-Reset"); reset != "" {
		if resetTime, err := strconv.ParseInt(reset, 10, 64); err == nil {
			r.rateLimitReset = time.Unix(resetTime, 0)
		}
	}

	// Success response - reset consecutive errors
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		r.consecutiveErrors = 0
	}

	return nil
}

// AcquireSlot attempts to acquire a concurrency slot
func (r *APIRateLimiter) AcquireSlot(ctx context.Context) error {
	select {
	case r.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ReleaseSlot releases a concurrency slot
func (r *APIRateLimiter) ReleaseSlot() {
	select {
	case <-r.semaphore:
		// Slot released
	default:
		// No slot to release (shouldn't happen in normal operation)
	}
}

// calculateBackoffDelay calculates exponential backoff delay
func (r *APIRateLimiter) calculateBackoffDelay() time.Duration {
	if r.consecutiveErrors <= 0 {
		return 0
	}

	// Calculate exponential backoff: base * 2^(errors-1)
	exponent := float64(r.consecutiveErrors - 1)
	multiplier := math.Pow(2, exponent)

	delay := time.Duration(float64(r.config.ExponentialBackoffBase) * multiplier)

	// Cap at maximum backoff delay
	if delay > r.config.MaxBackoffDelay {
		delay = r.config.MaxBackoffDelay
	}

	return delay
}

// RateLimitError represents a rate limiting error
type RateLimitError struct {
	StatusCode int
	RetryAfter time.Duration
	Message    string
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit error (HTTP %d): %s (retry after %v)",
		e.StatusCode, e.Message, e.RetryAfter)
}

// IsRateLimitError checks if an error is a rate limit error
func IsRateLimitError(err error) bool {
	_, ok := err.(*RateLimitError)
	return ok
}
