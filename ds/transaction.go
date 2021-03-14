package ds

import (
	"time"
)

type Transaction struct {
	Id                int       `json:"-"`
	BinId             string    `json:"bin"`
	Filename          string    `json:"filename"`
	Method            string    `json:"method"`
	Path              string    `json:"path"`
	IP                string    `json:"ip"`
	Status            int       `json:"status"`
	ReqBytes          int       `json:"req_bytes"`
	ReqBytesReadable  string    `json:"request-bytes-readable"`
	RespBytes         int       `json:"resp_bytes"`
	RespBytesReadable string    `json:"response-bytes-readable"`
	Operation         string    `json:"-"`
	Headers           string    `json:"trace"`
	Timestamp         time.Time `json:"timestamp"`
	TimestampRelative string    `json:"timestamp_relative"`
}
