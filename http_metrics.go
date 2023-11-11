package main

import (
	//"bytes"
	//"crypto/hmac"
	//"crypto/sha256"
	//"encoding/hex"
	"fmt"
	//"io"
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
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if username != h.config.MetricsUsername || password != h.config.MetricsPassword {
		time.Sleep(3 * time.Second)
		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	err := h.dao.Metrics().UpdateMetrics(h.metrics)
	if err != nil {
		fmt.Printf("Unable to UpdateMetrics(): %s\n", err.Error())
		http.Error(w, "Errno 328", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "max-age=1")

	//type Data struct {
	//        ds.Metrics
	//}
	h.metrics.Lock()
	defer h.metrics.Unlock()

	if err := h.templates.ExecuteTemplate(w, "metrics", h.metrics); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
	return
}
