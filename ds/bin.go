package ds

import (
	"time"
)

type Bin struct {
	Id                 string    `json:"id"`
	Status             int       `json:"-"`
	Readonly           bool      `json:"readonly"`
	Downloads          uint64    `json:"-"`
	Bytes              uint64    `json:"bytes"`
	BytesReadable      string    `json:"bytes_readable"`
	Updated            time.Time `json:"updated"`
	UpdatedRelative    string    `json:"updated_relative"`
	Created            time.Time `json:"created"`
	CreatedRelative    string    `json:"created_relative"`
	Expiration         time.Time `json:"expiration"`
	ExpirationRelative string    `json:"expiration_relative"`
	Deleted            time.Time `json:"-"`
	DeletedRelative    string    `json:"-"`
}

func (b *Bin) IsReadable() bool {
	// Not readable if expired
	if b.Expired() {
		return false
	}
	// Not readable if the deleted timestamp is more recent than zero
	if b.Deleted.IsZero() == false {
		return false
	}
	// Not readable if flagged as deleted
	if b.Status != 0 {
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
	if b.Expiration.Before(time.Now()) {
		return true
	}
	return false
}
