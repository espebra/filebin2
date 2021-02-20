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
	Bytes             int       `json:"bytes"`
	SizeRelative      string    `json:"size_readable"`
	Type              string    `json:"-"`
	Trace             string    `json:"trace"`
	Timestamp         time.Time `json:"timestamp"`
	TimestampRelative string    `json:"timestamp_relative"`
}
