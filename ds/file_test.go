package ds

import (
	"database/sql"
	"testing"
	"time"
)

func TestFileIsDeleted(t *testing.T) {
	tests := []struct {
		name      string
		deletedAt sql.NullTime
		want      bool
	}{
		{
			name: "deleted file",
			deletedAt: sql.NullTime{
				Time:  time.Now(),
				Valid: true,
			},
			want: true,
		},
		{
			name: "not deleted file (null)",
			deletedAt: sql.NullTime{
				Time:  time.Time{},
				Valid: false,
			},
			want: false,
		},
		{
			name: "not deleted file (zero time)",
			deletedAt: sql.NullTime{
				Time:  time.Time{},
				Valid: true,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := &File{DeletedAt: tt.deletedAt}
			if got := file.IsDeleted(); got != tt.want {
				t.Errorf("IsDeleted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFileIsReadable(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		file *File
		want bool
	}{
		{
			name: "readable file (not deleted)",
			file: &File{
				DeletedAt: sql.NullTime{Valid: false},
			},
			want: true,
		},
		{
			name: "not readable file (deleted)",
			file: &File{
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
			if got := tt.file.IsReadable(); got != tt.want {
				t.Errorf("IsReadable() = %v, want %v", got, tt.want)
			}
		})
	}
}
