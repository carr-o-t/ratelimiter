package core

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strconv"
	"time"
)

//go:embed token_bucken_redis_script.lua
var tokenBucketRedisLua string

type RedisStoreOptions struct {
	KeyPrefix string
	KeyTTL    time.Duration
}

type RedisStore struct {
	client RedisEvalClient
	prefix string
	ttl    time.Duration
}

type RedisEvalClient interface {
	Eval(ctx context.Context, script string, keys []string, args ...any) (any, error)
}

func NewRedisStore(client RedisEvalClient, opts RedisStoreOptions) (*RedisStore, error) {
	if client == nil {
		return nil, errors.New("redis client cannot be nil")
	}

	prefix := opts.KeyPrefix
	if prefix == "" {
		prefix = "ratelimiter:"
	}
	ttl := opts.KeyTTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}

	return &RedisStore{
		client: client,
		prefix: prefix,
		ttl:    ttl,
	}, nil
}

func (s *RedisStore) Allow(key string, cfg BucketConfig) (Decision, error) {
	if key == "" {
		return Decision{}, errors.New("key cannot be empty")
	}
	if cfg.Interval <= 0 {
		return Decision{}, errors.New("interval must be greater than 0")
	}
	if cfg.Capacity <= 0 || cfg.RefillRate <= 0 {
		return Decision{}, errors.New("capacity and refill rate must be greater than 0")
	}

	nowMs := time.Now().UnixMilli()
	intervalMs := cfg.Interval.Milliseconds()
	ttlMs := s.ttl.Milliseconds()
	if intervalMs <= 0 {
		return Decision{}, errors.New("interval must be at least 1ms")
	}
	if ttlMs <= 0 {
		ttlMs = intervalMs
	}

	result, err := s.client.Eval(context.Background(), tokenBucketRedisLua, []string{s.prefixedKey(key)},
		cfg.Capacity,
		cfg.RefillRate,
		intervalMs,
		nowMs,
		ttlMs,
	)
	if err != nil {
		return Decision{}, err
	}

	values, ok := result.([]any)
	if !ok || len(values) != 2 {
		return Decision{}, fmt.Errorf("unexpected redis lua result: %T", result)
	}

	allowed, err := toInt64(values[0])
	if err != nil {
		return Decision{}, err
	}
	remaining, err := toInt64(values[1])
	if err != nil {
		return Decision{}, err
	}

	return Decision{
		Allowed:   allowed == 1,
		Remaining: remaining,
		Limit:     cfg.Capacity,
		// For now this is an approximation for blocked responses.
		// It can be made exact later by returning reset metadata from Lua.
		RetryAfter: retryAfterForConfig(cfg, allowed == 1),
	}, nil
}

func (s *RedisStore) DeleteInactiveBuckets(_ time.Time) error {
	// Redis keys expires via TTL set in Allow()
	return nil
}

func (s *RedisStore) Close() error {
	return nil
}

func (s *RedisStore) prefixedKey(key string) string {
	return s.prefix + key
}

func toInt64(v interface{}) (int64, error) {
	switch t := v.(type) {
	case int64:
		return t, nil
	case int:
		return int64(t), nil
	case uint64:
		return int64(t), nil
	case string:
		parsed, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unexpected numeric type: %T", v)
	}
}

func retryAfterForConfig(cfg BucketConfig, allowed bool) time.Duration {
	if allowed {
		return 0
	}
	if cfg.RefillRate <= 0 {
		return 0
	}
	intervalNs := int64(cfg.Interval)
	waitNs := (intervalNs + cfg.RefillRate - 1) / cfg.RefillRate
	return time.Duration(waitNs)
}
