package ds

import (
	"time"
)

type FileContent struct {
	SHA256           string    `json:"sha256"`
	Bytes            uint64    `json:"bytes"`
	MD5              string    `json:"md5"`
	Mime             string    `json:"mime"`
	Downloads        uint64    `json:"downloads"`
	InStorage        bool      `json:"in_storage"`
	CreatedAt        time.Time `json:"created_at"`
	LastReferencedAt time.Time `json:"last_referenced_at"`
}
