package xcore

import (
	"net/http"
	"os"
	"testing"
	"time"
)

func TestNewGraceful_DefaultTimeout(t *testing.T) {
	g := NewGraceful(0, nil)
	if g.timeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", g.timeout)
	}
}

func TestNewGraceful_CustomTimeout(t *testing.T) {
	g := NewGraceful(60, nil)
	if g.timeout != 60*time.Second {
		t.Errorf("expected 60s timeout, got %v", g.timeout)
	}
}

func TestNewGraceful_NegativeTimeout(t *testing.T) {
	g := NewGraceful(-5, nil)
	if g.timeout != 30*time.Second {
		t.Errorf("expected default timeout for negative input, got %v", g.timeout)
	}
}

func TestGraceful_SetCallbackTimeout(t *testing.T) {
	g := NewGraceful(10, nil)
	g.SetCallbackTimeout(5 * time.Second)
	if g.callbackTimeout != 5*time.Second {
		t.Errorf("expected callback timeout 5s, got %v", g.callbackTimeout)
	}
}

func TestGraceful_AddCallback(t *testing.T) {
	g := NewGraceful(10, nil)

	g.AddCallback(func() error {
		return nil
	})

	g.mu.RLock()
	if len(g.callbacks) != 1 {
		t.Errorf("expected 1 callback, got %d", len(g.callbacks))
	}
	g.mu.RUnlock()
}

func TestGraceful_AddCallbackFunc(t *testing.T) {
	g := NewGraceful(10, nil)

	g.AddCallbackFunc("test_callback", func() error {
		return nil
	})

	g.mu.RLock()
	if len(g.callbacks) != 1 {
		t.Errorf("expected 1 callback, got %d", len(g.callbacks))
	}
	g.mu.RUnlock()
}

func TestGraceful_AddServer(t *testing.T) {
	g := NewGraceful(10, nil)

	server := &http.Server{}
	g.AddServer(server)

	g.mu.RLock()
	if len(g.callbacks) != 1 {
		t.Errorf("expected 1 callback, got %d", len(g.callbacks))
	}
	g.mu.RUnlock()
}

func TestGraceful_AddServer_Nil(t *testing.T) {
	g := NewGraceful(10, nil)
	g.AddServer(nil)

	g.mu.RLock()
	if len(g.callbacks) != 1 {
		t.Errorf("expected 1 callback for nil server (callback still registered), got %d", len(g.callbacks))
	}
	g.mu.RUnlock()
}

func TestGraceful_AddDatabase(t *testing.T) {
	g := NewGraceful(10, nil)

	db := &Database{}
	g.AddDatabase(db)

	g.mu.RLock()
	if len(g.callbacks) != 1 {
		t.Errorf("expected 1 callback, got %d", len(g.callbacks))
	}
	g.mu.RUnlock()
}

func TestGraceful_AddCache(t *testing.T) {
	g := NewGraceful(10, nil)

	cache := &MemoryCache{data: make(map[string]cacheItem)}
	g.AddCache(cache)

	g.mu.RLock()
	if len(g.callbacks) != 1 {
		t.Errorf("expected 1 callback, got %d", len(g.callbacks))
	}
	g.mu.RUnlock()
}

func TestGraceful_AddWebSocket(t *testing.T) {
	g := NewGraceful(10, nil)

	ws := &WebSocket{}
	g.AddWebSocket(ws)

	g.mu.RLock()
	if len(g.callbacks) != 1 {
		t.Errorf("expected 1 callback, got %d", len(g.callbacks))
	}
	g.mu.RUnlock()
}

func TestGraceful_Wait(t *testing.T) {
	g := NewGraceful(10, nil)
	g.StartForTest()

	g.wg.Add(1)
	go func() {
		time.Sleep(10 * time.Millisecond)
		g.wg.Done()
	}()

	g.Shutdown()

	done := make(chan struct{})
	go func() {
		g.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("Wait() timed out")
	}
}

func TestGraceful_Run_AlreadyStarted(t *testing.T) {
	g := NewGraceful(10, nil)
	g.StartForTest()

	g.AddCallback(func() error { return nil })

	// Run should return immediately if already started
	g.Run()

	// Cleanup
	g.Shutdown()
}

func TestGraceful_Shutdown(t *testing.T) {
	g := NewGraceful(10, nil)

	g.AddCallback(func() error {
		return nil
	})

	g.Run()

	// Shutdown should not block
	g.Shutdown()

	// Give time for signal to be processed
	time.Sleep(50 * time.Millisecond)
}

func TestGraceful_SignalChannel(t *testing.T) {
	g := NewGraceful(10, nil)

	ch := g.SignalChannel()
	if ch == nil {
		t.Error("SignalChannel returned nil")
	}
}

// Helper to start graceful without actual signal handling
func (g *Graceful) StartForTest() {
	if g.started {
		return
	}
	g.started = true
	g.signalChan = make(chan os.Signal, 1)
}
