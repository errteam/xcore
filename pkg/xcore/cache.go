package xcore

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// CacheItem represents a cached item with expiration
type CacheItem struct {
	Value      interface{}
	Expiration time.Time
	CreatedAt  time.Time
}

// IsExpired checks if the cache item is expired
func (i *CacheItem) IsExpired() bool {
	if i.Expiration.IsZero() {
		return false
	}
	return time.Now().After(i.Expiration)
}

// Cache defines the interface for cache operations
type Cache interface {
	// Get retrieves a value from the cache
	Get(ctx context.Context, key string) (interface{}, bool, error)

	// GetBytes retrieves a value as bytes from the cache
	GetBytes(ctx context.Context, key string) ([]byte, bool, error)

	// GetJSON retrieves a value and unmarshals it into the provided pointer
	GetJSON(ctx context.Context, key string, target interface{}) (bool, error)

	// Set stores a value in the cache with an optional TTL
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// SetBytes stores bytes in the cache with an optional TTL
	SetBytes(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// SetJSON stores a value as JSON in the cache with an optional TTL
	SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Delete removes a value from the cache
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists in the cache
	Exists(ctx context.Context, key string) (bool, error)

	// Clear removes all values from the cache
	Clear(ctx context.Context) error

	// Keys returns all keys in the cache (optionally matching a pattern)
	Keys(ctx context.Context, pattern string) ([]string, error)

	// Count returns the number of items in the cache
	Count(ctx context.Context) (int, error)

	// Close closes the cache connection
	Close() error
}

// MemoryCache is an in-memory cache implementation
type MemoryCache struct {
	mu      sync.RWMutex
	items   map[string]*CacheItem
	maxSize int
}

// MemoryCacheConfig holds configuration for memory cache
type MemoryCacheConfig struct {
	// MaxSize is the maximum number of items in the cache (0 = unlimited)
	MaxSize int
	// CleanupInterval is how often to clean up expired items (default: 1 minute)
	CleanupInterval time.Duration
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(cfg *MemoryCacheConfig) *MemoryCache {
	if cfg == nil {
		cfg = &MemoryCacheConfig{}
	}

	cache := &MemoryCache{
		items: make(map[string]*CacheItem),
	}

	if cfg.MaxSize > 0 {
		cache.maxSize = cfg.MaxSize
	}

	// Start cleanup goroutine
	cleanupInterval := cfg.CleanupInterval
	if cleanupInterval == 0 {
		cleanupInterval = time.Minute
	}

	go cache.startCleanup(cleanupInterval)

	return cache
}

// startCleanup periodically removes expired items
func (c *MemoryCache) startCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup removes expired items
func (c *MemoryCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, item := range c.items {
		if item.IsExpired() {
			delete(c.items, key)
		}
	}
}

// Get retrieves a value from the cache
func (c *MemoryCache) Get(ctx context.Context, key string) (interface{}, bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false, nil
	}

	if item.IsExpired() {
		return nil, false, nil
	}

	return item.Value, true, nil
}

// GetBytes retrieves a value as bytes from the cache
func (c *MemoryCache) GetBytes(ctx context.Context, key string) ([]byte, bool, error) {
	value, exists, err := c.Get(ctx, key)
	if err != nil || !exists {
		return nil, false, err
	}

	if bytes, ok := value.([]byte); ok {
		return bytes, true, nil
	}

	return nil, false, fmt.Errorf("value is not of type []byte")
}

// GetJSON retrieves a value and unmarshals it into the provided pointer
func (c *MemoryCache) GetJSON(ctx context.Context, key string, target interface{}) (bool, error) {
	value, exists, err := c.Get(ctx, key)
	if err != nil || !exists {
		return false, err
	}

	// If value is already bytes, unmarshal
	if bytes, ok := value.([]byte); ok {
		return true, json.Unmarshal(bytes, target)
	}

	// Otherwise, marshal and unmarshal (for interface{} values)
	bytes, err := json.Marshal(value)
	if err != nil {
		return false, err
	}

	return true, json.Unmarshal(bytes, target)
}

// Set stores a value in the cache with an optional TTL
func (c *MemoryCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check max size
	if c.maxSize > 0 && len(c.items) >= c.maxSize {
		// Simple eviction: remove oldest items
		c.evictOldest(c.maxSize / 10) // Evict 10% of items
	}

	item := &CacheItem{
		Value:     value,
		CreatedAt: time.Now(),
	}

	if ttl > 0 {
		item.Expiration = time.Now().Add(ttl)
	}

	c.items[key] = item
	return nil
}

// SetBytes stores bytes in the cache with an optional TTL
func (c *MemoryCache) SetBytes(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.Set(ctx, key, value, ttl)
}

// SetJSON stores a value as JSON in the cache with an optional TTL
func (c *MemoryCache) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.SetBytes(ctx, key, bytes, ttl)
}

