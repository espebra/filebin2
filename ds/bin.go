package ds

import (
	"time"
)

type Bin struct {
	Id                 string    `json:"id"`
	Readonly           bool      `json:"readonly"`
	Deleted            int       `json:"-"`
	Downloads          uint64    `json:"-"`
	Bytes              uint64    `json:"bytes"`
	BytesReadable      string    `json:"bytes_readable"`
	Updated            time.Time `json:"updated"`
	UpdatedRelative    string    `json:"updated_relative"`
	Created            time.Time `json:"created"`
	CreatedRelative    string    `json:"created_relative"`
	Expiration         time.Time `json:"expiration"`
	ExpirationRelative string    `json:"expiration_relative"`
}
