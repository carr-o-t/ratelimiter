package ratelimiter

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestManagerSameKeySharesBucket(t *testing.T) {
	m, err := NewManager(2, 1, time.Hour, time.Minute, time.Second)
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}
	defer m.Close()

	if !m.Allow("user-1") {
		t.Fatal("expected first request for same key to pass")
	}
	if !m.Allow("user-1") {
		t.Fatal("expected second request for same key to pass")
	}
	if m.Allow("user-1") {
		t.Fatal("expected third request for same key to be blocked")
	}
}

func TestManagerDifferentKeysIndependent(t *testing.T) {
	m, err := NewManager(1, 1, time.Hour, time.Minute, time.Second)
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}
	defer m.Close()

	if !m.Allow("user-a") {
		t.Fatal("expected first request for user-a to pass")
	}
	if !m.Allow("user-b") {
		t.Fatal("expected first request for user-b to pass")
	}
	if m.Allow("user-a") {
		t.Fatal("expected second request for user-a to be blocked")
	}
	if m.Allow("user-b") {
		t.Fatal("expected second request for user-b to be blocked")
	}
}

func TestManagerConcurrentRequestsAcrossMultipleKeys(t *testing.T) {
	const (
		keys           = 10
		capacity       = int64(3)
		requestsPerKey = 20
	)

	m, err := NewManager(capacity, 1, time.Hour, time.Minute, time.Second)
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}
	defer m.Close()

	var wg sync.WaitGroup
	allowedByKey := make([]int64, keys)

	for i := range keys {
		key := fmt.Sprintf("user-%d", i)
		idx := i
		for range requestsPerKey {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if m.Allow(key) {
					atomic.AddInt64(&allowedByKey[idx], 1)
				}
			}()
		}
	}

	wg.Wait()

	for i := range keys {
		if allowedByKey[i] != capacity {
			t.Fatalf("expected key user-%d to allow %d requests, got %d", i, capacity, allowedByKey[i])
		}
	}
}

func TestManagerConcurrentRequestsSameKey(t *testing.T) {
	const (
		capacity = int64(100)
		total    = 500
	)

	m, err := NewManager(capacity, 1, time.Hour, time.Minute, time.Second)
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}
	defer m.Close()

	var wg sync.WaitGroup
	var allowed int64

	for range total {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if m.Allow("shared-user") {
				atomic.AddInt64(&allowed, 1)
			}
		}()
	}

	wg.Wait()

	if allowed != capacity {
		t.Fatalf("expected %d requests to pass for same key, got %d", capacity, allowed)
	}
}

func TestManagerCleanupRemovesInactiveBucket(t *testing.T) {
	m, err := NewManager(2, 1, time.Hour, 30*time.Millisecond, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error creating manager: %v", err)
	}
	defer m.Close()

	if !m.Allow("inactive-user") {
		t.Fatal("expected initial request to create bucket")
	}

	time.Sleep(120 * time.Millisecond)

	m.mu.Lock()
	_, exists := m.buckets["inactive-user"]
	m.mu.Unlock()

	if exists {
		t.Fatal("expected inactive bucket to be cleaned up")
	}
}

func TestManagerCleanupGoroutineDoesNotLeak(t *testing.T) {
	base := runtime.NumGoroutine()

	log.Println("Goroutines before:", base)

	const managers = 30
	for range managers {
		m, err := NewManager(2, 1, time.Hour, time.Minute, 5*time.Millisecond)
		if err != nil {
			t.Fatalf("unexpected error creating manager: %v", err)
		}
		m.Close()
	}

	time.Sleep(50 * time.Millisecond)
	after := runtime.NumGoroutine()

	log.Println("Goroutines after:", after)

	// allow small background scheduling jitter
	if after > base+5 {
		t.Fatalf("possible goroutine leak: before=%d after=%d", base, after)
	}
}
