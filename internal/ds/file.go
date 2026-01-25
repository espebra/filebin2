package ds

import (
	"database/sql"
	"time"
)

type File struct {
	Id                     int           `json:"-"`
	Bin                    string        `json:"-"`
	Filename               string        `json:"filename"`
	Mime                   string        `json:"content-type"`
	Category               string        `json:"-"`
	Bytes                  uint64        `json:"bytes"`
	BytesReadable          string        `json:"bytes_readable"`
	MD5                    string        `json:"md5"`
	SHA256                 string        `json:"sha256"`
	Downloads              uint64        `json:"-"`
	Updates                uint64        `json:"-"`
	InStorage              bool          `json:"-"`
	IP                     string        `json:"-"`
	Headers                string        `json:"-"`
	UpdatedAt              time.Time     `json:"updated_at"`
	UpdatedAtRelative      string        `json:"updated_at_relative"`
	CreatedAt              time.Time     `json:"created_at"`
	CreatedAtRelative      string        `json:"created_at_relative"`
	DeletedAt              sql.NullTime  `json:"-"`
	DeletedAtRelative      string        `json:"-"`
	BinDeletedAt           sql.NullTime  `json:"-"`
	BinDeletedAtRelative   string        `json:"-"`
	BinExpiredAt           time.Time     `json:"-"`
	BinExpiredAtRelative   string        `json:"-"`
	AvailableForDownload   bool          `json:"-"`
	URL                    string        `json:"-"`
	UploadDurationMs       int64         `json:"-"`
	UploadDuration         time.Duration `json:"-"`
	UploadDurationReadable string        `json:"-"`
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
	return f.DeletedAt.Valid && !f.DeletedAt.Time.IsZero()
}

type FileByChecksum struct {
	SHA256                   string    `json:"sha256"`
	Count                    int       `json:"count"`
	Mime                     string    `json:"content-type"`
	Bytes                    uint64    `json:"bytes"`
	BytesReadable            string    `json:"bytes_readable"`
	BytesTotal               uint64    `json:"bytes_total"`
	BytesTotalReadable       string    `json:"bytes_total_readable"`
	DownloadsTotal           uint64    `json:"downloads_total"`
	UpdatesTotal             uint64    `json:"updates_total"`
	Blocked                  bool      `json:"blocked"`
	CreatedAt                time.Time `json:"created_at"`
	CreatedAtRelative        string    `json:"created_at_relative"`
	LastReferencedAt         time.Time `json:"last_referenced_at"`
	LastReferencedAtRelative string    `json:"last_referenced_at_relative"`
}
