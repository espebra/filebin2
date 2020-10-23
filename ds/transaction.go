package ds

import (
	"database/sql"
	"time"
)

type Transaction struct {
	Id                 int          `json:"-"`
	BinId              string       `json:"bin"`
	Filename           string       `json:"filename"`
	Method             string       `json:"method"`
	Path               string       `json:"path"`
	IP                 string       `json:"ip"`
	Type               string       `json:"-"`
	Trace              string       `json:"trace"`
	StartedAt          time.Time    `json:"started_at"`
	StartedAtRelative  string       `json:"started_at_relative"`
	FinishedAt         sql.NullTime `json:"finished_at"`
	FinishedAtRelative string       `json:"finished_at_relative"`
}
