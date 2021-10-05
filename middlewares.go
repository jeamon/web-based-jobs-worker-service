package main

// @middlewares.go contains middlewares for decorating each URL or API calls.

import (
	"context"
	"net/http"
	"strings"
)

// requestMiddleware adds for each request an id into the request context
// and logs accordingly its details. For api calls it enables CORS as well.
func requestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// generate and setup request id if not set.
		requestid := generateID()
		ctx := context.WithValue(r.Context(), "requestid", requestid)
		r = r.WithContext(ctx)

		if strings.HasPrefix(r.URL.Path, "/worker/api/") {
			// api call, enable cors and log.
			apilog.Printf("[request:%s] - [ip:%s] - [method:%s] - [url:%s] - [agent:%s]\n", requestid, r.RemoteAddr, r.Method, r.URL.Path, r.UserAgent())

			enableCORS(&w, r)

			if r.Method == "OPTIONS" {
				// responds to pre-flight requests before POST.
				w.WriteHeader(http.StatusOK)
				return
			}

		} else {
			// save as web requests.
			weblog.Printf("request: %s] - [ip: %s] - [method: %s] - [url: %s] - [agent: %s]\n", requestid, r.RemoteAddr, r.Method, r.URL.Path, r.UserAgent())
		}
		next.ServeHTTP(w, r)
	})
}

// enableCORS enables CORS for API calls.
func enableCORS(w *http.ResponseWriter, r *http.Request) {
	if origin := r.Header.Get("Origin"); origin != "" {
		(*w).Header().Set("Access-Control-Allow-Origin", origin)
		(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Requested-With")
		(*w).Header().Set("Access-Control-Allow-Credentials", "true")
	}
}
