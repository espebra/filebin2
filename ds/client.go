package ds

import (
	"fmt"
	"net"
)

type Client struct {
	IP          net.IP `json:"ip"`
	Fingerprint string `json:"fingerprint"`
	Network     string `json:"network"`
	City        string `json:"city"`
	Country     string `json:"country"`
	Continent   string `json:"continent"`
	Proxy       bool   `json:"proxy"`
}

func (c *Client) String() string {
	var s string
	s += fmt.Sprintf("IP: %s, network: %s, continent: %s, country: %s, city: %s.", c.IP.String(), c.Network, c.Continent, c.Country, c.City)
	if c.Fingerprint != "" {
		s += fmt.Sprintf(" Fingerprint: %s.", c.Fingerprint)
	}
	if c.Proxy {
		s += fmt.Sprintf(" IP is an anonymous proxy.")
	}
	return s
}
