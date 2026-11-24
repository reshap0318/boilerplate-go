package helpers

import "testing"

func TestRateLimiterLimits(t *testing.T) {
	rl := &RateLimiter{limit: 2, windowSecs: 60}
	if !rl.Allow("1.1.1.1").Allowed || !rl.Allow("1.1.1.1").Allowed {
		t.Fatal("first two requests should pass")
	}
	if rl.Allow("1.1.1.1").Allowed {
		t.Fatal("third request should be limited")
	}
}
