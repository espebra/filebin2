package ds

import (
	"time"
)

type Bin struct {
	Id              int       `json:"id"`
	Bid             string    `json:"bid"`
	Updated         time.Time `json:"updated"`
	UpdatedRelative string    `json:"updated_relative"`
	Created         time.Time `json:"created"`
	CreatedRelative string    `json:"created_relative"`
}
