package api

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func handleMetrics() http.Handler {
	return promhttp.Handler()
}
