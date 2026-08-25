package httpserver

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type rateEntry struct {
	windowStart time.Time
	count       int
}

type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]rateEntry
	now     func() time.Time
}

func newRateLimiter() *rateLimiter {
	return &rateLimiter{entries: map[string]rateEntry{}, now: time.Now}
}

func (l *rateLimiter) allow(key string, limit int, window time.Duration) bool {
	if limit <= 0 || window <= 0 {
		return true
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	e := l.entries[key]
	if e.windowStart.IsZero() || now.Sub(e.windowStart) >= window {
		e = rateEntry{windowStart: now, count: 1}
		l.entries[key] = e
		return true
	}
	if e.count >= limit {
		return false
	}
	e.count++
	l.entries[key] = e
	if len(l.entries) > 4096 {
		for k, v := range l.entries {
			if now.Sub(v.windowStart) > 2*window {
				delete(l.entries, k)
			}
		}
	}
	return true
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func (s *Server) rateLimit(scope string, limit int, window time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.limiter == nil {
			s.limiter = newRateLimiter()
		}
		key := scope + "|" + clientIP(r)
		if !s.limiter.allow(key, limit, window) {
			w.Header().Set("Retry-After", durationSeconds(window))
			writeProblem(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func durationSeconds(d time.Duration) string {
	n := int(d.Seconds())
	if n < 1 {
		n = 1
	}
	return fmtInt(n)
}

func fmtInt(v int) string {
	if v == 0 {
		return "0"
	}
	buf := [24]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
