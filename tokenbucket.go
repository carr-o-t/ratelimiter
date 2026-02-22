package ratelimiter

import (
	"log"
	"sync"
	"time"
)

type TokenBucket struct {
	capacity   int64 // max tokens
	tokens     int64 // current tokens
	refillRate int64 // tokens per second
	lastRefill int64 // last refill time (nanosec)
	mu         sync.Mutex
}

func NewTokenBucket(capacity int64, refillRate int64) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity, // initial tokens, full capacity
		refillRate: refillRate,
		lastRefill: time.Now().UnixNano(),
	}
}

// private func
func (tb *TokenBucket) refill() {
	now := time.Now().UnixNano()
	elapsed := now - tb.lastRefill
	newTokens := (elapsed * tb.refillRate) / 1e9

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

	beforeRefill := tb.tokens
	tb.refill()
	afterRefill := tb.tokens

	allowed := false
	if tb.tokens > 0 {
		log.Printf("tokens =%d greater than=%d token_bucket allow=true", tb.tokens, int64(0))
		tb.tokens--
		allowed = true
	}

	log.Printf(
		"token_bucket allow=%t tokens_before_refill=%d tokens_after_refill=%d tokens_after_allow=%d capacity=%d refill_rate=%d",
		allowed,
		beforeRefill,
		afterRefill,
		tb.tokens,
		tb.capacity,
		tb.refillRate,
	)

	return allowed
}
