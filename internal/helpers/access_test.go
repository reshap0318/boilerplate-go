package helpers

import (
	"context"
	"testing"
	"time"
)

func TestAccessUsesLoaderAndLocalCache(t *testing.T) {
	calls := 0
	acc := NewAccess(nil, func(userID uint) ([]string, []string, error) {
		calls++
		return []string{"admin"}, []string{"user.index"}, nil
	})
	ctx := context.WithValue(context.Background(), KeyUserID, uint(7))

	if !acc.HasPermission(ctx, "user.delete", "user.index") {
		t.Fatal("expected permission via ANY match")
	}
	if !acc.HasRole(ctx, "admin") || acc.HasRole(ctx, "staff") {
		t.Fatal("role check mismatch")
	}
	if calls != 1 {
		t.Fatalf("expected 1 loader call (cached), got %d", calls)
	}

	// Expired local entries are reloaded.
	acc.mu.Lock()
	acc.cache[7].expiresAt = time.Now().Add(-time.Second)
	acc.mu.Unlock()
	acc.HasPermission(ctx, "user.index")
	if calls != 2 {
		t.Fatalf("expected reload after TTL, got %d calls", calls)
	}

	acc.Invalidate(7)
	acc.HasPermission(ctx, "user.index")
	if calls != 3 {
		t.Fatalf("expected reload after Invalidate, got %d calls", calls)
	}
}

func TestAccessDeniesWithoutCaller(t *testing.T) {
	acc := NewAccess(nil, func(uint) ([]string, []string, error) { return nil, []string{"x"}, nil })
	if acc.HasPermission(context.Background(), "x") {
		t.Fatal("no caller in context must be denied")
	}
}
