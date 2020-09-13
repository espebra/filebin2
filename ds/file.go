package ds

import (
	"database/sql"
	"time"
)

type File struct {
	Id                int          `json:"-"`
	Bin               string       `json:"-"`
	Filename          string       `json:"filename"`
	InStorage         bool         `json:"-"`
	Mime              string       `json:"content-type"`
	Category          string       `json:"-"`
	Bytes             uint64       `json:"bytes"`
	BytesReadable     string       `json:"bytes_readable"`
	MD5               string       `json:"md5"`
	SHA256            string       `json:"sha256"`
	Downloads         uint64       `json:"-"`
	Updates           uint64       `json:"updates"`
	IP                string       `json:"-"`
	Trace             string       `json:"-"`
	Nonce             []byte       `json:"-"`
	UpdatedAt         time.Time    `json:"updated_at_"`
	UpdatedAtRelative string       `json:"updated_at__relative"`
	CreatedAt         time.Time    `json:"created_at_"`
	CreatedAtRelative string       `json:"created_at__relative"`
	DeletedAt         sql.NullTime `json:"-"`
	DeletedAtRelative string       `json:"-"`
	URL               string       `json:"-"`
}

func (f *File) IsReadable() bool {
	// Not readable if the file is deleted
	if f.IsDeleted() {
		return false
	}
	if f.InStorage == false {
		return false
	}
	return true
}

func (f *File) IsDeleted() bool {
        if f.DeletedAt.Valid {
                if f.DeletedAt.Time.IsZero() == false {
                        return true
                }
        }
	return false
}
