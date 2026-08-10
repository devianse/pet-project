package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIPRateLimiter_AllowsUpToBurst(t *testing.T) {
	l := newIPRateLimiter(rateEvery(time.Minute), 3)

	for i := range 3 {
		if !l.allow("1.2.3.4") {
			t.Fatalf("expected request %d within burst to be allowed", i+1)
		}
	}
}

func TestIPRateLimiter_BlocksAfterBurstExhausted(t *testing.T) {
	l := newIPRateLimiter(rateEvery(time.Minute), 3)

	for range 3 {
		l.allow("1.2.3.4")
	}

	if l.allow("1.2.3.4") {
		t.Fatal("expected request beyond burst to be blocked")
	}
}

func TestIPRateLimiter_TracksIndependentIPs(t *testing.T) {
	l := newIPRateLimiter(rateEvery(time.Minute), 1)

	if !l.allow("1.2.3.4") {
		t.Fatal("expected first IP's first request to be allowed")
	}
	if !l.allow("5.6.7.8") {
		t.Fatal("expected a different IP to have its own independent bucket")
	}
}

func TestIPRateLimiter_PurgeStaleRemovesOldEntries(t *testing.T) {
	l := newIPRateLimiter(rateEvery(time.Minute), 1)
	l.allow("1.2.3.4")
	l.visitors["1.2.3.4"].lastSeen = time.Now().Add(-time.Hour)

	l.purgeStale(10 * time.Minute)

	if _, ok := l.visitors["1.2.3.4"]; ok {
		t.Fatal("expected entry older than maxAge to be purged")
	}
}

func TestIPRateLimiter_PurgeStaleKeepsRecentEntries(t *testing.T) {
	l := newIPRateLimiter(rateEvery(time.Minute), 1)
	l.allow("1.2.3.4")

	l.purgeStale(10 * time.Minute)

	if _, ok := l.visitors["1.2.3.4"]; !ok {
		t.Fatal("expected a recently-seen entry to survive the purge")
	}
}

func TestClientIP_UsesXForwardedForWhenPresent(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.7")

	if got := clientIP(req); got != "203.0.113.7" {
		t.Fatalf("expected 203.0.113.7, got %q", got)
	}
}

func TestClientIP_FallsBackToRemoteAddrWithoutForwardedHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "10.0.0.1:12345"

	if got := clientIP(req); got != "10.0.0.1" {
		t.Fatalf("expected 10.0.0.1, got %q", got)
	}
}

func TestRateLimitMiddleware_AllowsRequestsWithinLimit(t *testing.T) {
	l := newIPRateLimiter(rateEvery(time.Minute), 1)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.RemoteAddr = "1.2.3.4:1"
	rec := httptest.NewRecorder()

	rateLimitMiddleware(l, next).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !called {
		t.Fatal("expected next handler to be called within the limit")
	}
}

func TestRateLimitMiddleware_Returns429WhenLimitExceeded(t *testing.T) {
	l := newIPRateLimiter(rateEvery(time.Minute), 1)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rateLimitMiddleware(l, next)

	first := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	first.RemoteAddr = "1.2.3.4:1"
	handler.ServeHTTP(httptest.NewRecorder(), first)

	second := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	second.RemoteAddr = "1.2.3.4:2"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, second)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
}
