package helpers

import (
	"sync"
	"time"
)

// nowFunc allows mocking time.Now() for testing
var nowFunc = time.Now

// Now returns current time (wrapper for testing)
func Now() time.Time {
	return nowFunc()
}

// RateLimitInfo contains rate limit information for response headers
type RateLimitInfo struct {
	Limit     int   // Maximum requests allowed per window
	Remaining int   // Remaining requests in current window
	Reset     int64 // Unix timestamp when the limit resets
	Allowed   bool  // Whether the request is allowed
}

// bucket represents a token bucket for a single client (thread-safe)
type bucket struct {
	mu         sync.Mutex
	tokens     int
	lastRefill time.Time
}

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	limit      int
	windowSecs int
	buckets    sync.Map // map[string]*bucket
}

// NewRateLimiter creates a new rate limiter and starts a background sweeper that
// removes buckets idle for more than one window, so memory does not grow with
// every distinct IP ever seen.
func NewRateLimiter(limit, windowSecs int) *RateLimiter {
	rl := &RateLimiter{
		limit:      limit,
		windowSecs: windowSecs,
	}
	go rl.sweep()
	return rl
}

// sweep periodically deletes buckets whose window has ended.
func (rl *RateLimiter) sweep() {
	window := time.Duration(rl.windowSecs) * time.Second
	if window <= 0 {
		return
	}
	ticker := time.NewTicker(window)
	defer ticker.Stop()
	for now := range ticker.C {
		rl.buckets.Range(func(key, value any) bool {
			b := value.(*bucket)
			b.mu.Lock()
			expired := now.Sub(b.lastRefill) >= window
			b.mu.Unlock()
			if expired {
				rl.buckets.Delete(key)
			}
			return true
		})
	}
}

// Allow checks if a request from the given IP is allowed and returns rate limit info
func (rl *RateLimiter) Allow(ip string) *RateLimitInfo {
	now := time.Now()
	resetTime := now.Add(time.Duration(rl.windowSecs) * time.Second).Unix()

	// Get or create bucket for this IP
	actual, loaded := rl.buckets.LoadOrStore(ip, &bucket{
		tokens:     rl.limit - 1,
		lastRefill: now,
	})
	if !loaded {
		return &RateLimitInfo{
			Limit:     rl.limit,
			Remaining: rl.limit - 1,
			Reset:     resetTime,
			Allowed:   true,
		}
	}

	b := actual.(*bucket)

	// Lock for thread-safe access
	b.mu.Lock()
	defer b.mu.Unlock()

	// Check if bucket needs refill (window expired)
	elapsed := now.Sub(b.lastRefill)
	if elapsed >= time.Duration(rl.windowSecs)*time.Second {
		// Refill tokens
		b.tokens = rl.limit - 1
		b.lastRefill = now
		resetTime = now.Add(time.Duration(rl.windowSecs) * time.Second).Unix()
		return &RateLimitInfo{
			Limit:     rl.limit,
			Remaining: rl.limit - 1,
			Reset:     resetTime,
			Allowed:   true,
		}
	}

	// Check if tokens available
	if b.tokens <= 0 {
		return &RateLimitInfo{
			Limit:     rl.limit,
			Remaining: 0,
			Reset:     b.lastRefill.Add(time.Duration(rl.windowSecs) * time.Second).Unix(),
			Allowed:   false,
		}
	}

	// Consume token
	b.tokens--
	return &RateLimitInfo{
		Limit:     rl.limit,
		Remaining: b.tokens,
		Reset:     b.lastRefill.Add(time.Duration(rl.windowSecs) * time.Second).Unix(),
		Allowed:   true,
	}
}
