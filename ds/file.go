package ds

import (
	"time"
)

type File struct {
	Id              int       `json:"-"`
	Bin             string    `json:"-"`
	Filename        string    `json:"filename"`
	Mime            string    `json:"content-type"`
	Bytes           uint64    `json:"bytes"`
	Checksum        string    `json:"checksum"`
	Downloads       uint64    `json:"-"`
	Nonce           []byte    `json:"-"`
	Updated         time.Time `json:"updated"`
	UpdatedRelative string    `json:"updated_relative"`
	Created         time.Time `json:"created"`
	CreatedRelative string    `json:"created_relative"`
}
