package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime configuration (12-factor via environment).
type Config struct {
	Addr                 string
	ReadHeaderTimeout    time.Duration
	ReadTimeout          time.Duration
	WriteTimeout         time.Duration
	IdleTimeout          time.Duration
	ShutdownTimeout      time.Duration
	MaxBodyBytes         int64
	PDFTimeout           time.Duration
	MaxConcurrentRenders int
	APIKeys              map[string]struct{}
	RateLimitRPS         float64
	RateLimitBurst       int
	LogJSON              bool
}

func defaultConfig() *Config {
	return &Config{
		Addr:                 ":8080",
		ReadHeaderTimeout:    5 * time.Second,
		ReadTimeout:          10 * time.Second,
		WriteTimeout:         60 * time.Second,
		IdleTimeout:          120 * time.Second,
		ShutdownTimeout:      30 * time.Second,
		MaxBodyBytes:         2 << 20,
		PDFTimeout:           30 * time.Second,
		MaxConcurrentRenders: 2,
		RateLimitRPS:         0,
		RateLimitBurst:       20,
		LogJSON:              false,
	}
}

func loadConfig() (*Config, error) {
	c := defaultConfig()

	if v := os.Getenv("DOCUMENT_SERVICE_ADDR"); v != "" {
		c.Addr = v
	}
	if v := os.Getenv("DOCUMENT_SERVICE_READ_HEADER_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("DOCUMENT_SERVICE_READ_HEADER_TIMEOUT: %w", err)
		}
		c.ReadHeaderTimeout = d
	}
	if v := os.Getenv("DOCUMENT_SERVICE_READ_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("DOCUMENT_SERVICE_READ_TIMEOUT: %w", err)
		}
		c.ReadTimeout = d
	}
	if v := os.Getenv("DOCUMENT_SERVICE_WRITE_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("DOCUMENT_SERVICE_WRITE_TIMEOUT: %w", err)
		}
		c.WriteTimeout = d
	}
	if v := os.Getenv("DOCUMENT_SERVICE_IDLE_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("DOCUMENT_SERVICE_IDLE_TIMEOUT: %w", err)
		}
		c.IdleTimeout = d
	}
	if v := os.Getenv("DOCUMENT_SERVICE_SHUTDOWN_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("DOCUMENT_SERVICE_SHUTDOWN_TIMEOUT: %w", err)
		}
		c.ShutdownTimeout = d
	}
	if v := os.Getenv("DOCUMENT_SERVICE_MAX_BODY_BYTES"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("DOCUMENT_SERVICE_MAX_BODY_BYTES: %w", err)
		}
		c.MaxBodyBytes = n
	}
	if v := os.Getenv("DOCUMENT_SERVICE_PDF_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("DOCUMENT_SERVICE_PDF_TIMEOUT: %w", err)
		}
		c.PDFTimeout = d
	}
	if v := os.Getenv("DOCUMENT_SERVICE_MAX_CONCURRENT_RENDERS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("DOCUMENT_SERVICE_MAX_CONCURRENT_RENDERS: %w", err)
		}
		if n < 1 {
			return nil, fmt.Errorf("DOCUMENT_SERVICE_MAX_CONCURRENT_RENDERS must be >= 1")
		}
		c.MaxConcurrentRenders = n
	}
	if v := os.Getenv("DOCUMENT_SERVICE_API_KEYS"); v != "" {
		c.APIKeys = parseAPIKeys(v)
	}
	if v := os.Getenv("DOCUMENT_SERVICE_RATE_LIMIT_RPS"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("DOCUMENT_SERVICE_RATE_LIMIT_RPS: %w", err)
		}
		c.RateLimitRPS = f
	}
	if v := os.Getenv("DOCUMENT_SERVICE_RATE_LIMIT_BURST"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("DOCUMENT_SERVICE_RATE_LIMIT_BURST: %w", err)
		}
		c.RateLimitBurst = n
	}
	if v := os.Getenv("DOCUMENT_SERVICE_LOG_JSON"); v != "" {
		c.LogJSON = strings.EqualFold(v, "1") || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
	}

	return c, nil
}

func parseAPIKeys(s string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, p := range strings.Split(s, ",") {
		k := strings.TrimSpace(p)
		if k != "" {
			out[k] = struct{}{}
		}
	}
	return out
}