// Delete removes a value from the cache
func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
	return nil
}

// Exists checks if a key exists in the cache
func (c *MemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return false, nil
	}

	if item.IsExpired() {
		return false, nil
	}

	return true, nil
}

// Clear removes all values from the cache
func (c *MemoryCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*CacheItem)
	return nil
}

// Keys returns all keys in the cache
func (c *MemoryCache) Keys(ctx context.Context, pattern string) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.items))
	for key, item := range c.items {
		if !item.IsExpired() {
			if pattern == "" || pattern == "*" || matchPattern(key, pattern) {
				keys = append(keys, key)
			}
		}
	}

	return keys, nil
}

// Count returns the number of items in the cache
func (c *MemoryCache) Count(ctx context.Context) (int, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	for _, item := range c.items {
		if !item.IsExpired() {
			count++
		}
	}

	return count, nil
}

// Close closes the cache (no-op for memory cache)
func (c *MemoryCache) Close() error {
	return nil
}

// evictOldest removes the oldest items from the cache
func (c *MemoryCache) evictOldest(count int) {
	// Simple implementation: remove first N items
	// In production, you might want to use LRU eviction
	evicted := 0
	for key := range c.items {
		if evicted >= count {
			break
		}
		delete(c.items, key)
		evicted++
	}
}

// matchPattern checks if a key matches a simple pattern (* wildcard only)
func matchPattern(key, pattern string) bool {
	if pattern == "*" {
		return true
	}

	// Simple prefix/suffix matching
	if len(pattern) > 0 {
		if pattern[0] == '*' && pattern[len(pattern)-1] == '*' {
			// Contains
			return containsIgnoreCase(key, pattern[1:len(pattern)-1])
		}
		if pattern[0] == '*' {
			// Suffix
			return hasSuffixIgnoreCase(key, pattern[1:])
		}
		if pattern[len(pattern)-1] == '*' {
			// Prefix
			return hasPrefixIgnoreCase(key, pattern[:len(pattern)-1])
		}
	}

	return key == pattern
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsIgnoreCaseSimple(s, substr))
}

func containsIgnoreCaseSimple(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func hasSuffixIgnoreCase(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func hasPrefixIgnoreCase(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// CacheStats holds cache statistics
type CacheStats struct {
	Hits      uint64  `json:"hits"`
	Misses    uint64  `json:"misses"`
	Sets      uint64  `json:"sets"`
	Deletes   uint64  `json:"deletes"`
	HitRatio  float64 `json:"hit_ratio"`
	ItemCount int     `json:"item_count"`
}

// CachedValue is a helper for type-safe caching
type CachedValue[T any] struct {
	cache Cache
}

// NewCachedValue creates a new typed cached value helper
func NewCachedValue[T any](cache Cache) *CachedValue[T] {
	return &CachedValue[T]{cache: cache}
}

// Get retrieves a typed value from the cache
func (cv *CachedValue[T]) Get(ctx context.Context, key string) (T, bool, error) {
	var zero T
	value, exists, err := cv.cache.Get(ctx, key)
	if err != nil || !exists {
		return zero, false, err
	}

	if typed, ok := value.(T); ok {
		return typed, true, nil
	}

	return zero, false, fmt.Errorf("value is not of expected type")
}

// GetOrSet retrieves a value from cache, or sets it using the provided function
func (cv *CachedValue[T]) GetOrSet(ctx context.Context, key string, ttl time.Duration, fn func() (T, error)) (T, bool, error) {
	// Try to get from cache
	value, exists, err := cv.Get(ctx, key)
	if err == nil && exists {
		return value, true, nil
	}

	// Get fresh value
	freshValue, err := fn()
	if err != nil {
		var zero T
		return zero, false, err
	}

	// Set in cache
	if err := cv.cache.Set(ctx, key, freshValue, ttl); err != nil {
		var zero T
		return zero, false, err
	}

	return freshValue, false, nil
}

// Set stores a typed value in the cache
func (cv *CachedValue[T]) Set(ctx context.Context, key string, value T, ttl time.Duration) error {
	return cv.cache.Set(ctx, key, value, ttl)
}

// Delete removes a typed value from the cache
func (cv *CachedValue[T]) Delete(ctx context.Context, key string) error {
	return cv.cache.Delete(ctx, key)
}
