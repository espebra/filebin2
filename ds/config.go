package ds

import (
	"net/url"
	"time"
)

type Config struct {
	Expiration           int
	ExpirationDuration   time.Duration
	LimitFileDownloads   uint64
	LimitStorageReadable string
	LimitStorageBytes    uint64
	HttpPort             int
	HttpHost             string
	HttpProxyHeaders     bool
	HttpAccessLog        string
	AdminUsername        string
	AdminPassword        string
	MetricsUsername      string
	MetricsPassword      string
	Metrics              bool
	MetricsAuth          string
	MetricsProxyURL      string
	SlackSecret          string
	SlackDomain          string
	SlackChannel         string
	Tmpdir               string
	RequireApproval      bool
	RequireCookie        bool
	ExpectedCookieValue  string
	AllowRobots          bool
	BaseUrl              url.URL
	RejectFileExtensions []string
}
