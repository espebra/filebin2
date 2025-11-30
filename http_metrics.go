package main

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"time"
)

func (h *HTTP) viewMetrics(w http.ResponseWriter, r *http.Request) {
	// If metrics are not enabled, exit early
	if !h.config.Metrics {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Check that authentication is enabled
	if h.config.MetricsAuth != "" {
		if h.config.MetricsAuth == "basic" {
			username, password, ok := r.BasicAuth()
			if ok == false || username != h.config.MetricsUsername || password != h.config.MetricsPassword {
				time.Sleep(3 * time.Second)
				w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		} else {
			// If an unknown authentication mechanism is specified,
			// reject the request.
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Get metrics from the database and update gauges
	err := h.dao.Metrics().UpdateMetrics(h.metrics)
	if err != nil {
		fmt.Printf("Unable to UpdateMetrics(): %s\n", err.Error())
		http.Error(w, "Errno 328", http.StatusInternalServerError)
		return
	}
	h.metrics.UpdateGauges()

	// Serve Prometheus metrics
	handler := promhttp.HandlerFor(h.metricsRegistry, promhttp.HandlerOpts{})
	handler.ServeHTTP(w, r)
}
