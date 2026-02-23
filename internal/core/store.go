package core

import "time"

type BucketConfig struct {
	Capacity   int64
	RefillRate int64
	Interval   time.Duration
}

type Decision struct {
	Allowed    bool
	Remaining  int64
	Limit      int64
	RetryAfter time.Duration
}

type Store interface {
	Allow(key string, cfg BucketConfig) (Decision, error)
	DeleteInactiveBuckets(cutoff time.Time) error
	Close() error
}
