package helpers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/reshap0318/go-boilerplate/internal/database"
)

// AccessLoader loads the role names and permission names held by a user.
// It is implemented by the repository layer (UserRoleRepository.FindAccessNames)
// and injected by the DI container, so this helper never builds SQL itself.
type AccessLoader func(userID uint) (roles []string, permissions []string, err error)

// localAccessTTL bounds how long the in-memory fallback cache may serve an entry.
// The local cache is per-process and cannot be invalidated by other instances,
// so entries must expire on their own.
const localAccessTTL = 5 * time.Minute

type userAccessData struct {
	permissions map[string]bool
	roles       map[string]bool
	expiresAt   time.Time
}

// accessCacheEntry is the JSON shape stored under the shared "session:%d" key.
// It matches dtos.UserDTO's roles/permissions fields, so it reads the same
// entry AuthLogin writes and writes back a shape AuthValidateToken can still read.
type accessCacheEntry struct {
	Roles []struct {
		Name string `json:"name"`
	} `json:"roles"`
	Permissions []struct {
		Name string `json:"name"`
	} `json:"permissions"`
}

// Access handles permission and role checking.
// Redis is used exclusively when available; the local in-memory cache is the
// fallback only while Redis is down. Either way, a miss falls back to the loader (DB).
type Access struct {
	redis  *database.RedisCache
	loader AccessLoader
	mu     sync.RWMutex
	cache  map[uint]*userAccessData
}

// NewAccess creates a new Access instance.
func NewAccess(redis *database.RedisCache, loader AccessLoader) *Access {
	return &Access{
		redis:  redis,
		loader: loader,
		cache:  make(map[uint]*userAccessData),
	}
}

// getUserAccess retrieves user permissions and roles.
// Redis alive: Redis → DB. Redis down: local cache → DB.
func (a *Access) getUserAccess(userID uint) (*userAccessData, bool) {
	if a.redis != nil && a.redis.IsCacheAvailable() {
		return a.getUserAccessFromRedis(userID)
	}
	return a.getUserAccessFromLocal(userID)
}

func sessionKey(userID uint) string {
	return fmt.Sprintf("session:%d", userID)
}

func (a *Access) getUserAccessFromRedis(userID uint) (*userAccessData, bool) {
	var cached accessCacheEntry
	if err := a.redis.GetJSON(sessionKey(userID), &cached); err == nil {
		return accessDataFromCacheEntry(&cached), true
	}

	data, err := a.load(userID)
	if err != nil {
		return nil, false
	}

	ttl := time.Duration(GetEnvInt("JWT_EXPIRATION", 24)) * time.Hour
	_ = a.redis.SetJSON(sessionKey(userID), accessCacheEntryFromData(data), ttl)

	return data, true
}

func (a *Access) getUserAccessFromLocal(userID uint) (*userAccessData, bool) {
	a.mu.RLock()
	data, ok := a.cache[userID]
	a.mu.RUnlock()
	if ok && time.Now().Before(data.expiresAt) {
		return data, true
	}

	data, err := a.load(userID)
	if err != nil {
		return nil, false
	}
	data.expiresAt = time.Now().Add(localAccessTTL)

	a.mu.Lock()
	a.cache[userID] = data
	a.mu.Unlock()

	return data, true
}

// load fetches role/permission names via the injected loader (one JOIN query).
func (a *Access) load(userID uint) (*userAccessData, error) {
	if a.loader == nil {
		return nil, fmt.Errorf("access loader not configured")
	}
	roles, perms, err := a.loader(userID)
	if err != nil {
		return nil, err
	}
	data := &userAccessData{
		permissions: make(map[string]bool, len(perms)),
		roles:       make(map[string]bool, len(roles)),
	}
	for _, r := range roles {
		data.roles[r] = true
	}
	for _, p := range perms {
		data.permissions[p] = true
	}
	return data, nil
}

func accessDataFromCacheEntry(cached *accessCacheEntry) *userAccessData {
	data := &userAccessData{
		permissions: make(map[string]bool),
		roles:       make(map[string]bool),
	}
	for _, r := range cached.Roles {
		data.roles[r.Name] = true
	}
	for _, p := range cached.Permissions {
		data.permissions[p.Name] = true
	}
	return data
}

func accessCacheEntryFromData(data *userAccessData) *accessCacheEntry {
	entry := &accessCacheEntry{}
	for role := range data.roles {
		entry.Roles = append(entry.Roles, struct {
			Name string `json:"name"`
		}{Name: role})
	}
	for perm := range data.permissions {
		entry.Permissions = append(entry.Permissions, struct {
			Name string `json:"name"`
		}{Name: perm})
	}
	return entry
}

// Warm loads a user's access data into the cache if it is not there yet.
// AuthValidateToken calls it after a DB fallback so later requests hit the cache.
func (a *Access) Warm(userID uint) {
	_, _ = a.getUserAccess(userID)
}

// HasPermission checks if the caller has ANY of the specified permissions.
func (a *Access) HasPermission(ctx context.Context, permissions ...string) bool {
	userID := GetCallerID(ctx)
	if userID == 0 {
		return false
	}

	data, ok := a.getUserAccess(userID)
	if !ok {
		return false
	}

	for _, perm := range permissions {
		if data.permissions[perm] {
			return true
		}
	}

	return false
}

// HasRole checks if the caller has the specified role.
func (a *Access) HasRole(ctx context.Context, role string) bool {
	userID := GetCallerID(ctx)
	if userID == 0 {
		return false
	}

	data, ok := a.getUserAccess(userID)
	if !ok {
		return false
	}

	return data.roles[role]
}

// Invalidate clears the cached access data for a user.
func (a *Access) Invalidate(userID uint) {
	a.mu.Lock()
	delete(a.cache, userID)
	a.mu.Unlock()

	if a.redis != nil && a.redis.IsCacheAvailable() {
		a.redis.Delete(sessionKey(userID))
	}
}
