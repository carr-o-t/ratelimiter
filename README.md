# ratelimiter

Lightweight, concurrency-safe rate limiter library in Go, with token bucket primitives and per-key manager support.

## Features

- Token bucket limiter with configurable refill interval
- Per-key rate limiting through `Manager` (for user/IP/API key style limits)
- Thread-safe `Allow()` calls
- Decision metadata support (`Allowed`, `Remaining`, `Limit`, `RetryAfter`)
- Middleware emits rate-limit headers (`X-RateLimit-Limit`, `X-RateLimit-Remaining`, `Retry-After`)
- Input validation for safer configuration
- Automatic inactive-bucket cleanup with graceful shutdown (`Stop` / `Close`)
- Pluggable backend design (in-memory and Redis/Lua)

## Install

```bash
go get github.com/carr-o-t/ratelimiter
```

## Quick Start

### Token bucket (single limiter)

```go
package main

import (
	"fmt"
	"time"

	"github.com/carr-o-t/ratelimiter"
)

func main() {
	// capacity=10, refillRate=5 per second (default interval is 1s)
	tb, err := ratelimiter.NewTokenBucket(10, 5)
	if err != nil {
		panic(err)
	}

	if tb.Allow() {
		fmt.Println("allowed")
	} else {
		fmt.Println("blocked")
	}

	// custom interval: 120 tokens per minute
	_, _ = ratelimiter.NewTokenBucket(200, 120, time.Minute)
}
```

### Manager (per-key limiter)

```go
package main

import (
	"net/http"
	"time"

	"github.com/carr-o-t/ratelimiter"
)

func main() {
	// key bucket config:
	// capacity=20, refillRate=10 per second
	// manager config:
	// bucketTTL=5m, cleanupInterval=30s
	m, err := ratelimiter.NewManager(20, 10, time.Second, 5*time.Minute, 30*time.Second)
	if err != nil {
		panic(err)
	}
	defer m.Close() // or m.Stop()

	http.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		key := r.RemoteAddr
		if !m.Allow(key) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	_ = http.ListenAndServe(":8080", nil)
}
```

### Redis backend (Lua, atomic)

`RedisStore` runs token-bucket logic inside Redis using a Lua script, so updates are atomic across distributed app instances.

`NewRedisStore` accepts a small `Eval`-based client interface. You can adapt your Redis client to this shape:

```go
type redisEvalAdapter struct {
	client *redis.Client // github.com/redis/go-redis/v9
}

func (a redisEvalAdapter) Eval(
	ctx context.Context,
	script string,
	keys []string,
	args ...interface{},
) (interface{}, error) {
	return a.client.Eval(ctx, script, keys, args...).Result()
}

adapter := redisEvalAdapter{client: redisClient}
store, err := ratelimiter.NewRedisStore(adapter, ratelimiter.RedisStoreOptions{
	KeyPrefix: "ratelimiter:",
	KeyTTL:    10 * time.Minute,
})
if err != nil {
	panic(err)
}

m, err := ratelimiter.NewManagerWithStore(
	store,
	20, 10, time.Second, 5*time.Minute, 30*time.Second,
)
if err != nil {
	panic(err)
}
defer m.Close()
```

### Middleware lifecycle (startup vs request time)

`Middleware` has this type:

```go
func (m *Manager) Middleware(
    keyFunc func(*http.Request) string,
) func(http.Handler) http.Handler
```

How to read it:

- At startup, you call `m.Middleware(keyFunc)` once to build the middleware wrapper.
- At startup, you wrap your route handler once: `wrapped := mw(myHandler)`.
- On every request, the wrapped handler runs:
  - key is extracted with `keyFunc`
  - `m.AllowDecision(key)` is evaluated internally
  - response headers are set:
    - `X-RateLimit-Limit`
    - `X-RateLimit-Remaining`
    - `Retry-After` (when blocked)
  - blocked requests return `429`
  - allowed requests call `next.ServeHTTP(...)`

Example:

```go
keyFunc := func(r *http.Request) string { return r.RemoteAddr }
mw := m.Middleware(keyFunc)      // startup-time setup
http.Handle("/api", mw(myHandler)) // wrap once; runs per request
```

## API

### `NewTokenBucket(capacity, refillRate int64, per ...time.Duration) (*TokenBucket, error)`

- `capacity`: max tokens in bucket
- `refillRate`: tokens added per `per` interval
- `per` (optional): refill interval. Defaults to `time.Second`.

### `(*TokenBucket) Allow() bool`

- Returns `true` if a token is available and consumed
- Returns `false` if request should be rate-limited

### `NewManager(capacity, refillRate int64, per, bucketTTL, cleanupInterval time.Duration) (*Manager, error)`

- Creates per-key token buckets lazily
- Removes inactive buckets older than `bucketTTL`
- Runs cleanup every `cleanupInterval`

### `(*Manager) Allow(key string) bool`

- Applies rate limit for a specific key

### `(*Manager) AllowDecision(key string) (Decision, error)`

- Applies rate limit and returns metadata for headers/clients
- `Decision` fields:
  - `Allowed bool`
  - `Remaining int64`
  - `Limit int64`
  - `RetryAfter time.Duration`

### `(*Manager) Stop()` / `(*Manager) Close()`

- Gracefully stops background cleanup goroutine

### `NewRedisStore(client RedisEvalClient, opts RedisStoreOptions) (*RedisStore, error)`

- Creates Redis-backed store with atomic Lua execution.
- `RedisStoreOptions.KeyPrefix` defaults to `ratelimiter:`
- `RedisStoreOptions.KeyTTL` defaults to `10m`

## Validation Rules

`NewTokenBucket` returns an error when:

- `capacity <= 0`
- `refillRate <= 0`
- `per <= 0`
- `refillRate > capacity`

`NewManager` additionally validates:

- `bucketTTL > 0`
- `cleanupInterval > 0`

## Concurrency

Both `TokenBucket` and `Manager` are safe for concurrent use.

## Testing

```bash
go test ./...
```

## Benchmarks And Examples

TO BE ADDED
