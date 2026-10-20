package ds

import (
	"time"
)

type Transaction struct {
	Id                int           `json:"-"`
	BinId             string        `json:"bin"`
	Filename          string        `json:"filename"`
	Method            string        `json:"method"`
	Path              string        `json:"path"`
	IP                string        `json:"ip"`
	Status            int           `json:"status"`
	ReqBytes          int64         `json:"req_bytes"`
	ReqBytesReadable  string        `json:"request-bytes-readable"`
	RespBytes         int64         `json:"resp_bytes"`
	RespBytesReadable string        `json:"response-bytes-readable"`
	Operation         string        `json:"-"`
	Headers           string        `json:"trace"`
	Timestamp         time.Time     `json:"timestamp"`
	TimestampRelative string        `json:"timestamp_relative"`
	CompletedAt       time.Time     `json:"completed"`
	Duration          time.Duration `json:"duration"`
}

// TransactionChecksums are the checksums of the file content transferred in
// a request, for the audit log. MD5 is base64 encoded, as in FileContent.
// PHash is the perceptual hash of images.
type TransactionChecksums struct {
	MD5    string
	SHA1   string
	SHA256 string
	PHash  string
}
