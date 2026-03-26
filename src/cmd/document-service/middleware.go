package main

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

func apiKeyAuth(cfg *Config, next http.Handler) http.Handler {
	if len(cfg.APIKeys) == 0 {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-API-Key")
		if key == "" {
			auth := r.Header.Get("Authorization")
			const p = "Bearer "
			if strings.HasPrefix(auth, p) {
				key = strings.TrimSpace(strings.TrimPrefix(auth, p))
			}
		}
		if _, ok := cfg.APIKeys[key]; !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type ipLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
	r        rate.Limit
	b        int
}

func newIPLimiter(rps float64, burst int) *ipLimiter {
	if burst < 1 {
		burst = 1
	}
	return &ipLimiter{
		limiters: make(map[string]*rate.Limiter),
		r:        rate.Limit(rps),
		b:        burst,
	}
}

func (il *ipLimiter) limiterFor(ip string) *rate.Limiter {
	il.mu.Lock()
	defer il.mu.Unlock()
	lim, ok := il.limiters[ip]
	if !ok {
		lim = rate.NewLimiter(il.r, il.b)
		il.limiters[ip] = lim
	}
	return lim
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func rateLimitMiddleware(il *ipLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !il.limiterFor(ip).Allow() {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func instrumentHandler(route string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		pdfInFlight.Inc()
		next.ServeHTTP(rec, r)
		pdfInFlight.Dec()
		d := time.Since(start).Seconds()
		status := strconv.Itoa(rec.status)
		httpRequestsTotal.WithLabelValues(route, status).Inc()
		httpRequestDurationSeconds.WithLabelValues(route, status).Observe(d)
	})
}

func metricsHandler() http.Handler {
	return promhttp.Handler()
}
