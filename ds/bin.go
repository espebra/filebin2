package ds

import (
	"time"
)

type Bin struct {
	Id                string    `json:"id"`
	Hidden            bool      `json:"-"`
	Deleted           bool      `json:"-"`
	Readonly          bool      `json:"readonly"`
	Downloads         uint64    `json:"-"`
	Bytes             uint64    `json:"bytes"`
	BytesReadable     string    `json:"bytes_readable"`
	Files             uint64    `json:"files"`
	UpdatedAt         time.Time `json:"updated_at"`
	UpdatedAtRelative string    `json:"updated_at_relative"`
	CreatedAt         time.Time `json:"created_at"`
	CreatedAtRelative string    `json:"created_at_relative"`
	ExpiredAt         time.Time `json:"expired_at"`
	ExpiredAtRelative string    `json:"expired_at_relative"`
	DeletedAt         time.Time `json:"-"`
	DeletedAtRelative string    `json:"-"`
	URL               string    `json:"-"`
}

func (b *Bin) IsReadable() bool {
	// Not readable if expired
	if b.Expired() {
		return false
	}
	// Not readable if the deleted timestamp is more recent than zero
	if b.DeletedAt.IsZero() == false {
		return false
	}
	// Not readable if flagged as deleted
	if b.Hidden {
		return false
	}
	return true
}

func (b *Bin) IsWritable() bool {
	// Not writable if not readable
	if b.IsReadable() == false {
		return false
	}
	// Not readable if bin is readonly
	if b.Readonly {
		return false
	}
	return true
}

func (b *Bin) Expired() bool {
	if b.ExpiredAt.Before(time.Now()) {
		return true
	}
	return false
}
