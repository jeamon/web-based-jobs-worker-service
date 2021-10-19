package main

// @middlewares.go contains middlewares for decorating each URL or API calls.

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// requestMiddleware adds for each request an id into the request context
// and logs accordingly its details. For api calls it enables CORS as well.
func requestMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		var requestid string
		if strings.HasPrefix(r.URL.Path, "/worker/api/") {
			// api call, build the id and enable cors and log.
			requestid = generateApiRequestID(time.Now().UTC())
			apilog.Printf("[request:%s] - [ip:%s] - [method:%s] - [url:%s] - [agent:%s]\n", requestid, r.RemoteAddr, r.Method, r.URL.Path, r.UserAgent())
			enableCORS(&w, r)

			if r.Method == "OPTIONS" {
				// responds to pre-flight requests before POST.
				w.WriteHeader(http.StatusOK)
				return
			}

		} else {
			// process as web requests. so build the id and log.
			requestid = generateWebRequestID(time.Now().UTC())
			weblog.Printf("[request: %s] - [ip: %s] - [method: %s] - [url: %s] - [agent: %s]\n", requestid, r.RemoteAddr, r.Method, r.URL.Path, r.UserAgent())
		}

		// add the request id to the context and forward the request.
		ctx := context.WithValue(r.Context(), "requestid", requestid)
		next.ServeHTTP(w, r.WithContext(ctx))
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
