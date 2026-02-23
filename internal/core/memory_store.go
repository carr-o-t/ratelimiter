package core

import (
	"sync"
	"time"
)

type MemoryStore struct {
	mu      sync.Mutex
	buckets map[string]*TokenBucket
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		buckets: make(map[string]*TokenBucket),
	}
}

func (s *MemoryStore) Allow(key string, cfg BucketConfig) (Decision, error) {
	s.mu.Lock()
	tb, ok := s.buckets[key]
	if !ok {
		var err error
		tb, err = NewTokenBucket(cfg.Capacity, cfg.RefillRate, cfg.Interval)
		if err != nil {
			s.mu.Unlock()
			return Decision{}, err
		}
		s.buckets[key] = tb
	}
	s.mu.Unlock()

	return Decision{Allowed: tb.Allow()}, nil
}

func (s *MemoryStore) DeleteInactiveBuckets(cutoff time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, bucket := range s.buckets {
		bucket.mu.Lock()
		lastSeen := bucket.lastSeen
		bucket.mu.Unlock()
		if lastSeen.Before(cutoff) {
			delete(s.buckets, key)
		}
	}
	return nil
}

func (s *MemoryStore) Close() error {
	return nil
}
