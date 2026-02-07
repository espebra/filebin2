package dbl

import (
	"path"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/espebra/filebin2/internal/ds"
)

// hydrateBin normalizes timestamps to UTC and populates human-readable fields.
func hydrateBin(bin *ds.Bin) {
	// https://github.com/lib/pq/issues/329
	bin.UpdatedAt = bin.UpdatedAt.UTC()
	bin.CreatedAt = bin.CreatedAt.UTC()
	bin.ExpiredAt = bin.ExpiredAt.UTC()
	bin.BytesReadable = humanize.Bytes(bin.Bytes)
	bin.UpdatedAtRelative = humanize.Time(bin.UpdatedAt)
	bin.CreatedAtRelative = humanize.Time(bin.CreatedAt)
	if bin.IsApproved() {
		bin.ApprovedAt.Time = bin.ApprovedAt.Time.UTC()
		bin.ApprovedAtRelative = humanize.Time(bin.ApprovedAt.Time)
	}
	bin.ExpiredAtRelative = humanize.Time(bin.ExpiredAt)
	if bin.IsDeleted() {
		bin.DeletedAt.Time = bin.DeletedAt.Time.UTC()
		bin.DeletedAtRelative = humanize.Time(bin.DeletedAt.Time)
	}
	bin.URL = path.Join("/", bin.Id)
}

// hydrateFile normalizes timestamps to UTC and populates human-readable fields.
func hydrateFile(file *ds.File) {
	// https://github.com/lib/pq/issues/329
	file.UpdatedAt = file.UpdatedAt.UTC()
	file.CreatedAt = file.CreatedAt.UTC()
	file.BinExpiredAt = file.BinExpiredAt.UTC()
	file.UpdatedAtRelative = humanize.Time(file.UpdatedAt)
	file.CreatedAtRelative = humanize.Time(file.CreatedAt)
	if file.IsDeleted() {
		file.DeletedAt.Time = file.DeletedAt.Time.UTC()
		file.DeletedAtRelative = humanize.Time(file.DeletedAt.Time)
	}
	if file.BinDeletedAt.Valid && !file.BinDeletedAt.Time.IsZero() {
		file.BinDeletedAt.Time = file.BinDeletedAt.Time.UTC()
		file.BinDeletedAtRelative = humanize.Time(file.BinDeletedAt.Time)
	}
	if !file.BinExpiredAt.IsZero() {
		file.BinExpiredAtRelative = humanize.Time(file.BinExpiredAt)
	}
	file.BytesReadable = humanize.Bytes(file.Bytes)
	file.UploadDuration = time.Duration(file.UploadDurationMs) * time.Millisecond
	file.UploadDurationReadable = file.UploadDuration.String()
	file.URL = path.Join("/", file.Bin, file.Filename)

	// Compute availability: file not deleted, bin not deleted, bin not expired, content in storage
	binDeleted := file.BinDeletedAt.Valid && !file.BinDeletedAt.Time.IsZero()
	binExpired := !file.BinExpiredAt.IsZero() && file.BinExpiredAt.Before(time.Now())
	file.AvailableForDownload = !file.IsDeleted() && !binDeleted && !binExpired && file.InStorage

	setCategory(file)
}

func setCategory(file *ds.File) {
	if strings.HasPrefix(file.Mime, "image") {
		file.Category = "image"
	} else if strings.HasPrefix(file.Mime, "video") {
		file.Category = "video"
	} else {
		file.Category = "unknown"
	}
}
