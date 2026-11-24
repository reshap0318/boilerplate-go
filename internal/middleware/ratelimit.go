package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/reshap0318/go-boilerplate/internal/helpers"
)

// RateLimit returns a rate limiting middleware that applies globally.
// It sets rate limit headers on all responses and returns 429 when limit exceeded.
func RateLimit(limiter *helpers.RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client IP (handle proxied requests)
		ip := getClientIP(c)

		// Check rate limit
		info := limiter.Allow(ip)

		// Set rate limit headers on ALL responses
		c.Header("X-RateLimit-Remaining", strconv.Itoa(info.Remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(info.Reset, 10))

		if !info.Allowed {
			// Calculate retry-after in seconds
			retryAfter := int(info.Reset - time.Now().Unix())
			if retryAfter < 0 {
				retryAfter = 0
			}
			c.Header("Retry-After", strconv.Itoa(retryAfter))

			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded. Please try again later.",
			})
			return
		}

		c.Next()
	}
}

// getClientIP returns the client IP using gin's ClientIP(), which only honours
// X-Forwarded-For / X-Real-IP when the request comes from a proxy listed in
// TRUSTED_PROXIES. Reading those headers directly would let any client pick its
// own IP and bypass the rate limit.
func getClientIP(c *gin.Context) string {
	return c.ClientIP()
}
