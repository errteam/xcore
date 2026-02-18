//go:build redis
// +build redis

package xcore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache is a Redis-backed cache implementation
type RedisCache struct {
	client  *redis.Client
	prefix  string
}

// RedisCacheConfig holds configuration for Redis cache
type RedisCacheConfig struct {
	// Addr is the Redis server address (host:port)
	Addr string
	// Password is the Redis password (optional)
	Password string
	// DB is the Redis database number (default: 0)
	DB int
	// Prefix is the key prefix for all cache operations
	Prefix string
	// PoolSize is the maximum number of socket connections
	PoolSize int
	// DialTimeout is the timeout for dialing connections
	DialTimeout time.Duration
	// ReadTimeout is the timeout for reading from connections
	ReadTimeout time.Duration
	// WriteTimeout is the timeout for writing to connections
	WriteTimeout time.Duration
}

// NewRedisCache creates a new Redis cache
func NewRedisCache(cfg *RedisCacheConfig) (*RedisCache, error) {
	if cfg == nil {
		cfg = &RedisCacheConfig{}
	}

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{
		client: client,
		prefix: cfg.Prefix,
	}, nil
}

// NewRedisCacheWithClient creates a Redis cache with an existing client
func NewRedisCacheWithClient(client *redis.Client, prefix string) *RedisCache {
	return &RedisCache{
		client: client,
		prefix: prefix,
	}
}

// key returns the prefixed key
func (c *RedisCache) key(k string) string {
	if c.prefix == "" {
		return k
	}
	return c.prefix + ":" + k
}

// Get retrieves a value from the cache
func (c *RedisCache) Get(ctx context.Context, key string) (interface{}, bool, error) {
	val, err := c.client.Get(ctx, c.key(key)).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return val, true, nil
}

// GetBytes retrieves a value as bytes from the cache
func (c *RedisCache) GetBytes(ctx context.Context, key string) ([]byte, bool, error) {
	val, exists, err := c.Get(ctx, key)
	if err != nil || !exists {
		return nil, false, err
	}
	if bytes, ok := val.([]byte); ok {
		return bytes, true, nil
	}
	return nil, false, fmt.Errorf("value is not of type []byte")
}

// GetJSON retrieves a value and unmarshals it into the provided pointer
func (c *RedisCache) GetJSON(ctx context.Context, key string, target interface{}) (bool, error) {
	val, exists, err := c.Get(ctx, key)
	if err != nil || !exists {
		return false, err
	}
	if bytes, ok := val.([]byte); ok {
		return true, json.Unmarshal(bytes, target)
	}
	return false, fmt.Errorf("value is not of type []byte")
}

// Set stores a value in the cache with an optional TTL
func (c *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	var bytes []byte
	var err error

	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		bytes, err = json.Marshal(value)
		if err != nil {
			return err
		}
	}

	if ttl > 0 {
		return c.client.SetEX(ctx, c.key(key), bytes, ttl).Err()
	}
	return c.client.Set(ctx, c.key(key), bytes, 0).Err()
}

// SetBytes stores bytes in the cache with an optional TTL
func (c *RedisCache) SetBytes(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return c.Set(ctx, key, value, ttl)
}

// SetJSON stores a value as JSON in the cache with an optional TTL
func (c *RedisCache) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	bytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.SetBytes(ctx, key, bytes, ttl)
}

// Delete removes a value from the cache
func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, c.key(key)).Err()
}

// Exists checks if a key exists in the cache
func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	result, err := c.client.Exists(ctx, c.key(key)).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

// Clear removes all values from the cache (with prefix)
func (c *RedisCache) Clear(ctx context.Context) error {
	if c.prefix == "" {
		return fmt.Errorf("clearing all keys is not allowed without a prefix")
	}

	keys, err := c.Keys(ctx, "*")
	if err != nil {
		return err
	}

	if len(keys) > 0 {
		prefixedKeys := make([]string, len(keys))
		for i, k := range keys {
			prefixedKeys[i] = c.key(k)
		}
		return c.client.Del(ctx, prefixedKeys...).Err()
	}

	return nil
}

// Keys returns all keys in the cache matching the pattern
func (c *RedisCache) Keys(ctx context.Context, pattern string) ([]string, error) {
	if pattern == "" {
		pattern = "*"
	}
	return c.client.Keys(ctx, c.key(pattern)).Result()
}

// Count returns the number of items in the cache
func (c *RedisCache) Count(ctx context.Context) (int, error) {
	keys, err := c.Keys(ctx, "*")
	if err != nil {
		return 0, err
	}
	return len(keys), nil
}

// Close closes the Redis connection
func (c *RedisCache) Close() error {
	return c.client.Close()
}

// Increment increments a counter
func (c *RedisCache) Increment(ctx context.Context, key string, value int64) (int64, error) {
	return c.client.IncrBy(ctx, c.key(key), value).Result()
}

// Decrement decrements a counter
func (c *RedisCache) Decrement(ctx context.Context, key string, value int64) (int64, error) {
	return c.client.DecrBy(ctx, c.key(key), value).Result()
}

