package web

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/espebra/filebin2/internal/ds"
)

const (
	telemetryMaxBodyBytes        = 4096
	telemetryMaxResponseBodyChrs = 512
	telemetryMaxFilenameChrs     = 256
)

var (
	binIDPattern        = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	connectionPattern   = regexp.MustCompile(`^[a-z0-9-]{1,16}$`)
	telemetryReasonsRaw = map[string]bool{
		"network":           true,
		"stalled":           true,
		"http_status":       true,
		"retry_network":     true,
		"retry_stalled":     true,
		"retry_http_status": true,
	}
)

// telemetryPayload is the JSON shape posted by the browser when an upload
// fails terminally. Fields that are unknown or out of range are dropped or
// clamped server-side.
type telemetryPayload struct {
	Bin                     string `json:"bin"`
	Filename                string `json:"filename"`
	Reason                  string `json:"reason"`
	HTTPStatus              int    `json:"http_status"`
	FileSize                uint64 `json:"file_size"`
	BytesUploaded           uint64 `json:"bytes_uploaded"`
	DurationMs              uint64 `json:"duration_ms"`
	TimeSinceLastProgressMs uint64 `json:"time_since_last_progress_ms"`
	RetryAttempts           int    `json:"retry_attempts"`
	ConnectionType          string `json:"connection_type"`
	ResponseBody            string `json:"response_body"`
}

func (h *HTTP) telemetry(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	body, err := io.ReadAll(io.LimitReader(r.Body, telemetryMaxBodyBytes+1))
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if len(body) > telemetryMaxBodyBytes {
		http.Error(w, "Payload too large", http.StatusRequestEntityTooLarge)
		return
	}

	var p telemetryPayload
	if err := json.Unmarshal(body, &p); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if !telemetryReasonsRaw[p.Reason] {
		http.Error(w, "Invalid reason", http.StatusBadRequest)
		return
	}
	if p.Bin != "" && !binIDPattern.MatchString(p.Bin) {
		http.Error(w, "Invalid bin", http.StatusBadRequest)
		return
	}
	if p.ConnectionType != "" && !connectionPattern.MatchString(p.ConnectionType) {
		// Don't fail the request for an unrecognized connection type, just drop it.
		p.ConnectionType = ""
	}
	if p.RetryAttempts < 0 {
		p.RetryAttempts = 0
	}
	// Defense-in-depth truncation; the client also truncates.
	if len(p.ResponseBody) > telemetryMaxResponseBodyChrs {
		p.ResponseBody = p.ResponseBody[:telemetryMaxResponseBodyChrs]
	}
	if len(p.Filename) > telemetryMaxFilenameChrs {
		p.Filename = p.Filename[:telemetryMaxFilenameChrs]
	}

	bucket := bucketReason(p.Reason, p.HTTPStatus)
	h.metrics.IncrClientUploadError(bucket)

	ip, err := extractIP(r.RemoteAddr)
	if err != nil {
		ip = ""
	}

	ua := r.Header.Get("User-Agent")
	if len(ua) > 256 {
		ua = ua[:256]
	}

	ev := ds.ClientUploadError{
		Timestamp:               time.Now().UTC(),
		IP:                      ip,
		UserAgent:               ua,
		Bin:                     p.Bin,
		Filename:                p.Filename,
		Reason:                  bucket,
		HTTPStatus:              p.HTTPStatus,
		FileSize:                p.FileSize,
		BytesUploaded:           p.BytesUploaded,
		DurationMs:              p.DurationMs,
		TimeSinceLastProgressMs: p.TimeSinceLastProgressMs,
		RetryAttempts:           p.RetryAttempts,
		ConnectionType:          p.ConnectionType,
		ResponseBody:            p.ResponseBody,
	}
	h.recordClientUploadError(ev)

	slog.Debug("client upload error reported",
		"reason", ev.Reason,
		"http_status", ev.HTTPStatus,
		"bin", ev.Bin,
		"filename", ev.Filename,
		"ip", ev.IP,
		"file_size", ev.FileSize,
		"bytes_uploaded", ev.BytesUploaded,
		"duration_ms", ev.DurationMs,
		"time_since_last_progress_ms", ev.TimeSinceLastProgressMs,
		"retry_attempts", ev.RetryAttempts,
		"connection_type", ev.ConnectionType,
		"response_body", ev.ResponseBody,
	)

	w.WriteHeader(http.StatusNoContent)
}

// bucketReason maps the raw client reason and HTTP status to a bounded
// Prometheus label value, keeping label cardinality small. Retry events are
// returned with a "retry_" prefix so they can be queried separately from
// terminal failures.
func bucketReason(rawReason string, httpStatus int) string {
	switch rawReason {
	case "network":
		return "network"
	case "stalled":
		return "stalled"
	case "http_status":
		return httpBucket(httpStatus)
	case "retry_network":
		return "retry_network"
	case "retry_stalled":
		return "retry_stalled"
	case "retry_http_status":
		return "retry_" + httpBucket(httpStatus)
	default:
		return "other"
	}
}

func httpBucket(httpStatus int) string {
	switch {
	case httpStatus >= 500 && httpStatus < 600:
		return "http_5xx"
	case httpStatus >= 400 && httpStatus < 500:
		return "http_4xx"
	default:
		return "http_other"
	}
}
