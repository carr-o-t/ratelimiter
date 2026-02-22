package ratelimiter

import (
	"errors"
	"sync"
	"time"
)

type Manager struct {
	buckets map[string]*TokenBucket
	mu      sync.Mutex

	capacity   int64
	refillRate int64
	interval   time.Duration

	bucketTTL       time.Duration
	cleanupInterval time.Duration

	stopCh   chan struct{}
	wg       sync.WaitGroup
	stopOnce sync.Once
}

func NewManager(
	capacity int64,
	refillRate int64,
	per time.Duration,
	bucketTTL time.Duration,
	cleanupInterval time.Duration,
) (*Manager, error) {
	var err error
	if err = validateTokenBucketConfig(capacity, refillRate, per); err != nil {
		return nil, err
	}

	if cleanupInterval <= 0 {
		return nil, errors.New("cleanup interval must be greater than 0")
	}

	if bucketTTL <= 0 {
		return nil, errors.New("bucket TTL must be greater than 0")
	}

	m := &Manager{
		buckets:         make(map[string]*TokenBucket),
		capacity:        capacity,
		refillRate:      refillRate,
		interval:        per,
		bucketTTL:       bucketTTL,
		cleanupInterval: cleanupInterval,
		stopCh:          make(chan struct{}),
	}

	m.wg.Add(1)
	go m.cleanupLoop()

	return m, nil
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
	tb.lastSeen = time.Now()

	m.mu.Unlock()

	return tb.Allow()
}

func (m *Manager) cleanupLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.Cleanup()
		case <-m.stopCh:
			return
		}
	}
}

func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
		m.wg.Wait()
	})
}

func (m *Manager) Close() {
	m.Stop()
}

func (m *Manager) Cleanup() {
	m.mu.Lock()

	for key, bucket := range m.buckets {
		if time.Since(bucket.lastSeen) > m.bucketTTL {
			delete(m.buckets, key)
		}
	}
	m.mu.Unlock()
}
