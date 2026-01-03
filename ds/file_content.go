package ds

import (
	"time"
)

type FileContent struct {
	SHA256           string    `json:"sha256"`
	Bytes            uint64    `json:"bytes"`
	ReferenceCount   int       `json:"reference_count"`
	InStorage        bool      `json:"in_storage"`
	CreatedAt        time.Time `json:"created_at"`
	LastReferencedAt time.Time `json:"last_referenced_at"`
}
