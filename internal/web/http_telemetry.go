package web

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/espebra/filebin2/internal/ds"
)

const (
	telemetryFailureMaxBodyBytes = 8192
	telemetrySuccessMaxBodyBytes = 2048
	telemetryMaxResponseBodyChrs = 512
	telemetryMaxFilenameChrs     = 256
	telemetryMaxStatusTextChrs   = 128
	telemetryMaxHeaderChrs       = 128
	telemetryMaxHostChrs         = 253
)

var (
	binIDPattern        = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	connectionPattern   = regexp.MustCompile(`^[a-z0-9-]{1,16}$`)
	telemetryHostRegexp = regexp.MustCompile(`^[A-Za-z0-9._:\[\]-]{1,253}$`)
	telemetryProtocols  = map[string]bool{
		"http":  true,
		"https": true,
	}
	telemetryFailureRaws = map[string]bool{
		"network":     true,
		"stalled":     true,
		"http_status": true,
	}
	telemetryStages = map[string]bool{
		"":                  true,
		"handshake":         true,
		"uploading":         true,
		"awaiting_response": true,
	}
	telemetryVisibilities = map[string]bool{
		"":          true,
		"visible":   true,
		"hidden":    true,
		"prerender": true,
		"unloaded":  true,
	}
)

// failurePayload is the JSON shape posted by the browser when an upload
// fails terminally. Retries are not reported as separate events; the
// final RetryAttempts value tells the story.
type failurePayload struct {
	Bin                     string  `json:"bin"`
	Filename                string  `json:"filename"`
	UploadHost              string  `json:"upload_host"`
	UploadProtocol          string  `json:"upload_protocol"`
	ScriptHost              string  `json:"script_host"`
	ScriptProtocol          string  `json:"script_protocol"`
	TopFrame                bool    `json:"top_frame"`
	Reason                  string  `json:"reason"`
	HTTPStatus              int     `json:"http_status"`
	FileSize                uint64  `json:"file_size"`
	BytesUploaded           uint64  `json:"bytes_uploaded"`
	DurationMs              uint64  `json:"duration_ms"`
	TimeSinceLastProgressMs uint64  `json:"time_since_last_progress_ms"`
	TimeToFirstProgressMs   uint64  `json:"time_to_first_progress_ms"`
	LastBytesPerSecond      uint64  `json:"last_bytes_per_second"`
	RetryAttempts           int     `json:"retry_attempts"`
	ConnectionType          string  `json:"connection_type"`
	Stage                   string  `json:"stage"`
	ReadyState              int     `json:"ready_state"`
	StatusText              string  `json:"status_text"`
	Online                  bool    `json:"online"`
	Visibility              string  `json:"visibility"`
	ConcurrentUploads       int     `json:"concurrent_uploads"`
	Downlink                float64 `json:"downlink"`
	RTT                     int     `json:"rtt"`
	SaveData                bool    `json:"save_data"`
	ResponseBody            string  `json:"response_body"`
	ResponseContentType     string  `json:"response_content_type"`
	RequestID               string  `json:"request_id"`
}

// successPayload is the JSON shape posted by the browser when an upload
// completes successfully. Slimmer than failurePayload because the server
// already records what was uploaded; this captures the perceptual
// timings only the client can measure.
type successPayload struct {
	Bin                   string  `json:"bin"`
	Filename              string  `json:"filename"`
	UploadHost            string  `json:"upload_host"`
	UploadProtocol        string  `json:"upload_protocol"`
	ScriptHost            string  `json:"script_host"`
	ScriptProtocol        string  `json:"script_protocol"`
	TopFrame              bool    `json:"top_frame"`
	FileSize              uint64  `json:"file_size"`
	DurationMs            uint64  `json:"duration_ms"`
	UploadingMs           uint64  `json:"uploading_ms"`
	ProcessingMs          uint64  `json:"processing_ms"`
	TimeToFirstProgressMs uint64  `json:"time_to_first_progress_ms"`
	AverageBytesPerSecond uint64  `json:"average_bytes_per_second"`
	RetryAttempts         int     `json:"retry_attempts"`
	ConnectionType        string  `json:"connection_type"`
	Downlink              float64 `json:"downlink"`
	RTT                   int     `json:"rtt"`
	SaveData              bool    `json:"save_data"`
	Visibility            string  `json:"visibility"`
}

