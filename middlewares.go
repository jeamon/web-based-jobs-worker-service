package main

// @middlewares.go contains web requests middleware for logging each URL or API calls.

import (
	"net/http"
)

// logRequestMiddleware is a middleware that logs incoming request details.
func logRequestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		weblog.Printf("received request - ip: %s - method: %s - url: %s - browser: %s\n", r.RemoteAddr, r.Method, r.URL.Path, r.UserAgent())
		next.ServeHTTP(w, r)
	})
}
