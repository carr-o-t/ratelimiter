package core

import (
	"errors"
	"sync"
	"time"
)

type TokenBucket struct {
	capacity   int64
	tokens     int64
	refillRate int64

	interval   time.Duration
	lastRefill time.Time
	lastSeen   time.Time

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

	now := time.Now()
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		interval:   interval,
		refillRate: refillRate,
		lastRefill: now,
		lastSeen:   now,
	}, nil
}

func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)

	newTokens := int64(float64(elapsed) / float64(tb.interval) * float64(tb.refillRate))
	if newTokens > 0 {
		tb.tokens += newTokens
		if tb.tokens > tb.capacity {
			tb.tokens = tb.capacity
		}
		timeUsed := time.Duration(newTokens * int64(tb.interval) / tb.refillRate)
		tb.lastRefill = tb.lastRefill.Add(timeUsed)
	}
}

func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.lastSeen = time.Now()
	tb.refill()

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	return false
}
