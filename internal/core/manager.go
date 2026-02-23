package core

import (
	"errors"
	"sync"
	"time"
)

type Manager struct {
	store Store

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
	return NewManagerWithStore(
		NewMemoryStore(),
		capacity,
		refillRate,
		per,
		bucketTTL,
		cleanupInterval,
	)
}

func NewManagerWithStore(
	store Store,
	capacity int64,
	refillRate int64,
	per time.Duration,
	bucketTTL time.Duration,
	cleanupInterval time.Duration,
) (*Manager, error) {
	if store == nil {
		return nil, errors.New("store cannot be nil")
	}
	if err := validateTokenBucketConfig(capacity, refillRate, per); err != nil {
		return nil, err
	}
	if cleanupInterval <= 0 {
		return nil, errors.New("cleanup interval must be greater than 0")
	}
	if bucketTTL <= 0 {
		return nil, errors.New("bucket TTL must be greater than 0")
	}

	m := &Manager{
		store:           store,
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
	decision, err := m.AllowDecision(key)
	if err != nil {
		return false
	}
	return decision.Allowed
}

func (m *Manager) AllowDecision(key string) (Decision, error) {
	return m.store.Allow(key, BucketConfig{
		Capacity:   m.capacity,
		RefillRate: m.refillRate,
		Interval:   m.interval,
	})
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
		_ = m.store.Close()
	})
}

func (m *Manager) Close() {
	m.Stop()
}

func (m *Manager) Cleanup() {
	cutoff := time.Now().Add(-m.bucketTTL)
	_ = m.store.DeleteInactiveBuckets(cutoff)
}
