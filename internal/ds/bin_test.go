package ds

import (
	"database/sql"
	"net/url"
	"testing"
	"time"
)

func TestBinIsExpired(t *testing.T) {
	tests := []struct {
		name      string
		expiredAt time.Time
		want      bool
	}{
		{
			name:      "expired bin from past",
			expiredAt: time.Now().Add(-1 * time.Hour),
			want:      true,
		},
		{
			name:      "not expired bin in future",
			expiredAt: time.Now().Add(1 * time.Hour),
			want:      false,
		},
		{
			name:      "expired just now",
			expiredAt: time.Now().Add(-1 * time.Second),
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bin := &Bin{ExpiredAt: tt.expiredAt}
			if got := bin.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBinIsDeleted(t *testing.T) {
	tests := []struct {
		name      string
		deletedAt sql.NullTime
		want      bool
	}{
		{
			name: "deleted bin",
			deletedAt: sql.NullTime{
				Time:  time.Now(),
				Valid: true,
			},
			want: true,
		},
		{
			name: "not deleted bin (null)",
			deletedAt: sql.NullTime{
				Time:  time.Time{},
				Valid: false,
			},
			want: false,
		},
		{
			name: "not deleted bin (zero time)",
			deletedAt: sql.NullTime{
				Time:  time.Time{},
				Valid: true,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bin := &Bin{DeletedAt: tt.deletedAt}
			if got := bin.IsDeleted(); got != tt.want {
				t.Errorf("IsDeleted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBinIsApproved(t *testing.T) {
	tests := []struct {
		name       string
		approvedAt sql.NullTime
		want       bool
	}{
		{
			name: "approved bin",
			approvedAt: sql.NullTime{
				Time:  time.Now(),
				Valid: true,
			},
			want: true,
		},
		{
			name: "not approved bin (null)",
			approvedAt: sql.NullTime{
				Time:  time.Time{},
				Valid: false,
			},
			want: false,
		},
		{
			name: "not approved bin (zero time)",
			approvedAt: sql.NullTime{
				Time:  time.Time{},
				Valid: true,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bin := &Bin{ApprovedAt: tt.approvedAt}
			if got := bin.IsApproved(); got != tt.want {
				t.Errorf("IsApproved() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBinIsReadable(t *testing.T) {
	now := time.Now()
	pastTime := now.Add(-1 * time.Hour)
	futureTime := now.Add(1 * time.Hour)

	tests := []struct {
		name string
		bin  *Bin
		want bool
	}{
		{
			name: "readable bin (not expired, not deleted)",
			bin: &Bin{
				ExpiredAt: futureTime,
				DeletedAt: sql.NullTime{Valid: false},
			},
			want: true,
		},
		{
			name: "not readable bin (expired)",
			bin: &Bin{
				ExpiredAt: pastTime,
				DeletedAt: sql.NullTime{Valid: false},
			},
			want: false,
		},
		{
			name: "not readable bin (deleted)",
			bin: &Bin{
				ExpiredAt: futureTime,
				DeletedAt: sql.NullTime{
					Time:  now,
					Valid: true,
				},
			},
			want: false,
		},
		{
			name: "not readable bin (expired and deleted)",
			bin: &Bin{
				ExpiredAt: pastTime,
				DeletedAt: sql.NullTime{
					Time:  now,
					Valid: true,
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bin.IsReadable(); got != tt.want {
				t.Errorf("IsReadable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBinIsWritable(t *testing.T) {
	now := time.Now()
	pastTime := now.Add(-1 * time.Hour)
	futureTime := now.Add(1 * time.Hour)

	tests := []struct {
		name string
		bin  *Bin
		want bool
	}{
		{
			name: "writable bin (not expired, not deleted, not readonly)",
			bin: &Bin{
				ExpiredAt: futureTime,
				DeletedAt: sql.NullTime{Valid: false},
				Readonly:  false,
			},
			want: true,
		},
		{
			name: "not writable bin (readonly)",
			bin: &Bin{
				ExpiredAt: futureTime,
				DeletedAt: sql.NullTime{Valid: false},
				Readonly:  true,
			},
			want: false,
		},
		{
			name: "not writable bin (expired)",
			bin: &Bin{
				ExpiredAt: pastTime,
				DeletedAt: sql.NullTime{Valid: false},
				Readonly:  false,
			},
			want: false,
		},
		{
			name: "not writable bin (deleted)",
			bin: &Bin{
				ExpiredAt: futureTime,
				DeletedAt: sql.NullTime{
					Time:  now,
					Valid: true,
				},
				Readonly: false,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.bin.IsWritable(); got != tt.want {
				t.Errorf("IsWritable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBinGenerateURL(t *testing.T) {
	tests := []struct {
		name    string
		binID   string
		baseURL string
		want    string
	}{
		{
			name:    "simple bin ID",
			binID:   "testbin",
			baseURL: "https://filebin.net",
			want:    "https://filebin.net/testbin",
		},
		{
			name:    "bin ID with special characters",
			binID:   "test-bin_123",
			baseURL: "https://filebin.net",
			want:    "https://filebin.net/test-bin_123",
		},
		{
			name:    "base URL with path",
			binID:   "mybin",
			baseURL: "https://filebin.net/app",
			want:    "https://filebin.net/app/mybin",
		},
		{
			name:    "base URL with trailing slash",
			binID:   "mybin",
			baseURL: "https://filebin.net/",
			want:    "https://filebin.net/mybin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bin := &Bin{Id: tt.binID}
			u, err := url.Parse(tt.baseURL)
			if err != nil {
				t.Fatalf("Failed to parse URL: %v", err)
			}

			err = bin.GenerateURL(*u)
			if err != nil {
				t.Errorf("GenerateURL() error = %v", err)
			}

			if bin.URL != tt.want {
				t.Errorf("GenerateURL() set URL = %v, want %v", bin.URL, tt.want)
			}
		})
	}
}
