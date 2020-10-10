package ds

import (
	"time"
)

type Transaction struct {
	Id                 int       `json:"-"`
	Bin                string    `json:"-"`
	Method             string    `json:"method"`
	Path               string    `json:"path"`
	IP                 string    `json:"ip"`
	Trace              string    `json:"trace"`
	StartedAt          time.Time `json:"started_at"`
	StartedAtRelative  string    `json:"started_at_relative"`
	FinishedAt         time.Time `json:"finished_at"`
	FinishedAtRelative string    `json:"finished_at_relative"`
}
