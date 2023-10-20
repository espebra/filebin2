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
	SlackSecret          string
	SlackDomain          string
	SlackChannel         string
	Tmpdir               string
	RequireApproval      bool
	AllowRobots          bool
	BaseUrl              url.URL
	RejectFileExtensions []string
}
