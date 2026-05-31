package ds

import "time"

// ClientUploadFailure is a per-upload terminal failure reported by the
// browser. Retry attempts are not stored as separate events; the count is
// carried via RetryAttempts on the terminal record.
type ClientUploadFailure struct {
	Timestamp               time.Time `json:"timestamp"`
	IP                      string    `json:"ip"`
	UserAgent               string    `json:"user_agent"`
	Bin                     string    `json:"bin"`
	Filename                string    `json:"filename"`
	UploadHost              string    `json:"upload_host"`
	UploadProtocol          string    `json:"upload_protocol"`
	Reason                  string    `json:"reason"`
	HTTPStatus              int       `json:"http_status"`
	FileSize                uint64    `json:"file_size"`
	BytesUploaded           uint64    `json:"bytes_uploaded"`
	DurationMs              uint64    `json:"duration_ms"`
	TimeSinceLastProgressMs uint64    `json:"time_since_last_progress_ms"`
	RetryAttempts           int       `json:"retry_attempts"`
	ConnectionType          string    `json:"connection_type"`
	ResponseBody            string    `json:"response_body"`
	StatusText              string    `json:"status_text"`
	ReadyState              int       `json:"ready_state"`
	Stage                   string    `json:"stage"`
	Online                  bool      `json:"online"`
	Visibility              string    `json:"visibility"`
	TimeToFirstProgressMs   uint64    `json:"time_to_first_progress_ms"`
	LastBytesPerSecond      uint64    `json:"last_bytes_per_second"`
	ConcurrentUploads       int       `json:"concurrent_uploads"`
	Downlink                float64   `json:"downlink"`
	RTT                     int       `json:"rtt"`
	SaveData                bool      `json:"save_data"`
	ResponseContentType     string    `json:"response_content_type"`
	RequestID               string    `json:"request_id"`
}
