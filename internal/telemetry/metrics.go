// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	CallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "eva_calls_total",
			Help: "Total number of calls processed by EVA",
		},
		[]string{"status"},
	)

	CallDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "eva_call_duration_seconds",
			Help:    "Duration of calls in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)
)

func StartMetricsServer(port string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// logger.Fatal().Err(err).Msg("Metrics server failed")
		}
	}()

	return server
}
