package ds

import (
	"time"
)

type Bin struct {
	Id              string    `json:"id"`
	Downloads       uint64    `json:"-"`
	Updated         time.Time `json:"updated"`
	UpdatedRelative string    `json:"updated_relative"`
	Created         time.Time `json:"created"`
	CreatedRelative string    `json:"created_relative"`
}
