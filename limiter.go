// interface

package ratelimiter

type Limiter interface {
	Allow() bool
}