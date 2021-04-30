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
	ClientId          string        `json:"client_id"`
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