func (h *HTTP) telemetryFailure(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	var p failurePayload
	if !decodeTelemetry(w, r, telemetryFailureMaxBodyBytes, &p) {
		return
	}

	if !telemetryFailureRaws[p.Reason] {
		http.Error(w, "Invalid reason", http.StatusBadRequest)
		return
	}
	if p.Bin != "" && !binIDPattern.MatchString(p.Bin) {
		http.Error(w, "Invalid bin", http.StatusBadRequest)
		return
	}
	if p.ConnectionType != "" && !connectionPattern.MatchString(p.ConnectionType) {
		p.ConnectionType = ""
	}
	if !telemetryStages[p.Stage] {
		p.Stage = ""
	}
	if !telemetryVisibilities[p.Visibility] {
		p.Visibility = ""
	}
	if p.RetryAttempts < 0 {
		p.RetryAttempts = 0
	}
	if p.ConcurrentUploads < 0 {
		p.ConcurrentUploads = 0
	}
	if p.ReadyState < 0 || p.ReadyState > 4 {
		p.ReadyState = 0
	}
	if p.Downlink < 0 {
		p.Downlink = 0
	}
	if p.RTT < 0 {
		p.RTT = 0
	}
	truncate(&p.Filename, telemetryMaxFilenameChrs)
	truncate(&p.ResponseBody, telemetryMaxResponseBodyChrs)
	truncate(&p.StatusText, telemetryMaxStatusTextChrs)
	truncate(&p.ResponseContentType, telemetryMaxHeaderChrs)
	truncate(&p.RequestID, telemetryMaxHeaderChrs)

	uploadHost := normalizeTelemetryHost(p.UploadHost)
	uploadProtocol := normalizeTelemetryProtocol(p.UploadProtocol)
	scriptHost := normalizeTelemetryHost(p.ScriptHost)
	scriptProtocol := normalizeTelemetryProtocol(p.ScriptProtocol)

	bucket := failureBucket(p.Reason, p.HTTPStatus)
	h.metrics.ObserveClientUploadFailure(bucket, p.DurationMs, p.TimeToFirstProgressMs)

	ev := ds.ClientUploadFailure{
		Timestamp:               time.Now().UTC(),
		IP:                      remoteIP(r),
		UserAgent:               truncatedUserAgent(r),
		Bin:                     p.Bin,
		Filename:                p.Filename,
		UploadHost:              uploadHost,
		UploadProtocol:          uploadProtocol,
		ScriptHost:              scriptHost,
		ScriptProtocol:          scriptProtocol,
		TopFrame:                p.TopFrame,
		Reason:                  bucket,
		HTTPStatus:              p.HTTPStatus,
		FileSize:                p.FileSize,
		BytesUploaded:           p.BytesUploaded,
		DurationMs:              p.DurationMs,
		TimeSinceLastProgressMs: p.TimeSinceLastProgressMs,
		TimeToFirstProgressMs:   p.TimeToFirstProgressMs,
		LastBytesPerSecond:      p.LastBytesPerSecond,
		RetryAttempts:           p.RetryAttempts,
		ConnectionType:          p.ConnectionType,
		Stage:                   p.Stage,
		ReadyState:              p.ReadyState,
		StatusText:              p.StatusText,
		Online:                  p.Online,
		Visibility:              p.Visibility,
		ConcurrentUploads:       p.ConcurrentUploads,
		Downlink:                p.Downlink,
		RTT:                     p.RTT,
		SaveData:                p.SaveData,
		ResponseBody:            p.ResponseBody,
		ResponseContentType:     p.ResponseContentType,
		RequestID:               p.RequestID,
	}
	h.recordClientUploadFailure(ev)

	slog.Debug("client upload failure",
		"reason", ev.Reason,
		"http_status", ev.HTTPStatus,
		"status_text", ev.StatusText,
		"ready_state", ev.ReadyState,
		"stage", ev.Stage,
		"bin", ev.Bin,
		"filename", ev.Filename,
		"ip", ev.IP,
		"upload_host", ev.UploadHost,
		"upload_protocol", ev.UploadProtocol,
		"script_host", ev.ScriptHost,
		"script_protocol", ev.ScriptProtocol,
		"top_frame", ev.TopFrame,
		"file_size", ev.FileSize,
		"bytes_uploaded", ev.BytesUploaded,
		"duration_ms", ev.DurationMs,
		"time_since_last_progress_ms", ev.TimeSinceLastProgressMs,
		"time_to_first_progress_ms", ev.TimeToFirstProgressMs,
		"last_bytes_per_second", ev.LastBytesPerSecond,
		"retry_attempts", ev.RetryAttempts,
		"concurrent_uploads", ev.ConcurrentUploads,
		"connection_type", ev.ConnectionType,
		"online", ev.Online,
		"visibility", ev.Visibility,
		"downlink", ev.Downlink,
		"rtt", ev.RTT,
		"save_data", ev.SaveData,
		"response_content_type", ev.ResponseContentType,
		"request_id", ev.RequestID,
		"response_body", ev.ResponseBody,
	)

	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTP) telemetrySuccess(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	var p successPayload
	if !decodeTelemetry(w, r, telemetrySuccessMaxBodyBytes, &p) {
		return
	}

	if p.Bin != "" && !binIDPattern.MatchString(p.Bin) {
		http.Error(w, "Invalid bin", http.StatusBadRequest)
		return
	}
	if p.ConnectionType != "" && !connectionPattern.MatchString(p.ConnectionType) {
		p.ConnectionType = ""
	}
	if !telemetryVisibilities[p.Visibility] {
		p.Visibility = ""
	}
	if p.RetryAttempts < 0 {
		p.RetryAttempts = 0
	}
	if p.Downlink < 0 {
		p.Downlink = 0
	}
	if p.RTT < 0 {
		p.RTT = 0
	}
	truncate(&p.Filename, telemetryMaxFilenameChrs)

	uploadHost := normalizeTelemetryHost(p.UploadHost)
	uploadProtocol := normalizeTelemetryProtocol(p.UploadProtocol)
	scriptHost := normalizeTelemetryHost(p.ScriptHost)
	scriptProtocol := normalizeTelemetryProtocol(p.ScriptProtocol)

	h.metrics.ObserveClientUploadSuccess(p.DurationMs, p.UploadingMs, p.ProcessingMs, p.TimeToFirstProgressMs, p.AverageBytesPerSecond)

	ev := ds.ClientUploadSuccess{
		Timestamp:             time.Now().UTC(),
		IP:                    remoteIP(r),
		UserAgent:             truncatedUserAgent(r),
		Bin:                   p.Bin,
		Filename:              p.Filename,
		UploadHost:            uploadHost,
		UploadProtocol:        uploadProtocol,
		ScriptHost:            scriptHost,
		ScriptProtocol:        scriptProtocol,
		TopFrame:              p.TopFrame,
		FileSize:              p.FileSize,
		DurationMs:            p.DurationMs,
		UploadingMs:           p.UploadingMs,
		ProcessingMs:          p.ProcessingMs,
		TimeToFirstProgressMs: p.TimeToFirstProgressMs,
		AverageBytesPerSecond: p.AverageBytesPerSecond,
		RetryAttempts:         p.RetryAttempts,
		ConnectionType:        p.ConnectionType,
		Downlink:              p.Downlink,
		RTT:                   p.RTT,
		SaveData:              p.SaveData,
		Visibility:            p.Visibility,
	}
	h.recordClientUploadSuccess(ev)

	slog.Debug("client upload success",
		"bin", ev.Bin,
		"filename", ev.Filename,
		"ip", ev.IP,
		"upload_host", ev.UploadHost,
		"upload_protocol", ev.UploadProtocol,
		"script_host", ev.ScriptHost,
		"script_protocol", ev.ScriptProtocol,
		"top_frame", ev.TopFrame,
		"file_size", ev.FileSize,
		"duration_ms", ev.DurationMs,
		"uploading_ms", ev.UploadingMs,
		"processing_ms", ev.ProcessingMs,
		"time_to_first_progress_ms", ev.TimeToFirstProgressMs,
		"average_bytes_per_second", ev.AverageBytesPerSecond,
		"retry_attempts", ev.RetryAttempts,
		"connection_type", ev.ConnectionType,
		"visibility", ev.Visibility,
		"downlink", ev.Downlink,
		"rtt", ev.RTT,
		"save_data", ev.SaveData,
	)

	w.WriteHeader(http.StatusNoContent)
}

