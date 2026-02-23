package core

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTokenBucket(t *testing.T) {
	tb, err := NewTokenBucket(2, 1) // bucket with capacity 2 tokens and refill rate 1 token per sec
	if err != nil {
		t.Fatalf("unexpected error creating bucket: %v", err)
	}

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
	tb, err := NewTokenBucket(100, 1, time.Hour) // effectively no refill during this test window
	if err != nil {
		t.Fatalf("unexpected error creating bucket: %v", err)
	}

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
	tb, err := NewTokenBucket(2, 1) // bucket with capacity 2 tokens and refill rate 1 token per sec
	if err != nil {
		t.Fatalf("unexpected error creating bucket: %v", err)
	}

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

func TestRefillGreaterThanCapacity(t *testing.T) {
	tb, err := NewTokenBucket(2, 2, time.Second)
	if err != nil {
		t.Fatalf("unexpected error creating bucket: %v", err)
	}

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

func TestInvalidConfig(t *testing.T) {
	_, err := NewTokenBucket(0, 10, time.Second)
	if err == nil {
		t.Fatal("expected error for zero capacity")
	}

	_, err = NewTokenBucket(10, 0, time.Second)
	if err == nil {
		t.Fatal("expected error for zero tokens")
	}

	_, err = NewTokenBucket(10, 10, 0)
	if err == nil {
		t.Fatal("expected error for zero interval")
	}

	_, err = NewTokenBucket(5, 10, time.Second)
	if err == nil {
		t.Fatal("expected error when tokens > capacity")
	}
}
