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

func (s *MemoryStore) GetOrCreate(
	key string,
	create func() (*TokenBucket, error),
) (*TokenBucket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tb, ok := s.buckets[key]; ok {
		return tb, nil
	}

	tb, err := create()
	if err != nil {
		return nil, err
	}
	s.buckets[key] = tb
	return tb, nil
}

func (s *MemoryStore) DeleteInactiveBuckets(cutoff time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, bucket := range s.buckets {
		if bucket.lastSeen.Before(cutoff) {
			delete(s.buckets, key)
		}
	}
	return nil
}

func (s *MemoryStore) Close() error {
	return nil
}