// decodeTelemetry reads, size-checks, and JSON-decodes a telemetry POST
// body into v. On error it writes the response and returns false.
func decodeTelemetry(w http.ResponseWriter, r *http.Request, maxBytes int64, v interface{}) bool {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return false
	}
	if int64(len(body)) > maxBytes {
		http.Error(w, "Payload too large", http.StatusRequestEntityTooLarge)
		return false
	}
	if err := json.Unmarshal(body, v); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return false
	}
	return true
}

// failureBucket maps the client-supplied reason and HTTP status to a
// bounded Prometheus label value. The label set is kept small to keep
// cardinality low.
func failureBucket(rawReason string, httpStatus int) string {
	switch rawReason {
	case "network":
		return "network"
	case "stalled":
		return "stalled"
	case "http_status":
		return httpBucket(httpStatus)
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

func remoteIP(r *http.Request) string {
	ip, err := extractIP(r.RemoteAddr)
	if err != nil {
		return ""
	}
	return ip
}

func truncatedUserAgent(r *http.Request) string {
	ua := r.Header.Get("User-Agent")
	if len(ua) > 256 {
		ua = ua[:256]
	}
	return ua
}

func truncate(s *string, max int) {
	if len(*s) > max {
		*s = (*s)[:max]
	}
}

// normalizeTelemetryProtocol accepts a URL.protocol-style value
// ("http:" or "https:") with or without the trailing colon and returns
// the bare scheme. Unknown values become empty.
func normalizeTelemetryProtocol(raw string) string {
	p := strings.ToLower(strings.TrimSuffix(raw, ":"))
	if telemetryProtocols[p] {
		return p
	}
	return ""
}

// normalizeTelemetryHost validates and lowercases a client-reported
// host (with optional :port). Invalid values become empty so downstream
// code never has to deal with junk hostnames.
func normalizeTelemetryHost(raw string) string {
	if len(raw) > telemetryMaxHostChrs {
		return ""
	}
	if raw == "" {
		return ""
	}
	if !telemetryHostRegexp.MatchString(raw) {
		return ""
	}
	return strings.ToLower(raw)
}
