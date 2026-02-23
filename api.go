package ratelimiter

import (
	"time"

	"github.com/carr-o-t/ratelimiter/internal/core"
)

type Limiter = core.Limiter
type Store = core.Store
type TokenBucket = core.TokenBucket
type Manager = core.Manager
type MemoryStore = core.MemoryStore
type RedisStore = core.RedisStore
type RedisStoreOptions = core.RedisStoreOptions
type RedisEvalClient = core.RedisEvalClient

func NewTokenBucket(capacity int64, refillRate int64, per ...time.Duration) (*TokenBucket, error) {
	return core.NewTokenBucket(capacity, refillRate, per...)
}

func NewMemoryStore() *MemoryStore {
	return core.NewMemoryStore()
}

func NewRedisStore(client RedisEvalClient, opts RedisStoreOptions) (*RedisStore, error) {
	return core.NewRedisStore(client, opts)
}

func NewManager(
	capacity int64,
	refillRate int64,
	per time.Duration,
	bucketTTL time.Duration,
	cleanupInterval time.Duration,
) (*Manager, error) {
	return core.NewManager(capacity, refillRate, per, bucketTTL, cleanupInterval)
}

func NewManagerWithStore(
	store Store,
	capacity int64,
	refillRate int64,
	per time.Duration,
	bucketTTL time.Duration,
	cleanupInterval time.Duration,
) (*Manager, error) {
	return core.NewManagerWithStore(store, capacity, refillRate, per, bucketTTL, cleanupInterval)
}
