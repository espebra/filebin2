package ds

import (
	"database/sql"
	"time"
)

type File struct {
	Id                int          `json:"-"`
	Bin               string       `json:"-"`
	Filename          string       `json:"filename"`
	Mime              string       `json:"content-type"`
	Category          string       `json:"-"`
	Bytes             uint64       `json:"bytes"`
	BytesReadable     string       `json:"bytes_readable"`
	MD5               string       `json:"md5"`
	SHA256            string       `json:"sha256"`
	Downloads         uint64       `json:"-"`
	Updates           uint64       `json:"-"`
	InStorage         bool         `json:"-"`
	IP                string       `json:"-"`
	ClientId          string       `json:"-"`
	Headers           string       `json:"-"`
	UpdatedAt         time.Time    `json:"updated_at"`
	UpdatedAtRelative string       `json:"updated_at_relative"`
	CreatedAt         time.Time    `json:"created_at"`
	CreatedAtRelative string       `json:"created_at_relative"`
	DeletedAt         sql.NullTime `json:"-"`
	DeletedAtRelative string       `json:"-"`
	URL               string       `json:"-"`
}

func (f *File) IsReadable() bool {
	// Not readable if the file is deleted
	// Note: Content availability must be checked separately via file_content.in_storage
	if f.IsDeleted() {
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

type FileByChecksum struct {
	SHA256             string `json:"sha256"`
	Count              int    `json:"count"`
	Mime               string `json:"content-type"`
	Bytes              uint64 `json:"bytes"`
	BytesReadable      string `json:"bytes_readable"`
	BytesTotal         uint64 `json:"bytes_total"`
	BytesTotalReadable string `json:"bytes_total_readable"`
	DownloadsTotal     uint64 `json:"downloads_total"`
	UpdatesTotal       uint64 `json:"updates_total"`
}
