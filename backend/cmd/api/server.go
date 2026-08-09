package main

import (
	"net/http"
	"time"
)

// maxRequestBodyBytes caps incoming request bodies before they reach
// JSON decoding — a large-body DoS shouldn't get as far as parsing.
const maxRequestBodyBytes = 1 << 20 // 1MB

// maxBytesMiddleware rejects requests whose declared Content-Length
// exceeds maxRequestBodyBytes outright, and wraps the body reader so an
// unknown-length (e.g. chunked) body that exceeds the cap fails on read
// instead of being decoded in full first.
func maxBytesMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > maxRequestBodyBytes {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		next.ServeHTTP(w, r)
	})
}

// newServer builds an http.Server with explicit timeouts. The zero-value
// server used by a bare http.ListenAndServe call has none, which leaves
// it exposed to slow-client (slowloris-style) connections that never
// finish sending headers or body.
func newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
