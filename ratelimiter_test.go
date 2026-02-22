package ratelimiter

import (
	"testing"
	"sync"
	"sync/atomic"
	"time"
)

func TestTokenBucket(t *testing.T) {
	tb := NewTokenBucket(2, 1)

	if !tb.Allow() {
		t.Fatal("Expected first request to pass")
	}

	if !tb.Allow() {
		t.Fatal("Expected second request to pass")
	}

	if tb.Allow() {
		t.Fatal("Expected third request to be blocked")
	}
}

func TestTokenBucketConcurrent(t *testing.T) {
	tb := NewTokenBucket(100, 0)

	var wg sync.WaitGroup
	allowed := int64(0)

	for range 200 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if tb.Allow() {
				atomic.AddInt64(&allowed, 1)
			}
		}()
	}

	wg.Wait()

	if allowed != 100 {
		t.Fatalf("Expected 100 allowed, got %d", allowed)
	}
}

func TestRefill(t *testing.T) {
	tb := NewTokenBucket(2, 1) // bucket with capacity 2 tokens and refill rate 1 token per sec

	tb.Allow()
	tb.Allow()

	if tb.Allow() {
		t.Fatal("Expected bucket to be empty")
	}

	time.Sleep(1 * time.Second)

	if !tb.Allow() {
		t.Fatal("Expected refill after 1 second")
	}
}