// GetCounter gets a counter value
func (c *RedisCache) GetCounter(ctx context.Context, key string) (int64, error) {
	val, err := c.client.Get(ctx, c.key(key)).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// SetCounter sets a counter with expiration
func (c *RedisCache) SetCounter(ctx context.Context, key string, value int64, ttl time.Duration) error {
	return c.client.SetEX(ctx, c.key(key), value, ttl).Err()
}

// Lock acquires a distributed lock
func (c *RedisCache) Lock(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	return c.client.SetNX(ctx, c.key(key), "locked", ttl).Result()
}

// Unlock releases a distributed lock
func (c *RedisCache) Unlock(ctx context.Context, key string) error {
	return c.client.Del(ctx, c.key(key)).Err()
}

// WithLock executes a function while holding a lock
func (c *RedisCache) WithLock(ctx context.Context, key string, ttl time.Duration, fn func() error) error {
	acquired, err := c.Lock(ctx, key, ttl)
	if err != nil {
		return err
	}
	if !acquired {
		return fmt.Errorf("failed to acquire lock")
	}
	defer c.Unlock(ctx, key)
	return fn()
}

// Publish publishes a message to a channel
func (c *RedisCache) Publish(ctx context.Context, channel string, message interface{}) error {
	var msgBytes []byte
	var err error

	switch v := message.(type) {
	case []byte:
		msgBytes = v
	case string:
		msgBytes = []byte(v)
	default:
		msgBytes, err = json.Marshal(message)
		if err != nil {
			return err
		}
	}

	return c.client.Publish(ctx, channel, string(msgBytes)).Err()
}

// Subscribe subscribes to a channel
func (c *RedisCache) Subscribe(ctx context.Context, channel string) *redis.PubSub {
	return c.client.Subscribe(ctx, channel)
}

// Pipeline returns a Redis pipeline for batch operations
func (c *RedisCache) Pipeline() redis.Pipeliner {
	return c.client.Pipeline()
}

// Client returns the underlying Redis client
func (c *RedisCache) Client() *redis.Client {
	return c.client
}

// HealthCheck checks if Redis is healthy
func (c *RedisCache) HealthCheck(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

// Stats returns Redis statistics
func (c *RedisCache) Stats() map[string]interface{} {
	stats := c.client.PoolStats()
	return map[string]interface{}{
		"hits":         stats.Hits,
		"misses":       stats.Misses,
		"timeouts":     stats.Timeouts,
		"total_conns":  stats.TotalConns,
		"idle_conns":   stats.IdleConns,
		"stale_conns":  stats.StaleConns,
	}
}

// Helper functions for common patterns

// CacheFunc caches the result of a function
func (c *RedisCache) CacheFunc(ctx context.Context, key string, ttl time.Duration, fn func() (interface{}, error)) (interface{}, error) {
	// Try to get from cache
	val, exists, err := c.Get(ctx, key)
	if err == nil && exists {
		return val, nil
	}

	// Execute function
	val, err = fn()
	if err != nil {
		return nil, err
	}

	// Store in cache
	if err := c.Set(ctx, key, val, ttl); err != nil {
		return nil, err
	}

	return val, nil
}

// CacheJSONFunc caches the JSON result of a function
func (c *RedisCache) CacheJSONFunc(ctx context.Context, key string, ttl time.Duration, fn func() (interface{}, error)) (interface{}, error) {
	// Try to get from cache
	val, exists, err := c.Get(ctx, key)
	if err == nil && exists {
		return val, nil
	}

	// Execute function
	val, err = fn()
	if err != nil {
		return nil, err
	}

	// Store in cache as JSON
	if err := c.SetJSON(ctx, key, val, ttl); err != nil {
		return nil, err
	}

	return val, nil
}

// DeletePattern deletes all keys matching a pattern
func (c *RedisCache) DeletePattern(ctx context.Context, pattern string) error {
	keys, err := c.Keys(ctx, pattern)
	if err != nil {
		return err
	}

	if len(keys) > 0 {
		prefixedKeys := make([]string, len(keys))
		for i, k := range keys {
			prefixedKeys[i] = c.key(k)
		}
		return c.client.Del(ctx, prefixedKeys...).Err()
	}

	return nil
}

// ScanKeys scans keys matching a pattern (for large datasets)
func (c *RedisCache) ScanKeys(ctx context.Context, pattern string, count int64) ([]string, error) {
	var keys []string
	var cursor uint64

	pattern = c.key(pattern)

	for {
		var err error
		var keysPage []string
		keysPage, cursor, err = c.client.Scan(ctx, cursor, pattern, count).Result()
		if err != nil {
			return nil, err
		}

		keys = append(keys, keysPage...)

		if cursor == 0 {
			break
		}
	}

	// Remove prefix from keys
	if c.prefix != "" {
		prefix := c.prefix + ":"
		for i, k := range keys {
			keys[i] = strings.TrimPrefix(k, prefix)
		}
	}

	return keys, nil
}
