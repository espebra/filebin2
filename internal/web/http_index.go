package web

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/internal/ds"
)

func (h *HTTP) index(w http.ResponseWriter, r *http.Request) {
	setRobotsPermissions(w, h.config.AllowRobots)
	w.Header().Set("Cache-Control", "max-age=0")

	type Data struct {
		ds.Common
		Bin              ds.Bin          `json:"bin"`
		AvailableStorage bool            `json:"available_storage"`
		SiteMessage      *ds.SiteMessage `json:"site_message,omitempty"`
	}
	var data Data
	data.Page = "front"
	data.Contact = h.config.Contact

	// Fetch site message if published for front page
	h.siteMessageMutex.RLock()
	if h.siteMessage.IsPublishedFrontPage() {
		message := h.siteMessage
		data.SiteMessage = &message
	}
	h.siteMessageMutex.RUnlock()

	bin := &ds.Bin{}
	bin.ExpiredAt = time.Now().UTC().Add(h.config.ExpirationDuration)
	bin.ExpiredAtRelative = humanize.Time(bin.ExpiredAt)
	bin.Id = h.dao.Bin().GenerateId()
	bin.GenerateURL(h.config.BaseUrl)
	data.Bin = *bin

	// Storage limit
	// 0 disables the limit
	// >= 1 enforces a limit, in number of gigabytes stored
	data.AvailableStorage = true
	if h.config.LimitStorageBytes > 0 {
		totalBytesConsumed := h.getCachedStorageBytes()
		if totalBytesConsumed >= h.config.LimitStorageBytes {
			data.AvailableStorage = false
		}
	}

	h.metrics.IncrFrontPageViewCount()

	if err := h.renderTemplate(w, "index", data); err != nil {
		slog.Error("failed to execute template", "error", err)
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
}

func (h *HTTP) about(w http.ResponseWriter, r *http.Request) {
	setRobotsPermissions(w, h.config.AllowRobots)
	w.Header().Set("Cache-Control", "max-age=3600")

	type Data struct {
		ds.Common
		Version string
	}
	var data Data
	data.Page = "about"
	data.Version = h.config.Version

	if err := h.renderTemplate(w, "about", data); err != nil {
		slog.Error("failed to execute template", "error", err)
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

	if h.config.RequireCookie {
		data.CookiesInUse = true
	}

	if err := h.renderTemplate(w, "privacy", data); err != nil {
		slog.Error("failed to execute template", "error", err)
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

	if err := h.renderTemplate(w, "terms", data); err != nil {
		slog.Error("failed to execute template", "error", err)
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

	if err := h.renderTemplate(w, "contact", data); err != nil {
		slog.Error("failed to execute template", "error", err)
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

	if err := h.renderTemplate(w, "api", data); err != nil {
		slog.Error("failed to execute template", "error", err)
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
	if err := h.renderTemplate(w, "apispec", data); err != nil {
		slog.Error("failed to execute template", "error", err)
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
		slog.Warn("database unavailable during status check")
		code = 503
	}

	if h.s3.Status() {
		data.S3Status = true
	} else {
		data.S3Status = false
		slog.Warn("S3 unavailable during status check")
		code = 503
	}

	w.Header().Set("Content-Type", "application/json")
	out, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		slog.Error("failed to parse json", "error", err)
		http.Error(w, "Errno 201", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(code)
	_, _ = w.Write(out)
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
		slog.Warn("S3 unavailable during status check")
		code = 503
	}

	if h.config.LimitStorageBytes > 0 {
		totalBytesConsumed := h.getCachedStorageBytes()
		if totalBytesConsumed >= h.config.LimitStorageBytes {
			data.S3Full = true
			code = 507
		}
	}

	w.Header().Set("Content-Type", "application/json")
	out, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		slog.Error("failed to parse json", "error", err)
		http.Error(w, "Errno 201", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(code)
	_, _ = w.Write(out)
}

func (h *HTTP) robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "max-age=3600")

	if err := h.renderTemplate(w, "robots", nil); err != nil {
		slog.Error("failed to execute template", "error", err)
		http.Error(w, "Errno 302", http.StatusInternalServerError)
		return
	}
}
