package ratelimiter

import (
	"errors"
	"sync"
	"time"
)

type TokenBucket struct {
	capacity   int64 // max tokens
	tokens     int64 // current tokens
	refillRate int64 // tokens per second

	interval   time.Duration
	lastRefill time.Time // last refill time
	lastSeen   time.Time // last seen time

	mu sync.Mutex
}

func validateTokenBucketConfig(capacity int64, tokens int64, per time.Duration) error {
	if capacity <= 0 {
		return errors.New("capacity must be greater than 0")
	}

	if tokens <= 0 {
		return errors.New("tokens per interval must be greater than 0")
	}

	if per <= 0 {
		return errors.New("interval must be greater than 0")
	}

	if tokens > capacity {
		return errors.New("tokens per interval cannot exceed capacity")
	}

	return nil
}

func NewTokenBucket(capacity int64, refillRate int64, per ...time.Duration) (*TokenBucket, error) {
	interval := time.Second
	if len(per) > 0 {
		interval = per[0]
	}
	if err := validateTokenBucketConfig(capacity, refillRate, interval); err != nil {
		return nil, err
	}

	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity, // initial tokens, full capacity
		interval:   interval,
		refillRate: refillRate,
		lastRefill: time.Now(),
		lastSeen:   time.Now(),
	}, nil
}

// private function
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)

	// calculate to prevent early overflow
	// and handle fractional intervals correctly
	newTokens := int64(float64(elapsed) / float64(tb.interval) * float64(tb.refillRate))

	if newTokens > 0 {
		tb.tokens += newTokens
		if tb.tokens > tb.capacity {
			tb.tokens = tb.capacity
		}

		// move lastRefill forward only by the time used for the generated tokens
		timeUsed := time.Duration(newTokens * int64(tb.interval) / tb.refillRate)
		tb.lastRefill = tb.lastRefill.Add(timeUsed)
	}
}

// public
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.lastSeen = time.Now()

	tb.refill()

	allowed := false
	if tb.tokens > 0 {
		tb.tokens--
		allowed = true
	}

	return allowed
}
