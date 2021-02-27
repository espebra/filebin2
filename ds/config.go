package ds

import (
	"net/url"
	"time"
)

type Config struct {
	Expiration         int
	ExpirationDuration time.Duration
	LimitFileDownloads uint64
	LimitStorage       uint64
	HttpPort           int
	HttpHost           string
	HttpProxyHeaders   bool
	HttpAccessLog      string
	AdminUsername      string
	AdminPassword      string
	Tmpdir             string
	RequireApproval    bool
	BaseUrl            url.URL
}
