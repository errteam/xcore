// Package xcore provides caching functionality for the xcore framework.
//
// This package defines the Cache interface and provides implementations for
// different cache backends. Supported drivers: memory, file, redis.
//
// The cache interface supports standard operations: Get, Set, Delete, Clear,
// Exists, Keys, TTL, MGet, MSet. It also supports tagging for cache invalidation.
package xcore

import (
	"context"
	"fmt"
	"time"
)

// Cache defines the interface for caching operations.
// Implementations can be in-memory, file-based, or distributed (e.g., Redis).
type Cache interface {
	Get(ctx context.Context, key string) (interface{}, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
	Exists(ctx context.Context, key string) (bool, error)
	Keys(ctx context.Context, pattern string) ([]string, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
	MGet(ctx context.Context, keys ...string) ([]interface{}, error)
	MSet(ctx context.Context, items map[string]interface{}) error
	Close() error
	Tags() CacheTags
}

// CacheTags defines the interface for cache tagging operations.
// Tags allow grouping related cache keys and invalidating them together.
type CacheTags interface {
	SetTags(ctx context.Context, key string, tags ...string) error
	GetTags(ctx context.Context, key string) ([]string, error)
	InvalidateByTag(ctx context.Context, tag string) error
	InvalidateByTags(ctx context.Context, tags ...string) error
}

// Cache errors.
var (
	ErrKeyNotFound = fmt.Errorf("key not found")
	ErrKeyExpired  = fmt.Errorf("key expired")
)

// NewCache creates a new Cache instance based on the configuration.
// If cfg is nil, defaults to memory cache with 60-second cleanup interval.
//
// Supported drivers:
//   - "memory": In-memory cache with TTL support
//   - "file": File-based cache
//   - "redis": Redis cache (requires valid config)
func NewCache(cfg *CacheConfig) (Cache, error) {
	if cfg == nil {
		return NewMemoryCache(60), nil
	}

	switch cfg.Driver {
	case "memory":
		return NewMemoryCache(cfg.CleanupInterval), nil
	case "file":
		return NewFileCache(cfg.FilePath, cfg.TTL)
	case "redis":
		return NewRedisCache(cfg)
	default:
		return NewMemoryCache(cfg.CleanupInterval), nil
	}
}
