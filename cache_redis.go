// Package xcore provides Redis cache implementation.
//
// This package implements a cache backed by Redis for distributed
// caching across multiple application instances.
package xcore

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisCache is a cache implementation backed by Redis.
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new Redis cache with the given configuration.
func NewRedisCache(cfg *CacheConfig) (*RedisCache, error) {
	opts := &redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}

	if cfg.RedisPoolSize > 0 {
		opts.PoolSize = cfg.RedisPoolSize
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisCache{client: client}, nil
}

func (c *RedisCache) Get(ctx context.Context, key string) (interface{}, error) {
	result, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return c.client.Set(ctx, key, value, ttl).Err()
}

func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, key).Err()
}

func (c *RedisCache) Clear(ctx context.Context) error {
	return c.client.FlushDB(ctx).Err()
}

func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.client.Exists(ctx, key).Result()
	return n > 0, err
}

func (c *RedisCache) Keys(ctx context.Context, pattern string) ([]string, error) {
	if pattern == "" {
		pattern = "*"
	}
	iter := c.client.Scan(ctx, 0, pattern, 0).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	return keys, iter.Err()
}

func (c *RedisCache) TTL(ctx context.Context, key string) (time.Duration, error) {
	dur, err := c.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if dur < 0 {
		return 0, fmt.Errorf("key not found: %s", key)
	}
	return dur, nil
}

func (c *RedisCache) MGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	results, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	interfaceResults := make([]interface{}, len(results))
	for i, v := range results {
		interfaceResults[i] = v
	}
	return interfaceResults, nil
}

func (c *RedisCache) MSet(ctx context.Context, items map[string]interface{}) error {
	pipe := c.client.Pipeline()
	for key, value := range items {
		pipe.Set(ctx, key, value, 24*time.Hour)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (c *RedisCache) Close() error {
	return c.client.Close()
}

func (c *RedisCache) Tags() CacheTags {
	return &redisTagCache{cache: c}
}

type redisTagCache struct {
	cache *RedisCache
}

func (t *redisTagCache) SetTags(ctx context.Context, key string, tags ...string) error {
	tagKey := "tag:" + key
	pipe := t.cache.client.Pipeline()
	for _, tag := range tags {
		pipe.SAdd(ctx, "tag:"+tag, key)
	}
	for _, tag := range tags {
		pipe.SAdd(ctx, tagKey, tag)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (t *redisTagCache) GetTags(ctx context.Context, key string) ([]string, error) {
	tagKey := "tag:" + key
	return t.cache.client.SMembers(ctx, tagKey).Result()
}

func (t *redisTagCache) InvalidateByTag(ctx context.Context, tag string) error {
	keys, err := t.cache.client.SMembers(ctx, "tag:"+tag).Result()
	if err != nil {
		return err
	}
	for _, key := range keys {
		t.cache.client.Del(ctx, "tag:"+key)
	}
	return t.cache.client.Del(ctx, "tag:"+tag).Err()
}

func (t *redisTagCache) InvalidateByTags(ctx context.Context, tags ...string) error {
	for _, tag := range tags {
		if err := t.InvalidateByTag(ctx, tag); err != nil {
			return err
		}
	}
	return nil
}
