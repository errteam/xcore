package xcore

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type MemoryCache struct {
	mu          sync.RWMutex
	data        map[string]cacheItem
	tags        map[string]map[string]bool
	cleanup     time.Duration
	stopCleanup chan struct{}
}

type cacheItem struct {
	value      interface{}
	expiration time.Time
	tags       []string
}

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

func (c *MemoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.data, key)
	return nil
}

func (c *MemoryCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = make(map[string]cacheItem)
	return nil
}

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

func matchesPattern(key, pattern string) bool {
	if pattern == "" || pattern == "*" {
		return true
	}
	return key == pattern
}

func sanitizeFilename(key string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(key)
}

func (c *MemoryCache) Close() error {
	close(c.stopCleanup)
	return nil
}

func (c *MemoryCache) Tags() CacheTags {
	return &memoryTagCache{cache: c}
}

type memoryTagCache struct {
	cache *MemoryCache
}

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

func (t *memoryTagCache) GetTags(ctx context.Context, key string) ([]string, error) {
	t.cache.mu.RLock()
	defer t.cache.mu.RUnlock()

	if item, ok := t.cache.data[key]; ok {
		return item.tags, nil
	}
	return nil, fmt.Errorf("key not found: %s", key)
}

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

func (t *memoryTagCache) InvalidateByTags(ctx context.Context, tags ...string) error {
	for _, tag := range tags {
		if err := t.InvalidateByTag(ctx, tag); err != nil {
			return err
		}
	}
	return nil
}
