package main

import (
	//"bytes"
	//"crypto/hmac"
	//"crypto/sha256"
	//"encoding/hex"
	//"fmt"
	"io"
	//"io/ioutil"
	"net/http"
	//"strconv"
	//"strings"
	"time"
)

func (h *HTTP) viewMetrics(w http.ResponseWriter, r *http.Request) {
	// Interpret empty credentials as not enabled, so reject early in this case
	if h.config.MetricsUsername == "" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if h.config.MetricsPassword == "" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	username, password, ok := r.BasicAuth()
	if ok == false {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if username != h.config.MetricsUsername || password != h.config.MetricsPassword {
		time.Sleep(3 * time.Second)
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	io.WriteString(w, h.metrics.Prometheus())
	return
}
