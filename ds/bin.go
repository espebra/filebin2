package ds

import (
	"database/sql"
	"time"
)

type Bin struct {
	Id                string       `json:"id"`
	Readonly          bool         `json:"readonly"`
	Downloads         uint64       `json:"-"`
	Updates           uint64       `json:"-"`
	Bytes             uint64       `json:"bytes"`
	BytesReadable     string       `json:"bytes_readable"`
	Files             uint64       `json:"files"`
	UpdatedAt         time.Time    `json:"updated_at"`
	UpdatedAtRelative string       `json:"updated_at_relative"`
	CreatedAt         time.Time    `json:"created_at"`
	CreatedAtRelative string       `json:"created_at_relative"`
	ExpiredAt         time.Time    `json:"expired_at"`
	ExpiredAtRelative string       `json:"expired_at_relative"`
	DeletedAt         sql.NullTime `json:"-"`
	DeletedAtRelative string       `json:"-"`
	URL               string       `json:"-"`
}

func (b *Bin) IsReadable() bool {
	// Not readable if expired
	if b.IsExpired() {
		return false
	}
	// Not readable if deleted
	if b.IsDeleted() {
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

func (b *Bin) IsExpired() bool {
	if b.ExpiredAt.Before(time.Now()) {
		return true
	}
	return false
}

func (b *Bin) IsDeleted() bool {
	if b.DeletedAt.Valid {
		if b.DeletedAt.Time.IsZero() == false {
			return true
		}
	}
	return false
}
