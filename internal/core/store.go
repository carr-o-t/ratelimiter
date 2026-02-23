package core

import "time"

type Store interface {
	GetOrCreate(key string, create func() (*TokenBucket, error)) (*TokenBucket, error)
	DeleteInactiveBuckets(cutoff time.Time) error
	Close() error
}
