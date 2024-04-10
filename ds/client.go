package ds

import (
	"database/sql"
	//"fmt"
	//"net"
	"time"
)

type Client struct {
	IP                    string       `json:"ip"`
	ASN                   int          `json:"asn"`
	Network               string       `json:"network"`
	City                  string       `json:"city"`
	Country               string       `json:"country"`
	Continent             string       `json:"continent"`
	Proxy                 bool         `json:"proxy"`
	Requests              uint64       `json:"requests"`
	FirstActiveAt         time.Time    `json:"first_active_at"`
	FirstActiveAtRelative string       `json:"first_active_at_relative"`
	LastActiveAt          time.Time    `json:"last_active_at"`
	LastActiveAtRelative  string       `json:"last_active_at_relative"`
	BannedAt              sql.NullTime `json:"banned_at"`
	BannedAtRelative      string       `json:"banned_at_relative"`
	BannedBy              string       `json:"banned_by"`
}

func (c *Client) IsBanned() bool {
	if c.BannedAt.Valid {
		if c.BannedAt.Time.IsZero() == false {
			return true
		}
	}
	return false
}

type AutonomousSystem struct {
	ASN                   int          `json:"asn"`
	Organization          string       `json:"organization"`
	Network               string       `json:"network"`
	Requests              uint64       `json:"requests"`
	FirstActiveAt         time.Time    `json:"first_active_at"`
	FirstActiveAtRelative string       `json:"first_active_at_relative"`
	LastActiveAt          time.Time    `json:"last_active_at"`
	LastActiveAtRelative  string       `json:"last_active_at_relative"`
	BannedAt              sql.NullTime `json:"banned_at"`
	BannedAtRelative      string       `json:"banned_at_relative"`
}

func (a *AutonomousSystem) IsBanned() bool {
	if a.BannedAt.Valid {
		if a.BannedAt.Time.IsZero() == false {
			return true
		}
	}
	return false
}
