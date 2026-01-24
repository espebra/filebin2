package ds

import (
	"database/sql"
	"testing"
	"time"
)

func TestClientIsBanned(t *testing.T) {
	tests := []struct {
		name     string
		bannedAt sql.NullTime
		want     bool
	}{
		{
			name: "banned client",
			bannedAt: sql.NullTime{
				Time:  time.Now(),
				Valid: true,
			},
			want: true,
		},
		{
			name: "not banned client (null)",
			bannedAt: sql.NullTime{
				Time:  time.Time{},
				Valid: false,
			},
			want: false,
		},
		{
			name: "not banned client (zero time)",
			bannedAt: sql.NullTime{
				Time:  time.Time{},
				Valid: true,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{BannedAt: tt.bannedAt}
			if got := client.IsBanned(); got != tt.want {
				t.Errorf("Client.IsBanned() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAutonomousSystemIsBanned(t *testing.T) {
	tests := []struct {
		name     string
		bannedAt sql.NullTime
		want     bool
	}{
		{
			name: "banned AS",
			bannedAt: sql.NullTime{
				Time:  time.Now(),
				Valid: true,
			},
			want: true,
		},
		{
			name: "not banned AS (null)",
			bannedAt: sql.NullTime{
				Time:  time.Time{},
				Valid: false,
			},
			want: false,
		},
		{
			name: "not banned AS (zero time)",
			bannedAt: sql.NullTime{
				Time:  time.Time{},
				Valid: true,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			as := &AutonomousSystem{BannedAt: tt.bannedAt}
			if got := as.IsBanned(); got != tt.want {
				t.Errorf("AutonomousSystem.IsBanned() = %v, want %v", got, tt.want)
			}
		})
	}
}
