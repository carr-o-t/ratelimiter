package core

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeRedisEntry struct {
	tokens       int64
	lastRefillMs int64
	lastSeenMs   int64
	expiresAtMs  int64
}

type fakeRedisEvalClient struct {
	mu   sync.Mutex
	data map[string]fakeRedisEntry
}

func newFakeRedisEvalClient() *fakeRedisEvalClient {
	return &fakeRedisEvalClient{
		data: make(map[string]fakeRedisEntry),
	}
}

func (c *fakeRedisEvalClient) Eval(_ context.Context, _ string, keys []string, args ...any) (any, error) {
	if len(keys) != 1 {
		return nil, fmt.Errorf("expected one key")
	}
	if len(args) != 5 {
		return nil, fmt.Errorf("expected five args")
	}

	capacity := toInt64OrZero(args[0])
	refillRate := toInt64OrZero(args[1])
	intervalMs := toInt64OrZero(args[2])
	nowMs := toInt64OrZero(args[3])
	ttlMs := toInt64OrZero(args[4])
	key := keys[0]

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.data[key]
	if ok && entry.expiresAtMs > 0 && nowMs >= entry.expiresAtMs {
		delete(c.data, key)
		ok = false
	}

	if !ok {
		entry = fakeRedisEntry{
			tokens:       capacity,
			lastRefillMs: nowMs,
		}
	}

	elapsed := nowMs - entry.lastRefillMs
	if elapsed > 0 {
		newTokens := (elapsed * refillRate) / intervalMs
		if newTokens > 0 {
			entry.tokens += newTokens
			if entry.tokens > capacity {
				entry.tokens = capacity
			}
			consumedMs := (newTokens * intervalMs) / refillRate
			if consumedMs < 0 {
				consumedMs = 0
			}
			entry.lastRefillMs += consumedMs
		}
	}

	allowed := int64(0)
	if entry.tokens > 0 {
		entry.tokens--
		allowed = 1
	}

	entry.lastSeenMs = nowMs
	if ttlMs > 0 {
		entry.expiresAtMs = nowMs + ttlMs
	}
	c.data[key] = entry

	return []any{allowed, entry.tokens}, nil
}

func toInt64OrZero(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case uint64:
		return int64(t)
	case int32:
		return int64(t)
	default:
		return 0
	}
}

func newRedisBackedManagerForTest(tb testing.TB, client RedisEvalClient, capacity, refill int64, interval time.Duration) *Manager {
	tb.Helper()

	store, err := NewRedisStore(client, RedisStoreOptions{
		KeyPrefix: "test:",
		KeyTTL:    time.Minute,
	})
	if err != nil {
		tb.Fatalf("failed to create redis store: %v", err)
	}

	m, err := NewManagerWithStore(store, capacity, refill, interval, time.Minute, 10*time.Millisecond)
	if err != nil {
		tb.Fatalf("failed to create manager: %v", err)
	}
	return m
}

func TestRedisStoreBurstTraffic(t *testing.T) {
	client := newFakeRedisEvalClient()
	m := newRedisBackedManagerForTest(t, client, 5, 1, time.Hour)
	defer m.Close()

	for i := 0; i < 5; i++ {
		if !m.Allow("user-1") {
			t.Fatalf("expected request %d to pass", i+1)
		}
	}
	if m.Allow("user-1") {
		t.Fatal("expected burst beyond capacity to be blocked")
	}
}

func TestRedisStoreRapidRefill(t *testing.T) {
	client := newFakeRedisEvalClient()
	m := newRedisBackedManagerForTest(t, client, 2, 2, 100*time.Millisecond)
	defer m.Close()

	if !m.Allow("user-rapid") {
		t.Fatal("expected first token to be available")
	}
	if !m.Allow("user-rapid") {
		t.Fatal("expected second token to be available")
	}
	if m.Allow("user-rapid") {
		t.Fatal("expected bucket to be empty after consuming capacity")
	}

	time.Sleep(120 * time.Millisecond)

	if !m.Allow("user-rapid") {
		t.Fatal("expected rapid refill to allow request")
	}
}

func TestRedisStoreConcurrentRequests(t *testing.T) {
	client := newFakeRedisEvalClient()
	m := newRedisBackedManagerForTest(t, client, 100, 1, time.Hour)
	defer m.Close()

	var wg sync.WaitGroup
	var allowed int64

	for range 500 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if m.Allow("shared-key") {
				atomic.AddInt64(&allowed, 1)
			}
		}()
	}
	wg.Wait()

	if allowed != 100 {
		t.Fatalf("expected 100 allowed requests, got %d", allowed)
	}
}

func TestRedisStoreConcurrentMultipleKeys(t *testing.T) {
	client := newFakeRedisEvalClient()
	m := newRedisBackedManagerForTest(t, client, 10, 1, time.Hour)
	defer m.Close()

	const keys = 8
	const requestsPerKey = 100

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
		if allowedByKey[i] != 10 {
			t.Fatalf("expected key user-%d to allow 10 requests, got %d", i, allowedByKey[i])
		}
	}
}

func TestRedisStoreCrossInstanceSimulation(t *testing.T) {
	client := newFakeRedisEvalClient()

	m1 := newRedisBackedManagerForTest(t, client, 3, 1, time.Hour)
	defer m1.Close()
	m2 := newRedisBackedManagerForTest(t, client, 3, 1, time.Hour)
	defer m2.Close()

	if !m1.Allow("shared-user") {
		t.Fatal("expected first request to pass")
	}
	if !m2.Allow("shared-user") {
		t.Fatal("expected second request from another instance to pass")
	}
	if !m1.Allow("shared-user") {
		t.Fatal("expected third request to pass")
	}
	if m2.Allow("shared-user") {
		t.Fatal("expected fourth request across instances to be blocked")
	}
}
