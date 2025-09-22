package ratelimit

import (
	"net/http"
)

// RateLimitedTransport wraps an HTTP transport with rate limiting capabilities
type RateLimitedTransport struct {
	// Base transport for actual HTTP operations
	Base http.RoundTripper

	// Rate limiter for controlling request frequency
	RateLimiter RateLimiter
}

// NewRateLimitedTransport creates a new rate-limited HTTP transport
func NewRateLimitedTransport(base http.RoundTripper, rateLimiter RateLimiter) *RateLimitedTransport {
	if base == nil {
		base = http.DefaultTransport
	}

	return &RateLimitedTransport{
		Base:        base,
		RateLimiter: rateLimiter,
	}
}

// RoundTrip implements http.RoundTripper with rate limiting
func (t *RateLimitedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	// Acquire concurrency slot
	if err := t.RateLimiter.AcquireSlot(ctx); err != nil {
		return nil, err
	}
	defer t.RateLimiter.ReleaseSlot()

	// Wait for rate limiting
	if err := t.RateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	// Execute the actual HTTP request
	response, err := t.Base.RoundTrip(req)

	// Handle the response for rate limiting feedback
	if response != nil {
		if handleErr := t.RateLimiter.HandleResponse(response); handleErr != nil {
			// If rate limiter indicates we should retry, we could implement retry logic here
			// For now, we'll just return the original response and let the caller handle it
			_ = handleErr // Explicitly ignore the error for now
		}
	}

	return response, err
}

// BearerTokenRateLimitedTransport combines bearer token auth with rate limiting
type BearerTokenRateLimitedTransport struct {
	Token       string
	RateLimiter RateLimiter
	Base        http.RoundTripper
}

// NewBearerTokenRateLimitedTransport creates a new transport with both auth and rate limiting
func NewBearerTokenRateLimitedTransport(token string, rateLimiter RateLimiter) *BearerTokenRateLimitedTransport {
	return &BearerTokenRateLimitedTransport{
		Token:       token,
		RateLimiter: rateLimiter,
		Base:        http.DefaultTransport,
	}
}

// RoundTrip implements http.RoundTripper with auth and rate limiting
func (t *BearerTokenRateLimitedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	// Acquire concurrency slot
	if err := t.RateLimiter.AcquireSlot(ctx); err != nil {
		return nil, err
	}
	defer t.RateLimiter.ReleaseSlot()

	// Wait for rate limiting
	if err := t.RateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	// Add authorization header
	req.Header.Set("Authorization", "Bearer "+t.Token)

	// Execute the actual HTTP request
	response, err := t.Base.RoundTrip(req)

	// Handle the response for rate limiting feedback
	if response != nil {
		if handleErr := t.RateLimiter.HandleResponse(response); handleErr != nil {
			// Rate limiter detected an issue, but we still return the original response
			// The application can decide how to handle rate limit errors
			_ = handleErr // Explicitly ignore the error for now
		}
	}

	return response, err
}
