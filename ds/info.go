package ds

type Info struct {
	CurrentLogEntries int64 `json:"current_log_entries"`

	CurrentBytes         int64  `json:"current_bytes"`
	CurrentBytesReadable string `json:"current_bytes_readable"`
	CurrentFiles         int64  `json:"current_files"`
	CurrentFilesReadable string `json:"current_files_readable"`
	CurrentBins          int64  `json:"current_bins"`
	CurrentBinsReadable  string `json:"current_bins_readable"`

	FreeBytes         int64  `json:"-"`
	FreeBytesReadable string `json:"-"`

	TotalBytes         int64  `json:"total_bytes"`
	TotalBytesReadable string `json:"total_bytes_readable"`
	TotalFiles         int64  `json:"total_files"`
	TotalFilesReadable string `json:"total_files_readable"`
	TotalBins          int64  `json:"total_bins"`
	TotalBinsReadable  string `json:"total_bins_readable"`
}
