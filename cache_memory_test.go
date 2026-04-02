package xcore

import (
	"context"
	"testing"
	"time"
)

func TestNewMemoryCache(t *testing.T) {
	cache := NewMemoryCache(60)
	if cache == nil {
		t.Error("NewMemoryCache returned nil")
	}
	if cache.data == nil {
		t.Error("data map should be initialized")
	}
}

func TestNewMemoryCache_DefaultCleanup(t *testing.T) {
	cache := NewMemoryCache(0)
	if cache.cleanup != 60*time.Second {
		t.Errorf("expected 60s cleanup interval, got %v", cache.cleanup)
	}
}

func TestMemoryCache_SetAndGet(t *testing.T) {
	cache := NewMemoryCache(60)
	ctx := context.Background()

	err := cache.Set(ctx, "key1", "value1", 10*time.Second)
	if err != nil {
		t.Errorf("Set failed: %v", err)
	}

	val, err := cache.Get(ctx, "key1")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if val != "value1" {
		t.Errorf("expected 'value1', got '%v'", val)
	}

	cache.Close()
}

func TestMemoryCache_Get_NotFound(t *testing.T) {
	cache := NewMemoryCache(60)
	ctx := context.Background()

	_, err := cache.Get(ctx, "nonexistent")
	if err == nil {
		t.Error("Get should fail for nonexistent key")
	}
}

func TestMemoryCache_Get_Expired(t *testing.T) {
	cache := NewMemoryCache(60)
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 1*time.Millisecond)
	time.Sleep(10 * time.Millisecond)

	_, err := cache.Get(ctx, "key1")
	if err == nil {
		t.Error("Get should fail for expired key")
	}

	cache.Close()
}

func TestMemoryCache_Delete(t *testing.T) {
	cache := NewMemoryCache(60)
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 10*time.Second)
	err := cache.Delete(ctx, "key1")
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	_, err = cache.Get(ctx, "key1")
	if err == nil {
		t.Error("Get should fail after delete")
	}

	cache.Close()
}

func TestMemoryCache_Clear(t *testing.T) {
	cache := NewMemoryCache(60)
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 10*time.Second)
	cache.Set(ctx, "key2", "value2", 10*time.Second)

	err := cache.Clear(ctx)
	if err != nil {
		t.Errorf("Clear failed: %v", err)
	}

	_, err = cache.Get(ctx, "key1")
	if err == nil {
		t.Error("Get should fail after clear")
	}

	_, err = cache.Get(ctx, "key2")
	if err == nil {
		t.Error("Get should fail after clear")
	}

	cache.Close()
}

func TestMemoryCache_Exists(t *testing.T) {
	cache := NewMemoryCache(60)
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 10*time.Second)

	exists, err := cache.Exists(ctx, "key1")
	if err != nil {
		t.Errorf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("key1 should exist")
	}

	exists, err = cache.Exists(ctx, "nonexistent")
	if err != nil {
		t.Errorf("Exists failed: %v", err)
	}
	if exists {
		t.Error("nonexistent key should not exist")
	}

	cache.Close()
}

func TestMemoryCache_Keys(t *testing.T) {
	cache := NewMemoryCache(60)
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 10*time.Second)
	cache.Set(ctx, "key2", "value2", 10*time.Second)
	cache.Set(ctx, "key3", "value3", 10*time.Second)

	keys, err := cache.Keys(ctx, "")
	if err != nil {
		t.Errorf("Keys failed: %v", err)
	}
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}

	keys, err = cache.Keys(ctx, "key1")
	if err != nil {
		t.Errorf("Keys failed: %v", err)
	}
	if len(keys) != 1 || keys[0] != "key1" {
		t.Errorf("expected [key1], got %v", keys)
	}

	cache.Close()
}

func TestMemoryCache_TTL(t *testing.T) {
	cache := NewMemoryCache(60)
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 10*time.Second)

	ttl, err := cache.TTL(ctx, "key1")
	if err != nil {
		t.Errorf("TTL failed: %v", err)
	}
	if ttl <= 0 {
		t.Error("TTL should be positive for non-expired key")
	}

	ttl, err = cache.TTL(ctx, "nonexistent")
	if err == nil {
		t.Error("TTL should fail for nonexistent key")
	}

	cache.Close()
}

func TestMemoryCache_MGet(t *testing.T) {
	cache := NewMemoryCache(60)
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 10*time.Second)
	cache.Set(ctx, "key2", "value2", 10*time.Second)

	results, err := cache.MGet(ctx, "key1", "key2", "key3")
	if err != nil {
		t.Errorf("MGet failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
	if results[0] != "value1" {
		t.Errorf("expected 'value1', got %v", results[0])
	}
	if results[1] != "value2" {
		t.Errorf("expected 'value2', got %v", results[1])
	}
	if results[2] != nil {
		t.Errorf("expected nil for nonexistent, got %v", results[2])
	}

	cache.Close()
}

func TestMemoryCache_MSet(t *testing.T) {
	cache := NewMemoryCache(60)
	ctx := context.Background()

	items := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}
	err := cache.MSet(ctx, items)
	if err != nil {
		t.Errorf("MSet failed: %v", err)
	}

	val1, _ := cache.Get(ctx, "key1")
	if val1 != "value1" {
		t.Errorf("expected 'value1', got %v", val1)
	}

	val2, _ := cache.Get(ctx, "key2")
	if val2 != "value2" {
		t.Errorf("expected 'value2', got %v", val2)
	}

	cache.Close()
}

func TestMemoryCache_Close(t *testing.T) {
	cache := NewMemoryCache(60)
	err := cache.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

func TestMemoryCache_Tags(t *testing.T) {
	cache := NewMemoryCache(60)
	tags := cache.Tags()
	if tags == nil {
		t.Error("Tags() returned nil")
	}

	ctx := context.Background()
	cache.Set(ctx, "key1", "value1", 10*time.Second)

	err := tags.SetTags(ctx, "key1", "tag1", "tag2")
	if err != nil {
		t.Errorf("SetTags failed: %v", err)
	}

	tagList, err := tags.GetTags(ctx, "key1")
	if err != nil {
		t.Errorf("GetTags failed: %v", err)
	}
	if len(tagList) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tagList))
	}

	cache.Close()
}

func TestMemoryCache_InvalidateByTag(t *testing.T) {
	cache := NewMemoryCache(60)
	tags := cache.Tags()
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 10*time.Second)
	cache.Set(ctx, "key2", "value2", 10*time.Second)
	tags.SetTags(ctx, "key1", "tag1")
	tags.SetTags(ctx, "key2", "tag1")

	err := tags.InvalidateByTag(ctx, "tag1")
	if err != nil {
		t.Errorf("InvalidateByTag failed: %v", err)
	}

	_, err = cache.Get(ctx, "key1")
	if err == nil {
		t.Error("key1 should be invalidated")
	}

	_, err = cache.Get(ctx, "key2")
	if err == nil {
		t.Error("key2 should be invalidated")
	}

	cache.Close()
}

func TestMemoryCache_InvalidateByTags(t *testing.T) {
	cache := NewMemoryCache(60)
	tags := cache.Tags()
	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 10*time.Second)
	tags.SetTags(ctx, "key1", "tag1", "tag2")

	err := tags.InvalidateByTags(ctx, "tag1", "tag2")
	if err != nil {
		t.Errorf("InvalidateByTags failed: %v", err)
	}

	_, err = cache.Get(ctx, "key1")
	if err == nil {
		t.Error("key1 should be invalidated")
	}

	cache.Close()
}
