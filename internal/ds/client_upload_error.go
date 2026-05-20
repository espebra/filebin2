package ds

import "time"

type ClientUploadError struct {
	Timestamp               time.Time `json:"timestamp"`
	IP                      string    `json:"ip"`
	UserAgent               string    `json:"user_agent"`
	Bin                     string    `json:"bin"`
	Filename                string    `json:"filename"`
	Reason                  string    `json:"reason"`
	HTTPStatus              int       `json:"http_status"`
	FileSize                uint64    `json:"file_size"`
	BytesUploaded           uint64    `json:"bytes_uploaded"`
	DurationMs              uint64    `json:"duration_ms"`
	TimeSinceLastProgressMs uint64    `json:"time_since_last_progress_ms"`
	RetryAttempts           int       `json:"retry_attempts"`
	ConnectionType          string    `json:"connection_type"`
	ResponseBody            string    `json:"response_body"`
}
