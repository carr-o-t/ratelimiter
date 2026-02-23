package core

import (
	"net/http"
	"strconv"
	"time"
)

func (m *Manager) Middleware(
	keyFunc func(*http.Request) string,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			decision, err := m.AllowDecision(key)
			if err != nil {
				http.Error(w, "rate limiter error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(decision.Limit, 10))
			w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(decision.Remaining, 10))
			if !decision.Allowed && decision.RetryAfter > 0 {
				w.Header().Set("Retry-After", strconv.FormatInt(durationCeilSeconds(decision.RetryAfter), 10))
			}

			if !decision.Allowed {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func durationCeilSeconds(d time.Duration) int64 {
	if d <= 0 {
		return 0
	}
	return int64((d + time.Second - 1) / time.Second)
}
