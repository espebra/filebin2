package ds

import "time"

// ClientUploadSuccess is a per-upload completion event reported by the
// browser. The shape is intentionally slimmer than ClientUploadFailure:
// success events are high-volume, and the most useful client-side signal
// is the performance picture (throughput, time spent in each phase, retry
// count needed to finally succeed).
type ClientUploadSuccess struct {
	Timestamp             time.Time `json:"timestamp"`
	IP                    string    `json:"ip"`
	UserAgent             string    `json:"user_agent"`
	Bin                   string    `json:"bin"`
	Filename              string    `json:"filename"`
	UploadHost            string    `json:"upload_host"`
	UploadProtocol        string    `json:"upload_protocol"`
	FileSize              uint64    `json:"file_size"`
	DurationMs            uint64    `json:"duration_ms"`
	UploadingMs           uint64    `json:"uploading_ms"`
	ProcessingMs          uint64    `json:"processing_ms"`
	TimeToFirstProgressMs uint64    `json:"time_to_first_progress_ms"`
	AverageBytesPerSecond uint64    `json:"average_bytes_per_second"`
	RetryAttempts         int       `json:"retry_attempts"`
	ConnectionType        string    `json:"connection_type"`
	Downlink              float64   `json:"downlink"`
	RTT                   int       `json:"rtt"`
	SaveData              bool      `json:"save_data"`
	Visibility            string    `json:"visibility"`
}
