package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	pdfInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "document_service_pdf_renders_in_flight",
		Help: "Number of PDF render jobs currently running.",
	})
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "document_service_http_requests_total",
			Help: "Total HTTP requests handled.",
		},
		[]string{"route", "status"},
	)
	httpRequestDurationSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "document_service_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"route", "status"},
	)
)
