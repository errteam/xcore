package xcore

import (
	"context"
	"fmt"
	"time"
)

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

type CacheTags interface {
	SetTags(ctx context.Context, key string, tags ...string) error
	GetTags(ctx context.Context, key string) ([]string, error)
	InvalidateByTag(ctx context.Context, tag string) error
	InvalidateByTags(ctx context.Context, tags ...string) error
}

var ErrKeyNotFound = fmt.Errorf("key not found")
var ErrKeyExpired = fmt.Errorf("key expired")

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
