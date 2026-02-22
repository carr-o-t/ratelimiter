package ratelimiter

import (
	"sync"
	"time"
)

type Manager struct {
	buckets map[string]*TokenBucket
	mu      sync.Mutex

	capacity   int64
	refillRate int64
	interval   time.Duration
}

func NewManager(capacity int64, refillRate int64, per time.Duration) (*Manager, error) {
	if err := validateTokenBucketConfig(capacity, refillRate, per); err != nil {
		return nil, err
	}

	return &Manager{
		buckets:   make(map[string]*TokenBucket),
		capacity:  capacity,
		refillRate: refillRate,
		interval:  per,
	}, nil
}

func (m *Manager) Allow(key string) bool {
	m.mu.Lock()
	tb, exists := m.buckets[key]
	if !exists {
		var err error
		tb, err = NewTokenBucket(m.capacity, m.refillRate, m.interval)
		if err != nil {
			m.mu.Unlock()
			return false
		}
		m.buckets[key] = tb
	}
	m.mu.Unlock()

	return tb.Allow()
}