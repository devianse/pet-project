package main

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// rateEvery is a thin re-export so callers of this file don't need their
// own import of golang.org/x/time/rate just to build a rate.Limit.
func rateEvery(interval time.Duration) rate.Limit {
	return rate.Every(interval)
}

// visitor pairs a per-IP token bucket with the last time it was touched,
// so stale buckets can be pruned without bounding the map size up front.
type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// ipRateLimiter hands out one token bucket per client IP. It exists so a
// single misbehaving (or attacking) IP can be throttled without limiting
// every other caller sharing the endpoint.
type ipRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	r        rate.Limit
	burst    int
}

func newIPRateLimiter(r rate.Limit, burst int) *ipRateLimiter {
	return &ipRateLimiter{
		visitors: make(map[string]*visitor),
		r:        r,
		burst:    burst,
	}
}

// allow reports whether a request from ip may proceed, consuming a token
// from that IP's bucket if so. A first-seen IP gets a fresh bucket.
func (l *ipRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	v, ok := l.visitors[ip]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(l.r, l.burst)}
		l.visitors[ip] = v
	}
	v.lastSeen = time.Now()
	return v.limiter.Allow()
}

// clientIP extracts the caller's IP for rate-limiting purposes.
//
// X-Forwarded-For is trusted here because the backend has no host port
// mapping in infra/docker-compose.yml — Caddy's reverse proxy is the only
// path to this process, and Caddy always sets/overwrites this header with
// the real client IP. That trust boundary breaks if the backend is ever
// exposed directly (e.g. a host port added for debugging), which would let
// a caller spoof the header to dodge its own limit.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if ip := strings.TrimSpace(strings.Split(fwd, ",")[0]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// purgeStale drops any visitor not seen within maxAge, so a long-running
// process doesn't accumulate one map entry per distinct IP forever.
func (l *ipRateLimiter) purgeStale(maxAge time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for ip, v := range l.visitors {
		if v.lastSeen.Before(cutoff) {
			delete(l.visitors, ip)
		}
	}
}

// startCleanup periodically purges visitors idle longer than maxAge, until
// ctx is cancelled. Without this, the visitors map would grow by one entry
// per distinct IP ever seen for the lifetime of the process.
func (l *ipRateLimiter) startCleanup(ctx context.Context, interval, maxAge time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.purgeStale(maxAge)
		}
	}
}

// rateLimitMiddleware rejects a request with 429 once its caller's IP has
// exhausted its token bucket in l, rather than forwarding it to next.
func rateLimitMiddleware(l *ipRateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientIP(r)) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
