package rbac

import (
	"sync"
	"time"
)

// cachedPermissions holds cached permissions for a user
type cachedPermissions struct {
	Allowed   []string
	Denied    []uint
	ExpiresAt time.Time
}

// permissionCache caches user permissions
type permissionCache struct {
	mu    sync.RWMutex
	items map[uint]*cachedPermissions
	ttl   time.Duration
}

// newPermissionCache creates a new permission cache
func newPermissionCache(ttl time.Duration) *permissionCache {
	cache := &permissionCache{
		items: make(map[uint]*cachedPermissions),
		ttl:   ttl,
	}

	// Start cleanup goroutine
	go cache.startCleanup()

	return cache
}

// startCleanup periodically removes expired items
func (c *permissionCache) startCleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup removes expired items
func (c *permissionCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for userID, item := range c.items {
		if now.After(item.ExpiresAt) {
			delete(c.items, userID)
		}
	}
}

// GetUserPermissions gets cached permissions for a user
func (c *permissionCache) GetUserPermissions(userID uint) (*cachedPermissions, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[userID]
	if !exists {
		return nil, false
	}

	if time.Now().After(item.ExpiresAt) {
		return nil, false
	}

	return item, true
}

// SetUserPermissions caches permissions for a user
func (c *permissionCache) SetUserPermissions(userID uint, allowed []string, denied []uint) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[userID] = &cachedPermissions{
		Allowed:   allowed,
		Denied:    denied,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// InvalidateUser invalidates cache for a specific user
func (c *permissionCache) InvalidateUser(userID uint) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, userID)
}

// Clear clears all cached items
func (c *permissionCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[uint]*cachedPermissions)
}

// Count returns the number of cached items
func (c *permissionCache) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}
