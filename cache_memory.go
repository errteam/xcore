// Package xcore provides an in-memory cache implementation.
//
// The MemoryCache implements the Cache interface using a concurrent-safe map.
// It supports TTL (time-to-live) for entries, automatic cleanup of expired entries,
// and cache tagging for grouped invalidation.
package xcore

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MemoryCache is an in-memory cache implementation with TTL and tagging support.
// It uses a sync.RWMutex for concurrent access and runs a background goroutine
// to clean up expired entries.
type MemoryCache struct {
	mu          sync.RWMutex
	data        map[string]cacheItem
	tags        map[string]map[string]bool
	cleanup     time.Duration
	stopCleanup chan struct{}
}

// cacheItem represents a single cache entry with value, expiration time, and optional tags.
type cacheItem struct {
	value      interface{}
	expiration time.Time
	tags       []string
}

// NewMemoryCache creates a new in-memory cache with the specified cleanup interval.
// If cleanupInterval <= 0, defaults to 60 seconds.
// Starts a background goroutine for cleaning up expired entries.
func NewMemoryCache(cleanupInterval int) *MemoryCache {
	interval := time.Duration(cleanupInterval) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}

	c := &MemoryCache{
		data:        make(map[string]cacheItem),
		tags:        make(map[string]map[string]bool),
		cleanup:     interval,
		stopCleanup: make(chan struct{}),
	}

	go c.cleanupExpired()

	return c
}

// Get retrieves a value from the cache by key.
// Returns the value if found and not expired, or an error if not found or expired.
func (c *MemoryCache) Get(ctx context.Context, key string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, found := c.data[key]
	if !found {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	if time.Now().After(item.expiration) {
		return nil, fmt.Errorf("key expired: %s", key)
	}

	return item.value, nil
}

// Set stores a value in the cache with the specified TTL.
// If ttl <= 0, defaults to 1 year.
func (c *MemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiration := time.Now().Add(ttl)
	if ttl <= 0 {
		expiration = time.Now().Add(365 * 24 * time.Hour)
	}

	c.data[key] = cacheItem{
		value:      value,
		expiration: expiration,
	}

	return nil
}

// Delete removes a key from the cache.
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.data, key)
	return nil
}

// Clear removes all entries from the cache.
func (c *MemoryCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = make(map[string]cacheItem)
	return nil
}

// Exists checks if a key exists and is not expired.
// Returns true if the key exists and is valid, false otherwise.
func (c *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, found := c.data[key]
	if !found {
		return false, nil
	}

	if time.Now().After(item.expiration) {
		return false, nil
	}

	return true, nil
}

// cleanupExpired runs periodically to remove expired entries from the cache.
// Uses a ticker for periodic cleanup. Stops when stopCleanup channel is closed.
func (c *MemoryCache) cleanupExpired() {
	ticker := time.NewTicker(c.cleanup)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for key, item := range c.data {
				if now.After(item.expiration) {
					delete(c.data, key)
				}
			}
			c.mu.Unlock()
		case <-c.stopCleanup:
			return
		}
	}
}

// Keys returns all non-expired keys matching the pattern.
// If pattern is empty or "*", returns all keys.
func (c *MemoryCache) Keys(ctx context.Context, pattern string) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var keys []string
	now := time.Now()
	for key, item := range c.data {
		if now.Before(item.expiration) {
			if pattern == "" || matchesPattern(key, pattern) {
				keys = append(keys, key)
			}
		}
	}
	return keys, nil
}

// TTL returns the remaining time-to-live for a key.
// Returns an error if the key is not found or has expired.
func (c *MemoryCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, found := c.data[key]
	if !found {
		return 0, fmt.Errorf("key not found: %s", key)
	}

	if time.Now().After(item.expiration) {
		return 0, fmt.Errorf("key expired: %s", key)
	}

	return time.Until(item.expiration), nil
}

// MGet retrieves multiple values by keys.
// Returns a slice with nil values for missing or expired keys.
func (c *MemoryCache) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	results := make([]interface{}, len(keys))
	now := time.Now()

	for i, key := range keys {
		item, found := c.data[key]
		if !found || now.After(item.expiration) {
			results[i] = nil
			continue
		}
		results[i] = item.value
	}

	return results, nil
}

// MSet sets multiple key-value pairs at once.
// All keys are set with a default TTL of 1 year.
func (c *MemoryCache) MSet(ctx context.Context, items map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, value := range items {
		c.data[key] = cacheItem{
			value:      value,
			expiration: now.Add(365 * 24 * time.Hour),
		}
	}

	return nil
}

// matchesPattern checks if a key matches a simple pattern.
// Currently only supports exact match or empty/wildcard pattern.
func matchesPattern(key, pattern string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	return key == pattern
}

// sanitizeFilename converts a cache key to a safe filename.
// Replaces characters that are invalid in filenames: / \ : * ? " < > |
func sanitizeFilename(key string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(key)
}

// Close stops the background cleanup goroutine.
func (c *MemoryCache) Close() error {
	close(c.stopCleanup)
	return nil
}

// Tags returns a CacheTags implementation for managing cache tags.
func (c *MemoryCache) Tags() CacheTags {
	return &memoryTagCache{cache: c}
}

// memoryTagCache implements CacheTags for the MemoryCache.
// It maintains a reverse index mapping tags to keys.
type memoryTagCache struct {
	cache *MemoryCache
}

// SetTags associates tags with a cache key.
// If the key doesn't exist, returns an error.
func (t *memoryTagCache) SetTags(ctx context.Context, key string, tags ...string) error {
	t.cache.mu.Lock()
	defer t.cache.mu.Unlock()

	if item, ok := t.cache.data[key]; ok {
		item.tags = tags
		t.cache.data[key] = item
	} else {
		return fmt.Errorf("key not found: %s", key)
	}

	for _, tag := range tags {
		if t.cache.tags[tag] == nil {
			t.cache.tags[tag] = make(map[string]bool)
		}
		t.cache.tags[tag][key] = true
	}
	return nil
}

// GetTags returns the tags associated with a cache key.
func (t *memoryTagCache) GetTags(ctx context.Context, key string) ([]string, error) {
	t.cache.mu.RLock()
	defer t.cache.mu.RUnlock()

	if item, ok := t.cache.data[key]; ok {
		return item.tags, nil
	}
	return nil, fmt.Errorf("key not found: %s", key)
}

// InvalidateByTag removes all cache entries with the specified tag.
// Also removes the tag from the tag index.
func (t *memoryTagCache) InvalidateByTag(ctx context.Context, tag string) error {
	t.cache.mu.Lock()
	defer t.cache.mu.Unlock()

	if keys, ok := t.cache.tags[tag]; ok {
		for key := range keys {
			delete(t.cache.data, key)
		}
		delete(t.cache.tags, tag)
	}
	return nil
}

// InvalidateByTags invalidates cache entries matching any of the given tags.
// Calls InvalidateByTag for each tag.
func (t *memoryTagCache) InvalidateByTags(ctx context.Context, tags ...string) error {
	for _, tag := range tags {
		if err := t.InvalidateByTag(ctx, tag); err != nil {
			return err
		}
	}
	return nil
}
