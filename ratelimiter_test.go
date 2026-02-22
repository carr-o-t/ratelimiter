package ratelimiter

import "testing"

func TestTokenBucket(t *testing.T) {
	tb := NewTokenBucket(2, 1)

	if !tb.Allow() {
		t.Fatal("Expected first request to pass")
	}

	if !tb.Allow() {
		t.Fatal("Expected second request to pass")
	}

	if tb.Allow() {
		t.Fatal("Expected third request to be blocked")
	}
}