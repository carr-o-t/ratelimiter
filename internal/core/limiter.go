package core

type Limiter interface {
	Allow() bool
}
