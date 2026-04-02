// Package xcore provides file-based cache implementation.
//
// This package implements a file-based cache that stores cached data
// as JSON files in a specified directory.
package xcore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileCache is a file-based cache implementation.
// Data is stored as JSON files in the specified directory.
type FileCache struct {
	mu       sync.RWMutex
	data     map[string]fileCacheItem
	basePath string
	ttl      time.Duration
}

type fileCacheItem struct {
	filename   string
	expiration time.Time
}

func NewFileCache(basePath string, ttl int) (*FileCache, error) {
	if basePath == "" {
		basePath = "./cache"
	}

	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, err
	}

	return &FileCache{
		data:     make(map[string]fileCacheItem),
		basePath: basePath,
		ttl:      time.Duration(ttl) * time.Second,
	}, nil
}

func (c *FileCache) Get(ctx context.Context, key string) (interface{}, error) {
	c.mu.RLock()
	item, found := c.data[key]
	c.mu.RUnlock()

	if !found {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	if time.Now().After(item.expiration) {
		_ = c.Delete(ctx, key)
		return nil, fmt.Errorf("key expired: %s", key)
	}

	data, err := os.ReadFile(item.filename)
	if err != nil {
		return nil, err
	}

	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}

	return value, nil
}

func (c *FileCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ttl <= 0 {
		ttl = c.ttl
	}

	safeKey := sanitizeFilename(key)
	filename := filepath.Join(c.basePath, safeKey+".cache")
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return err
	}

	expiration := time.Now().Add(ttl)
	c.data[key] = fileCacheItem{
		filename:   filename,
		expiration: expiration,
	}

	return nil
}

func (c *FileCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, found := c.data[key]
	if found {
		os.Remove(item.filename)
		delete(c.data, key)
	}

	return nil
}

func (c *FileCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, item := range c.data {
		os.Remove(item.filename)
	}

	c.data = make(map[string]fileCacheItem)
	return nil
}

func (c *FileCache) Exists(ctx context.Context, key string) (bool, error) {
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

func (c *FileCache) Keys(ctx context.Context, pattern string) ([]string, error) {
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

func (c *FileCache) TTL(ctx context.Context, key string) (time.Duration, error) {
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

func (c *FileCache) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
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

		data, err := os.ReadFile(item.filename)
		if err != nil {
			results[i] = nil
			continue
		}

		var value interface{}
		if err := json.Unmarshal(data, &value); err != nil {
			results[i] = nil
			continue
		}
		results[i] = value
	}

	return results, nil
}

func (c *FileCache) MSet(ctx context.Context, items map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errs []error
	now := time.Now()

	for key, value := range items {
		safeKey := sanitizeFilename(key)
		filename := filepath.Join(c.basePath, safeKey+".cache")
		data, err := json.Marshal(value)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to marshal key %s: %w", key, err))
			continue
		}

		if err := os.WriteFile(filename, data, 0644); err != nil {
			errs = append(errs, fmt.Errorf("failed to write file for key %s: %w", key, err))
			continue
		}

		c.data[key] = fileCacheItem{
			filename:   filename,
			expiration: now.Add(c.ttl),
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("MSet failed for %d keys: %v", len(errs), errs)
	}

	return nil
}

func (c *FileCache) Close() error {
	return nil
}

func (c *FileCache) Tags() CacheTags {
	return &fileTagCache{cache: c}
}

type fileTagCache struct {
	cache *FileCache
}

func (t *fileTagCache) SetTags(ctx context.Context, key string, tags ...string) error {
	return fmt.Errorf("tags not supported for file cache")
}

func (t *fileTagCache) GetTags(ctx context.Context, key string) ([]string, error) {
	return nil, fmt.Errorf("tags not supported for file cache")
}

func (t *fileTagCache) InvalidateByTag(ctx context.Context, tag string) error {
	return fmt.Errorf("tags not supported for file cache")
}

func (t *fileTagCache) InvalidateByTags(ctx context.Context, tags ...string) error {
	return fmt.Errorf("tags not supported for file cache")
}
