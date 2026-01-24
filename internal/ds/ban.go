package ds

import (
	"time"
)

type Ban struct {
	Client
	Counter           uint64    `json:"counter"`
	CreatedAt         time.Time `json:"created_at"`
	CreatedAtRelative string    `json:"created_at_relative"`
}
