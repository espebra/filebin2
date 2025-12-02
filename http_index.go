package main

import (
	"encoding/json"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/ds"
	"io"
	"net/http"
	"time"
)

func (h *HTTP) index(w http.ResponseWriter, r *http.Request) {
	setRobotsPermissions(w, h.config.AllowRobots)
	w.Header().Set("Cache-Control", "max-age=0")

	type Data struct {
		ds.Common
		Bin              ds.Bin `json:"bin"`
		AvailableStorage bool   `json:"available_storage"`
	}
	var data Data
	data.Page = "front"
	data.Contact = h.config.Contact

	bin := &ds.Bin{}
	bin.ExpiredAt = time.Now().UTC().Add(h.config.ExpirationDuration)
	bin.ExpiredAtRelative = humanize.Time(bin.ExpiredAt)
	bin.Id = h.dao.Bin().GenerateId()
	if err := bin.GenerateURL(h.config.BaseUrl); err != nil {
		fmt.Printf("Unable to generate URL: %s\n", err.Error())
		http.Error(w, "Errno 9824", http.StatusInternalServerError)
		return
	}
	data.Bin = *bin

	// Storage limit
	// 0 disables the limit
	// >= 1 enforces a limit, in number of gigabytes stored
	data.AvailableStorage = true
	if h.config.LimitStorageBytes > 0 {
		totalBytesConsumed := h.dao.Metrics().StorageBytesAllocated()
		//fmt.Printf("Using %d bytes, limit is %d bytes\n", totalBytesConsumed, h.config.LimitStorageBytes)
		if totalBytesConsumed >= h.config.LimitStorageBytes {
			data.AvailableStorage = false
		}
	}

	h.metrics.IncrFrontPageViewCount()

	if err := h.templates.ExecuteTemplate(w, "index", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
}

func (h *HTTP) about(w http.ResponseWriter, r *http.Request) {
	setRobotsPermissions(w, h.config.AllowRobots)
	w.Header().Set("Cache-Control", "max-age=3600")

	type Data struct {
		ds.Common
	}
	var data Data
	data.Page = "about"

	if err := h.templates.ExecuteTemplate(w, "about", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
}

func (h *HTTP) privacy(w http.ResponseWriter, r *http.Request) {
	setRobotsPermissions(w, h.config.AllowRobots)
	w.Header().Set("Cache-Control", "max-age=3600")

	type Data struct {
		ds.Common
		CookiesInUse bool
		Bin          ds.Bin `json:"bin"`
	}

	var data Data
	data.Page = "privacy"

	if h.config.RequireCookie == true {
		data.CookiesInUse = true
	}

	if err := h.templates.ExecuteTemplate(w, "privacy", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
}

func (h *HTTP) terms(w http.ResponseWriter, r *http.Request) {
	setRobotsPermissions(w, h.config.AllowRobots)
	w.Header().Set("Cache-Control", "max-age=3600")

	type Data struct {
		ds.Common
		Bin ds.Bin `json:"bin"`
	}
	var data Data
	data.Page = "terms"
	data.Contact = h.config.Contact

	if err := h.templates.ExecuteTemplate(w, "terms", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
}

func (h *HTTP) contact(w http.ResponseWriter, r *http.Request) {
	setRobotsPermissions(w, h.config.AllowRobots)
	w.Header().Set("Cache-Control", "max-age=3600")

	type Data struct {
		ds.Common
		Bin ds.Bin `json:"bin"`
	}
	var data Data
	data.Page = "contact"
	data.Contact = h.config.Contact

	if err := h.templates.ExecuteTemplate(w, "contact", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
}

func (h *HTTP) api(w http.ResponseWriter, r *http.Request) {
	setRobotsPermissions(w, h.config.AllowRobots)
	w.Header().Set("Cache-Control", "max-age=3600")

	type Data struct {
		ds.Common
		Bin ds.Bin `json:"bin"`
	}
	var data Data
	data.Page = "api"

	if err := h.templates.ExecuteTemplate(w, "api", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
}

func (h *HTTP) apiSpec(w http.ResponseWriter, r *http.Request) {
	setRobotsPermissions(w, h.config.AllowRobots)
	w.Header().Set("Cache-Control", "max-age=3600")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	type Data struct {
		ds.Common
		Bin ds.Bin `json:"bin"`
	}
	var data Data
	data.Page = "api"

	w.Header().Set("Content-Type", "text/plain")
	if err := h.templates.ExecuteTemplate(w, "apispec", data); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
}

func (h *HTTP) filebinStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Robots-Tag", "none")
	w.Header().Set("Cache-Control", "max-age=1")

	type Data struct {
		AppStatus bool `json:"app-status"`
		DbStatus  bool `json:"db-status"`
		S3Status  bool `json:"s3-status"`
	}
	var data Data

	code := 200
	data.AppStatus = true
	if h.dao.Status() {
		data.DbStatus = true
	} else {
		data.DbStatus = false
		fmt.Printf("Database unavailable during status check\n")
		code = 503
	}

	if h.s3.Status() {
		data.S3Status = true
	} else {
		data.S3Status = false
		fmt.Printf("S3 unavailable during status check\n")
		code = 503
	}

	w.Header().Set("Content-Type", "application/json")
	out, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		fmt.Printf("Failed to parse json: %s\n", err.Error())
		http.Error(w, "Errno 201", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(code)
	io.WriteString(w, string(out))
}

func (h *HTTP) storageStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Robots-Tag", "none")
	w.Header().Set("Cache-Control", "max-age=1")

	type Data struct {
		S3Status bool `json:"s3-status"`
		S3Full   bool `json:"s3-full"`
	}
	var data Data

	code := 200

	if h.s3.Status() {
		data.S3Status = true
	} else {
		data.S3Status = false
		fmt.Printf("S3 unavailable during status check\n")
		code = 503
	}

	if h.config.LimitStorageBytes > 0 {
		totalBytesConsumed := h.dao.Metrics().StorageBytesAllocated()
		if totalBytesConsumed >= h.config.LimitStorageBytes {
			data.S3Full = true
			code = 507
		}
	}

	w.Header().Set("Content-Type", "application/json")
	out, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		fmt.Printf("Failed to parse json: %s\n", err.Error())
		http.Error(w, "Errno 201", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(code)
	io.WriteString(w, string(out))
}

func (h *HTTP) robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=3600")

	if err := h.templates.ExecuteTemplate(w, "robots", nil); err != nil {
		fmt.Printf("Failed to execute template: %s\n", err.Error())
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
}
