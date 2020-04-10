package ds

import (
	"time"
)

type File struct {
	Id              int       `json:"-"`
	Bin             string    `json:"-"`
	Filename        string    `json:"filename"`
	Deleted         int       `json:"-"`
	Mime            string    `json:"content-type"`
	Bytes           uint64    `json:"bytes"`
	BytesReadable   string    `json:"bytes_readable"`
	MD5             string    `json:"md5"`
	SHA256          string    `json:"sha256"`
	Downloads       uint64    `json:"-"`
	Updates         uint64    `json:"-"`
	Nonce           []byte    `json:"-"`
	Updated         time.Time `json:"updated"`
	UpdatedRelative string    `json:"updated_relative"`
	Created         time.Time `json:"created"`
	CreatedRelative string    `json:"created_relative"`
	URL             string    `json:"url"`
}
