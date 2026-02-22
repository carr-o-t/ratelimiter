package ratelimiter

import (
	"sync"
	"time"
)

type TokenBucket struct {
	capacity   int64     // max tokens
	tokens     int64     // current tokens
	refillRate int64     // tokens per second
	lastRefill time.Time // last refill time (nanosec)
	mu         sync.Mutex
}

func NewTokenBucket(capacity int64, refillRate int64) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity, // initial tokens, full capacity
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// private func
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	newTokens := int64(elapsed * float64(tb.refillRate))

	if newTokens > 0 {
		tb.tokens += newTokens
		if tb.tokens > tb.capacity {
			tb.tokens = tb.capacity
		}
		tb.lastRefill = now
	}

}

// public
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}
